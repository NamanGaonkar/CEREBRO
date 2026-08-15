package downloader

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cerebro/internal/model"

	"github.com/kkdai/youtube/v2"
)

// Quality labels shown in the YouTube quality modal.
const (
	Q4K    = "4K MP4"
	Q1080p = "1080p MP4"
	Q720p  = "720p MP4"
	QMP3   = "320kbps MP3"
)

func ytClient() *youtube.Client { return &youtube.Client{} }

// YouTubeQualityMenu resolves the qualities actually available for a video.
// It returns an ordered list of labels and a map from label -> itag number.
func YouTubeQualityMenu(videoID string) ([]string, map[string]string, error) {
	v, err := ytClient().GetVideo(videoID)
	if err != nil {
		return nil, nil, err
	}
	menu := make(map[string]string)
	var order []string
	add := func(label, itag string) {
		if itag == "" {
			return
		}
		if _, ok := menu[label]; !ok {
			menu[label] = itag
			order = append(order, label)
		}
	}
	var fourK, fullHD, hd, muxed720 string
	for _, f := range v.Formats {
		if !strings.HasPrefix(f.MimeType, "video/") {
			continue
		}
		switch f.Quality {
		case "hd2160":
			if fourK == "" {
				fourK = strconv.Itoa(f.ItagNo)
			}
		case "hd1080":
			if fullHD == "" {
				fullHD = strconv.Itoa(f.ItagNo)
			}
		case "hd720":
			if hd == "" {
				hd = strconv.Itoa(f.ItagNo)
			}
			if f.AudioChannels > 0 && muxed720 == "" {
				muxed720 = strconv.Itoa(f.ItagNo)
			}
		}
	}
	sel720 := hd
	if muxed720 != "" {
		sel720 = muxed720
	}
	audio := bestAudioItag(v)
	add(Q4K, fourK)
	add(Q1080p, fullHD)
	add(Q720p, sel720)
	add(QMP3, audio)
	if len(order) == 0 {
		// Last resort: expose every playable video format.
		for _, f := range v.Formats {
			if strings.HasPrefix(f.MimeType, "video/") {
				label := strings.ToUpper(f.Quality) + " " + mimeShort(f.MimeType)
				add(label, strconv.Itoa(f.ItagNo))
			}
		}
	}
	return order, menu, nil
}

// bestAudioItag finds the highest-bitrate audio-only format.
func bestAudioItag(v *youtube.Video) string {
	best, rate := "", -1
	for _, f := range v.Formats {
		if f.AudioChannels > 0 && !strings.HasPrefix(f.MimeType, "video/") {
			if f.Bitrate > rate {
				best, rate = strconv.Itoa(f.ItagNo), f.Bitrate
			}
		}
	}
	return best
}

func mimeShort(t string) string {
	t = strings.Split(t, ";")[0]
	return strings.TrimPrefix(t, "video/")
}

// extFromMime maps an audio mime type to a file extension.
func extFromMime(mime string) string {
	switch {
	case strings.HasPrefix(mime, "audio/webm"):
		return ".webm"
	case strings.HasPrefix(mime, "audio/ogg"):
		return ".ogg"
	case strings.HasPrefix(mime, "audio/mp4"), strings.HasPrefix(mime, "audio/mpeg"):
		return ".m4a"
	case strings.HasPrefix(mime, "audio/aac"):
		return ".aac"
	default:
		return ".m4a"
	}
}

// findFormat resolves an itag to a concrete format.
func findFormat(v *youtube.Video, itag string) (youtube.Format, bool) {
	n, err := strconv.Atoi(itag)
	if err != nil || n <= 0 {
		return youtube.Format{}, false
	}
	for _, f := range v.Formats {
		if f.ItagNo == n {
			return f, true
		}
	}
	return youtube.Format{}, false
}

// downloadYouTube is the top-level YouTube job: get info, pick streams,
// download them and (with ffmpeg) merge audio+video into a single .mp4.
func (m *Manager) downloadYouTube(job *model.DownloadJob, quality string) {
	client := ytClient()
	// Resolving can take several seconds with no bytes to show — surface it
	// as a distinct state so the row never looks stalled at 0%.
	m.mutate(job.ID, func(j *model.DownloadJob) { j.Status = model.StatusResolving })
	m.emit(job)
	v, err := client.GetVideo(job.Result.VideoID)
	if err != nil {
		m.fail(job, fmt.Errorf("youtube: %w", err))
		return
	}
	m.mutate(job.ID, func(j *model.DownloadJob) { j.Status = model.StatusDownloading })
	m.emit(job)

	_, ffmpegErr := exec.LookPath("ffmpeg")
	ffmpegOK := ffmpegErr == nil
	if quality == QMP3 {
		m.downloadAudioOnly(job, client, v, ffmpegOK)
		return
	}
	m.downloadVideo(job, client, v, quality, ffmpegOK)
}

func (m *Manager) downloadAudioOnly(job *model.DownloadJob, client *youtube.Client, v *youtube.Video, ffmpegOK bool) {
	f, ok := findFormat(v, job.Result.QualityMap[QMP3])
	if !ok {
		m.fail(job, fmt.Errorf("no audio stream available"))
		return
	}
	tmp := filepath.Join(m.dir, ".cerebro_audio_"+job.ID)
	if err := m.streamToFile(job, client, v, &f, tmp); err != nil {
		m.fail(job, err)
		return
	}
	defer os.Remove(tmp)

	out := filepath.Join(m.dir, model.SanitizeFilename(job.Result.Title)+".mp3")
	if !ffmpegOK {
		// No ffmpeg: keep the raw audio container with its native extension
		// instead of deleting it (defer os.Remove is harmless after rename).
		rawOut := filepath.Join(m.dir, model.SanitizeFilename(job.Result.Title)+extFromMime(f.MimeType))
		if err := os.Rename(tmp, rawOut); err != nil {
			m.fail(job, err)
			return
		}
		m.complete(job, rawOut)
		return
	}
	m.mutate(job.ID, func(j *model.DownloadJob) {
		j.Status = model.StatusMerging
		if j.Progress > 0.9 {
			j.Progress = 0.9
		}
	})
	m.emit(job)
	stop := m.creepMerge(job)
	if err := runFFmpeg("-y", "-i", tmp, "-vn", "-codec:a", "libmp3lame", "-b:a", "320k", out); err != nil {
		stop()
		m.fail(job, fmt.Errorf("ffmpeg: %w", err))
		return
	}
	stop()
	m.complete(job, out)
}

func (m *Manager) downloadVideo(job *model.DownloadJob, client *youtube.Client, v *youtube.Video, quality string, ffmpegOK bool) {
	vf, ok := findFormat(v, job.Result.QualityMap[quality])
	if !ok {
		m.fail(job, fmt.Errorf("selected quality (%s) is not available", quality))
		return
	}
	af, aok := findFormat(v, bestAudioItag(v))
	needsMerge := !(vf.AudioChannels > 0) && aok

	vtmp := filepath.Join(m.dir, ".cerebro_video_"+job.ID)
	atmp := filepath.Join(m.dir, ".cerebro_audio_"+job.ID)

	// Probe stream sizes so the progress bar knows the total.
	vsize := streamSize(client, v, &vf)
	asize := int64(0)
	if needsMerge {
		asize = streamSize(client, v, &af)
	}
	m.mutate(job.ID, func(j *model.DownloadJob) { j.BytesTotal = vsize + asize })
	m.emit(job)

	if err := m.streamToFile(job, client, v, &vf, vtmp); err != nil {
		m.fail(job, err)
		return
	}
	defer os.Remove(vtmp)
	if needsMerge {
		if err := m.streamToFile(job, client, v, &af, atmp); err != nil {
			m.fail(job, err)
			return
		}
		defer os.Remove(atmp)
	}

	out := filepath.Join(m.dir, model.SanitizeFilename(job.Result.Title)+".mp4")
	if needsMerge {
		if !ffmpegOK {
			m.fail(job, fmt.Errorf("ffmpeg is required to merge video and audio"))
			return
		}
		m.mutate(job.ID, func(j *model.DownloadJob) {
			j.Status = model.StatusMerging
			if j.Progress > 0.9 {
				j.Progress = 0.9
			}
		})
		m.emit(job)
		stop := m.creepMerge(job)
		if err := runFFmpeg("-y", "-i", vtmp, "-i", atmp,
			"-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
			"-shortest", "-movflags", "+faststart", out); err != nil {
			stop()
			m.fail(job, fmt.Errorf("ffmpeg: %w", err))
			return
		}
		stop()
	} else {
		if err := os.Rename(vtmp, out); err != nil {
			m.fail(job, err)
			return
		}
	}
	m.complete(job, out)
}

// creepMerge animates the progress bar while ffmpeg merges streams — the
// merge phase moves no bytes, so without this the bar freezes at ~90% and
// then suddenly jumps to 100%. It creeps toward 99% and stops the moment the
// job leaves the merging state or reaches the cap.
func (m *Manager) creepMerge(job *model.DownloadJob) (stop func()) {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(150 * time.Millisecond):
				m.mutate(job.ID, func(j *model.DownloadJob) {
					if j.Status != model.StatusMerging || j.Progress >= 0.99 {
						return
					}
					j.Progress += 0.012
					if j.Progress > 0.99 {
						j.Progress = 0.99
					}
				})
			}
		}
	}()
	return func() { close(done) }
}

// streamSize returns a stream's byte size. GetVideo already populates
// ContentLength on most formats, so we avoid an extra network round-trip;
// only probe when the length is unknown.
func streamSize(client *youtube.Client, v *youtube.Video, f *youtube.Format) int64 {
	if f.ContentLength > 0 {
		return f.ContentLength
	}
	rc, n, err := client.GetStream(v, f)
	if err != nil {
		return 0
	}
	_ = rc.Close()
	return n
}

// streamToFile downloads a format into path, streaming progress into the job.
func (m *Manager) streamToFile(job *model.DownloadJob, client *youtube.Client, v *youtube.Video, f *youtube.Format, path string) error {
	rc, _, err := client.GetStream(v, f)
	if err != nil {
		return fmt.Errorf("youtube stream: %w", err)
	}
	defer rc.Close()
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, io.TeeReader(rc, &jobWriter{m: m, id: job.ID})); err != nil {
		return fmt.Errorf("youtube download: %w", err)
	}
	return nil
}

// jobWriter funnels streamed bytes into a job's progress counters.
type jobWriter struct {
	m  *Manager
	id string
}

func (w *jobWriter) Write(p []byte) (int, error) {
	w.m.mutate(w.id, func(j *model.DownloadJob) {
		j.BytesDone += int64(len(p))
		if j.BytesTotal > 0 {
			j.Progress = float64(j.BytesDone) / float64(j.BytesTotal)
		}
	})
	return len(p), nil
}

// runFFmpeg executes ffmpeg with the given args.
func runFFmpeg(args ...string) error {
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

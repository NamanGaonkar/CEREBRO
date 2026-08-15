// Package streamer plays torrents instantly through mpv. Pieces are
// downloaded sequentially on demand and piped straight to mpv's stdin, so
// playback starts in seconds — no full download, no browser, no localhost.
package streamer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cerebro/internal/model"

	"github.com/anacrolix/torrent"
	"github.com/kkdai/youtube/v2"
)

// findMpv locates the mpv binary. exec.LookPath covers normal installs; on
// Windows we also probe the common per-user / Program Files locations so
// streaming works even from terminals whose PATH predates the install.
func findMpv() (string, error) {
	if p, err := exec.LookPath("mpv"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "mpv", "mpv.exe"),
		filepath.Join(home, "scoop", "apps", "mpv", "current", "mpv.exe"),
		`C:\Program Files\mpv\mpv.exe`,
		`C:\Program Files (x86)\mpv\mpv.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "mpv", "mpv.exe"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("mpv is not installed — streaming needs it · install with: winget install shinchiro.mpv")
}

// Stream resolves magnet and pipes the largest file to mpv. It returns once
// mpv exits (EOF after the file finishes, or the user closes the player).
// The torrent's pieces are written to a temp dir that is removed when the
// player closes — streaming never fills the user's downloads folder.
// (dir is ignored on purpose; playback data always goes to temp.)
func Stream(ctx context.Context, magnet, dir string) error {
	mpv, err := findMpv()
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "cerebro-stream-")
	if err != nil {
		return fmt.Errorf("stream temp dir: %w", err)
	}
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = tmp
	cfg.Seed = false
	cfg.NoUpload = true
	cfg.Debug = false
	client, err := torrent.NewClient(cfg)
	if err != nil {
		os.RemoveAll(tmp)
		return fmt.Errorf("torrent client: %w", err)
	}
	// LIFO order: client.Close runs first, then the temp dir is removed.
	defer os.RemoveAll(tmp)
	defer client.Close()

	t, err := client.AddMagnet(model.EnhanceMagnet(magnet))
	if err != nil {
		return fmt.Errorf("adding magnet: %w", err)
	}
	select {
	case <-t.GotInfo():
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return errors.New("timed out fetching torrent metadata")
	}

	var file *torrent.File
	for _, f := range t.Files() {
		if file == nil || f.Length() > file.Length() {
			file = f
		}
	}
	if file == nil {
		return errors.New("torrent contains no files")
	}

	// Sequential piece reader: the next piece needed for playback is always
	// prioritized, so playback can begin long before the file completes.
	reader := file.NewReader()
	reader.SetResponsive() // sequential: always prioritize the next piece needed
	defer reader.Close()

	cmd := exec.CommandContext(ctx, mpv,
		"--cache=yes", "--demuxer-max-bytes=100M",
		"--force-window", "--no-terminal", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting mpv: %w", err)
	}

	_, copyErr := io.Copy(stdin, reader)
	_ = stdin.Close()
	_ = cmd.Wait()

	// Closing the player mid-stream is a clean stop (mpv closes stdin), not
	// an error. Only report the context being canceled by the app itself.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if copyErr != nil && !errors.Is(copyErr, os.ErrClosed) {
		return nil // mpv quit first — treat as user-stopped playback
	}
	return nil
}

// StreamYouTube plays a YouTube video in mpv at the best available quality.
// It tries, in order: (1) the highest-quality separate video + audio streams
// merged in mpv via --audio-file (1080p/4K; most modern videos have no muxed
// format, and when one exists it is usually 240p), (2) the best muxed format,
// and finally (3) hands the plain watch URL to mpv so its built-in yt-dlp
// support resolves the video — a bulletproof fallback when the one-shot
// googlevideo URLs above are expired/blocked (which surfaces as mpv exit 2).
// mpv's stdio is detached so the TUI terminal is never garbled.
func StreamYouTube(ctx context.Context, videoID string) error {
	mpv, err := findMpv()
	if err != nil {
		return err
	}
	client := &youtube.Client{}
	v, err := client.GetVideo(videoID)
	if err != nil {
		return fmt.Errorf("youtube: %w", err)
	}

	args := []string{"--cache=yes", "--demuxer-max-bytes=100M", "--force-window", "--no-terminal"}

	// Path 1: separate best video + best audio streams merged in mpv.
	if bestV, bestA, err := bestSeparateFormats(v); err == nil {
		if videoURL, vErr := client.GetStreamURL(v, bestV); vErr == nil {
			if audioURL, aErr := client.GetStreamURL(v, bestA); aErr == nil {
				if runMpv(ctx, mpv, append(args, "--audio-file="+audioURL, videoURL)) == nil {
					return nil
				}
			}
		}
	}

	// Path 2: best muxed format as a single URL.
	if f, ok := bestMuxedFormat(v); ok {
		if streamURL, sErr := client.GetStreamURL(v, &f); sErr == nil {
			if runMpv(ctx, mpv, append(args, streamURL)) == nil {
				return nil
			}
		}
	}

	// Path 3: resolve the watch URL with yt-dlp ourselves and hand mpv the
	// direct stream URLs. mpv's built-in ytdl hook cannot reliably find a
	// pip-installed yt-dlp (it's not next to mpv.exe and often not on PATH), so
	// relying on it silently fails — which looked like "P opens the player
	// once, then never again". Resolving here is bulletproof: yt-dlp survives
	// expired one-shot URLs and bot checks, and mpv just plays the URLs.
	ytdlp, fErr := findYtDlp()
	if fErr != nil {
		return fErr
	}
	if videoURL, audioURL, rErr := resolveWithYtDlp(ctx, ytdlp, "https://www.youtube.com/watch?v="+videoID); rErr == nil {
		if audioURL != "" {
			err = runMpv(ctx, mpv, append(args, "--audio-file="+audioURL, videoURL))
		} else {
			err = runMpv(ctx, mpv, append(args, videoURL))
		}
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("streaming failed: %w", err)
}

// findYtDlp locates the yt-dlp binary. exec.LookPath covers installs on
// PATH; on Windows we also probe the pip Scripts folders (where `pip install
// yt-dlp` lands) plus the common scoop/winget locations.
func findYtDlp() (string, error) {
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "AppData", "Roaming", "Python", "Python314", "Scripts", "yt-dlp.exe"),
		filepath.Join(home, "AppData", "Roaming", "Python", "Python313", "Scripts", "yt-dlp.exe"),
		filepath.Join(home, "AppData", "Roaming", "Python", "Python312", "Scripts", "yt-dlp.exe"),
		filepath.Join(home, "AppData", "Roaming", "Python", "Python311", "Scripts", "yt-dlp.exe"),
		filepath.Join(home, "AppData", "Local", "Programs", "Python", "Python314", "Scripts", "yt-dlp.exe"),
		filepath.Join(home, "scoop", "apps", "yt-dlp", "current", "yt-dlp.exe"),
		filepath.Join(home, "scoop", "shims", "yt-dlp.exe"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("yt-dlp is not installed — streaming needs it · install with: pip install yt-dlp (or winget install yt-dlp)")
}

// resolveWithYtDlp asks yt-dlp for the direct stream URLs of a watch URL.
// It returns the video URL and, when the best format is split (video+audio
// streams), the audio URL to merge via mpv's --audio-file.
func resolveWithYtDlp(ctx context.Context, ytdlp, watchURL string) (videoURL, audioURL string, err error) {
	cmd := exec.CommandContext(ctx, ytdlp,
		"--no-playlist", "--no-warnings", "--format", "bestvideo+bestaudio/best", "--get-url", watchURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("yt-dlp resolve: %w", err)
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 {
		return "", "", errors.New("yt-dlp returned no stream URLs")
	}
	videoURL = lines[0]
	if len(lines) > 1 {
		audioURL = lines[1]
	}
	return videoURL, audioURL, nil
}

// runMpv launches mpv with the given args and reports whether playback
// actually started. mpv exits 2 when it cannot open the input (expired URL,
// bot-checked stream…); those count as "didn't start" so the caller can try
// the next route. A clean exit (user closed the player, file finished) is
// success. stdio is detached so the TUI console stays clean.
func runMpv(ctx context.Context, mpv string, args []string) error {
	cmd := exec.CommandContext(ctx, mpv, args...)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// bestSeparateFormats picks the highest-bitrate video-only and audio-only
// streams — the pair mpv merges with --audio-file.
func bestSeparateFormats(v *youtube.Video) (*youtube.Format, *youtube.Format, error) {
	var bestV, bestA *youtube.Format
	for i := range v.Formats {
		f := &v.Formats[i]
		if strings.HasPrefix(f.MimeType, "video/") && f.AudioChannels == 0 {
			if bestV == nil || f.Bitrate > bestV.Bitrate {
				bestV = f
			}
		} else if !strings.HasPrefix(f.MimeType, "video/") && f.AudioChannels > 0 {
			if bestA == nil || f.Bitrate > bestA.Bitrate {
				bestA = f
			}
		}
	}
	if bestV == nil || bestA == nil {
		return nil, nil, errors.New("no playable video+audio streams found for this video")
	}
	return bestV, bestA, nil
}

// StreamURL plays any direct URL (music, mp4, direct links) in mpv. Used for
// audio and direct-download results that point straight at a playable file.
func StreamURL(ctx context.Context, url string) error {
	if url == "" {
		return errors.New("no playable URL for this result")
	}
	mpv, err := findMpv()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, mpv, "--cache=yes", "--force-window", "--no-terminal", url)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// bestMuxedFormat picks the highest-quality video format that already carries
// audio (no separate merge step needed for instant playback).
func bestMuxedFormat(v *youtube.Video) (youtube.Format, bool) {
	var best youtube.Format
	found := false
	for _, f := range v.Formats {
		if !strings.HasPrefix(f.MimeType, "video/") || f.AudioChannels == 0 {
			continue
		}
		if !found || f.Bitrate > best.Bitrate || (f.Bitrate == best.Bitrate && f.Quality != "") {
			best = f
			found = true
		}
	}
	return best, found
}

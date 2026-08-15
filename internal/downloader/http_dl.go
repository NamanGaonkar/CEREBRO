package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cerebro/internal/doh"
	"cerebro/internal/model"
)

// chunkCount is the number of parallel range-request workers (CEREBRO MAX:
// 16 lanes for maximum direct-download throughput).
const chunkCount = 16

// httpClient and probeClient route every request through the DoH transport
// so direct downloads bypass ISP-level tracker blocks. The probe uses a short
// timeout; the download client is long-lived for chunk transfers.
var (
	httpClient  = doh.NewClient(10 * time.Minute)
	probeClient = doh.NewClient(30 * time.Second)
)

// chunkMeta persists resume state for a chunked download.
type chunkMeta struct {
	URL   string  `json:"url"`
	Total int64   `json:"total"`
	Done  []int64 `json:"done"` // bytes finished per chunk
}

// downloadHTTP is the multi-threaded, resumable HTTP downloader for
// PDFs, comics and other direct links.
func (m *Manager) downloadHTTP(job *model.DownloadJob) {
	raw := job.Result.URL
	if raw == "" {
		m.fail(job, fmt.Errorf("no download URL"))
		return
	}
	url := resolveDownloadURL(raw)

	m.mutate(job.ID, func(j *model.DownloadJob) { j.Status = model.StatusDownloading })
	m.emit(job)

	total, acceptRanges := probeSize(url)

	outPath := filepath.Join(m.dir, filenameFromURL(url, job.Result))
	partPath := outPath + ".part"
	metaPath := outPath + ".meta"

	if total <= 0 || !acceptRanges {
		m.downloadSingleStream(job, url, outPath, partPath)
		return
	}

	meta := &chunkMeta{URL: url, Total: total}
	if err := loadChunkMeta(metaPath, meta); err != nil || meta.Total != total {
		meta = &chunkMeta{URL: url, Total: total, Done: make([]int64, chunkCount)}
	}

	m.mutate(job.ID, func(j *model.DownloadJob) {
		j.BytesTotal = total
		j.Result.Size = model.FormatBytes(total)
	})
	m.emit(job)

	f, err := os.OpenFile(partPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		m.fail(job, err)
		return
	}
	defer f.Close()

	// If the part file is missing or shorter than the expected size, the
	// persisted resume state is stale — discard it to avoid skipping bytes.
	if st, serr := f.Stat(); serr == nil && st.Size() < total {
		meta.Done = make([]int64, chunkCount)
	}
	if err := f.Truncate(total); err != nil {
		m.fail(job, err)
		return
	}

	chunkSize := (total + chunkCount - 1) / chunkCount
	if len(meta.Done) != chunkCount {
		meta.Done = make([]int64, chunkCount)
	}

	// Seed progress from the resumed chunk state so the bar doesn't restart at 0.
	var resumed int64
	for _, d := range meta.Done {
		resumed += d
	}
	m.mutate(job.ID, func(j *model.DownloadJob) { j.BytesDone = resumed })

	var metaMu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	recordErr := func(e error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		errMu.Unlock()
	}

	for i := int64(0); i < chunkCount; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > total {
			end = total
		}
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(i, start, end int64) {
			defer wg.Done()
			metaMu.Lock()
			pos := meta.Done[i] + start
			metaMu.Unlock()
			for attempt := 0; attempt < 3; attempt++ {
				err := m.downloadChunkRange(job, httpClient, url, i, pos, end, f, &metaMu, meta, metaPath)
				if err == nil {
					return
				}
				if attempt < 2 {
					time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
					metaMu.Lock()
					pos = meta.Done[i] + start
					metaMu.Unlock()
				} else {
					recordErr(err)
				}
			}
		}(i, start, end)
	}
	wg.Wait()

	var done int64
	for _, d := range meta.Done {
		done += d
	}
	m.mutate(job.ID, func(j *model.DownloadJob) { j.BytesDone = done })
	if done >= total {
		_ = os.Rename(partPath, outPath)
		_ = os.Remove(metaPath)
		m.complete(job, outPath)
		return
	}
	_ = saveChunkMeta(metaPath, meta)
	if firstErr != nil {
		m.fail(job, fmt.Errorf("download interrupted: %v", firstErr))
		return
	}
	m.fail(job, fmt.Errorf("incomplete download: %d/%d bytes", done, total))
}

// downloadChunkRange streams one byte range into the part file at the
// correct offset, tracking progress and persisted resume state.
func (m *Manager) downloadChunkRange(job *model.DownloadJob, client *http.Client, url string, idx, pos, end int64, f *os.File, metaMu *sync.Mutex, meta *chunkMeta, metaPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", pos, end-1))
	req.Header.Set("User-Agent", "cerebro/1.0")
	if strings.Contains(url, "libgen") {
		req.Header.Set("Referer", "https://libgen.is/")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return fmt.Errorf("chunk %d: server ignored Range request", idx)
	}
	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("chunk %d: HTTP %d", idx, resp.StatusCode)
	}
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.WriteAt(buf[:n], pos); werr != nil {
				return werr
			}
			pos += int64(n)
			metaMu.Lock()
			meta.Done[idx] += int64(n)
			if pos >= end {
				_ = saveChunkMeta(metaPath, meta)
				metaMu.Unlock()
				return nil
			}
			metaMu.Unlock()
			m.mutate(job.ID, func(j *model.DownloadJob) {
				j.BytesDone += int64(n)
				if j.BytesTotal > 0 {
					j.Progress = float64(j.BytesDone) / float64(j.BytesTotal)
				}
			})
		}
		if rerr != nil {
			if rerr == io.EOF {
				return fmt.Errorf("chunk %d: unexpected EOF", idx)
			}
			return rerr
		}
	}
}

// downloadSingleStream is the fallback for servers without range support.
func (m *Manager) downloadSingleStream(job *model.DownloadJob, url, outPath, partPath string) {
	client := httpClient
	f, err := os.OpenFile(partPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		m.fail(job, err)
		return
	}
	defer f.Close()

	cur, _ := f.Seek(0, io.SeekEnd)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		m.fail(job, err)
		return
	}
	req.Header.Set("User-Agent", "cerebro/1.0")
	if cur > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", cur))
	}
	resp, err := client.Do(req)
	if err != nil {
		m.fail(job, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK && cur > 0 {
		// The server ignored our resume Range; a fresh attempt is required.
		f.Close()
		_ = os.Remove(partPath)
		m.fail(job, fmt.Errorf("server does not support resume"))
		return
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		m.fail(job, fmt.Errorf("HTTP %d", resp.StatusCode))
		return
	}
	if resp.ContentLength > 0 {
		m.mutate(job.ID, func(j *model.DownloadJob) {
			j.BytesTotal = cur + resp.ContentLength
		})
		m.emit(job)
	}
	if _, err := io.Copy(f, io.TeeReader(resp.Body, &jobWriter{m: m, id: job.ID})); err != nil {
		m.fail(job, fmt.Errorf("download: %w", err))
		return
	}
	_ = os.Rename(partPath, outPath)
	m.complete(job, outPath)
}

// probeSize checks whether a URL supports range requests and its size.
// It uses its own short-timeout client so the long-lived download client is
// never clamped by the probe timeout.
func probeSize(url string) (total int64, acceptRanges bool) {
	client := probeClient
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("User-Agent", "cerebro/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusPartialContent {
		return resp.ContentLength, true
	}
	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		return resp.ContentLength, false
	}
	return 0, false
}

// resolveDownloadURL unwraps libgen ads/get pages into a direct file endpoint.
func resolveDownloadURL(raw string) string {
	if !strings.Contains(raw, "ads.php") && !strings.Contains(raw, "get.php") {
		return raw
	}
	md5 := extractMD5FromURL(raw)
	if md5 == "" {
		return raw
	}
	for _, host := range []string{"https://cdn1.libgen.is", "https://cdn2.libgen.is", "https://cdn3.libgen.is"} {
		u := host + "/get.php?md5=" + md5
		if probeOK(u) {
			return u
		}
	}
	return raw
}

// probeOK does a lightweight HEAD check that a URL serves a file.
func probeOK(u string) bool {
	req, err := http.NewRequest(http.MethodHead, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "cerebro/1.0")
	resp, err := probeClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func extractMD5FromURL(s string) string {
	i := strings.Index(s, "md5=")
	if i < 0 {
		return ""
	}
	rest := s[i+4:]
	if j := strings.IndexAny(rest, "&\"'"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

// filenameFromURL derives an output file name from the URL or result metadata.
func filenameFromURL(raw string, r model.SearchResult) string {
	clean := strings.Split(raw, "?")[0]
	clean = strings.TrimRight(clean, "/")
	tail := clean
	if i := strings.LastIndex(clean, "/"); i >= 0 {
		tail = clean[i+1:]
	}
	if tail == "" || strings.ContainsAny(tail, "=#") {
		tail = model.SanitizeFilename(r.Title)
		if r.Ext != "" && !strings.HasSuffix(strings.ToLower(tail), "."+r.Ext) {
			tail += "." + r.Ext
		}
	}
	return model.SanitizeFilename(tail)
}

func loadChunkMeta(path string, meta *chunkMeta) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, meta)
}

func saveChunkMeta(path string, meta *chunkMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

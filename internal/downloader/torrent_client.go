package downloader

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cerebro/internal/model"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

// torrentClient lazily builds the shared BitTorrent client.
func (m *Manager) torrentClient() (*torrent.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil {
		return m.client, nil
	}
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = m.dir
	cfg.Seed = true
	cfg.NoUpload = false
	cfg.Debug = false
	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	m.client = client
	return client, nil
}

// downloadTorrent streams a torrent (magnet or .torrent URL) into the
// downloads directory, reporting progress and peer counts.
func (m *Manager) downloadTorrent(job *model.DownloadJob) {
	client, err := m.torrentClient()
	if err != nil {
		m.fail(job, err)
		return
	}

	m.mutate(job.ID, func(j *model.DownloadJob) { j.Status = model.StatusResolving })
	m.emit(job)

	var t *torrent.Torrent
	switch {
	case strings.HasPrefix(job.Result.Magnet, "magnet:"):
		t, err = client.AddMagnet(model.EnhanceMagnet(job.Result.Magnet))
	case strings.HasPrefix(job.Result.URL, "http"):
		t, err = m.addTorrentFromURL(client, job.Result.URL)
	default:
		err = fmt.Errorf("no usable magnet or torrent URL")
	}
	if err != nil {
		m.fail(job, err)
		return
	}

	select {
	case <-t.GotInfo():
	case <-time.After(45 * time.Second):
		m.fail(job, fmt.Errorf("timed out fetching torrent metadata"))
		return
	}

	m.mutate(job.ID, func(j *model.DownloadJob) {
		j.BytesTotal = t.Length()
		j.Result.Size = model.FormatBytes(t.Length())
		j.Status = model.StatusDownloading
	})
	m.emit(job)

	for _, f := range t.Files() {
		f.Download()
	}

	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		<-tick.C
		done := t.BytesCompleted()
		st := t.Stats()
		m.mutate(job.ID, func(j *model.DownloadJob) {
			j.BytesDone = done
			if total := t.Length(); total > 0 {
				j.Progress = float64(done) / float64(total)
			}
			j.Peers = st.ActivePeers
		})
		if done >= t.Length() && t.Length() > 0 {
			m.mutate(job.ID, func(j *model.DownloadJob) { j.Status = model.StatusSeeding })
			m.emit(job)
			m.complete(job, m.dir)
			return
		}
		m.emit(job)
	}
}

// addTorrentFromURL downloads a .torrent file and adds it to the client.
func (m *Manager) addTorrentFromURL(client *torrent.Client, raw string) (*torrent.Torrent, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cerebro-max/2.0")
	// Route through DoH so .torrent fetches bypass ISP tracker blocks too.
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("torrent fetch: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	mi, err := metainfo.Load(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("invalid .torrent: %w", err)
	}
	return client.AddTorrent(mi)
}

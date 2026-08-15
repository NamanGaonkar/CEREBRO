// Package downloader owns the download engines: torrents, YouTube with ffmpeg
// merging, and chunked/resumable HTTP downloads.
package downloader

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"cerebro/internal/db"
	"cerebro/internal/model"
	"cerebro/internal/streamer"

	"github.com/anacrolix/torrent"
)

// Manager owns every active download and streams progress updates to the UI.
// All job mutations happen under the manager lock; the UI only ever sees
// defensive copies, so there are no data races with the download goroutines.
type Manager struct {
	dir     string
	mu      sync.Mutex
	client  *torrent.Client
	jobs    map[string]*model.DownloadJob
	updates chan *model.DownloadJob
	db      *db.DB
}

// NewManager creates a download manager writing into dir.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:     dir,
		jobs:    make(map[string]*model.DownloadJob),
		updates: make(chan *model.DownloadJob, 512),
	}
}

// Dir returns the download directory.
func (m *Manager) Dir() string { return m.dir }

// SetDB attaches the history/dedup store. Optional: without it the manager
// simply skips recording. Past downloads are loaded into the job list so the
// dashboard doubles as a persistent history view.
func (m *Manager) SetDB(d *db.DB) {
	m.db = d
	m.loadHistory()
}

// loadHistory seeds the job map with previously completed downloads (newest
// first) so the dashboard shows everything ever downloaded, not just this
// session. They render as "past download" rows.
func (m *Manager) loadHistory() {
	if m.db == nil {
		return
	}
	recs, err := m.db.Downloads()
	if err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range recs {
		id := "hist-" + r.ID
		if _, ok := m.jobs[id]; ok {
			continue
		}
		m.jobs[id] = &model.DownloadJob{
			ID:         id,
			Result:     model.SearchResult{Title: r.Title, Category: r.Category, URL: r.URL, Size: model.FormatBytes(r.Size), Source: "history"},
			Status:     model.StatusCompleted,
			Progress:   1,
			BytesTotal: r.Size,
			BytesDone:  r.Size,
			OutputPath: r.Path,
			StartedAt:  r.AddedAt,
		}
	}
}

// OwnedSet returns the set of already-downloaded keys (normalized titles and
// hashes) used to tag results with [OWNED].
func (m *Manager) OwnedSet() map[string]bool {
	if m.db == nil {
		return map[string]bool{}
	}
	owned, err := m.db.Owned()
	if err != nil {
		return map[string]bool{}
	}
	return owned
}

// Updates returns the channel on which job snapshots are streamed.
func (m *Manager) Updates() <-chan *model.DownloadJob { return m.updates }

// Jobs returns a defensive snapshot of all jobs, newest first.
func (m *Manager) Jobs() []*model.DownloadJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*model.DownloadJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		c := *j
		out = append(out, &c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}

// Start launches a download for the given result and returns its job.
// quality is only used for YouTube results.
func (m *Manager) Start(r model.SearchResult, quality string) (*model.DownloadJob, error) {
	job := &model.DownloadJob{
		ID:        model.NewID(),
		Result:    r,
		Status:    model.StatusQueued,
		StartedAt: time.Now(),
	}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	switch r.Category {
	case model.CatYouTube:
		go m.downloadYouTube(job, quality)
	case model.CatTorrent, model.CatGame:
		go m.downloadTorrent(job)
	case model.CatPDF, model.CatAudio, model.CatMP4, model.CatDirect, model.CatSoftware:
		go m.downloadHTTP(job)
	default:
		m.fail(job, fmt.Errorf("unsupported category %q", r.Category))
	}
	m.emit(job)
	return job, nil
}

// mutate applies fn to the live job under the manager lock.
func (m *Manager) mutate(id string, fn func(*model.DownloadJob)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok {
		fn(j)
	}
}

// emit pushes a defensive copy of the job to the updates channel.
func (m *Manager) emit(job *model.DownloadJob) {
	m.mu.Lock()
	c := *job
	m.mu.Unlock()
	select {
	case m.updates <- &c:
	default:
	}
}

func (m *Manager) fail(job *model.DownloadJob, err error) {
	m.mutate(job.ID, func(j *model.DownloadJob) {
		j.Status = model.StatusFailed
		j.Err = err
		j.FinishedAt = time.Now()
	})
	m.emit(job)
}

func (m *Manager) complete(job *model.DownloadJob, path string) {
	m.mutate(job.ID, func(j *model.DownloadJob) {
		j.Status = model.StatusCompleted
		j.Progress = 1
		j.BytesDone = j.BytesTotal
		j.OutputPath = path
		j.FinishedAt = time.Now()
	})
	m.emit(job)
	if m.db != nil {
		_ = m.db.RecordDownload(job.ID, job.Result.Title, job.Result.Category,
			path, model.ResultHash(job.Result), job.Result.URL, job.BytesTotal)
	}
}

// Stream plays a result immediately in mpv: torrent magnets pipe through the
// sequential piece streamer, YouTube videos stream their best playable format
// directly. The job shows as "streaming" while mpv is open.
func (m *Manager) Stream(r model.SearchResult) (*model.DownloadJob, error) {
	job := &model.DownloadJob{
		ID:        model.NewID(),
		Result:    r,
		Status:    model.StatusStreaming,
		StartedAt: time.Now(),
	}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var err error
		switch {
		case r.Category == model.CatYouTube:
			if r.VideoID == "" {
				err = fmt.Errorf("no video id to stream")
			} else {
				err = streamer.StreamYouTube(ctx, r.VideoID)
			}
		case r.Category == model.CatTorrent, r.Category == model.CatGame:
			if r.Magnet == "" {
				err = fmt.Errorf("no magnet link to stream")
			} else {
				err = streamer.Stream(ctx, r.Magnet, m.dir)
			}
		default:
			// Music, direct links, MP4s: hand the URL straight to mpv.
			if r.URL == "" {
				err = fmt.Errorf("no playable URL to stream")
			} else {
				err = streamer.StreamURL(ctx, r.URL)
			}
		}
		if err != nil {
			m.fail(job, err)
			return
		}
		m.complete(job, m.dir)
	}()
	m.emit(job)
	return job, nil
}

// RecordSearch logs a query in the search history store.
func (m *Manager) RecordSearch(q string) {
	if m.db != nil {
		_ = m.db.AddSearch(q)
	}
}

// Close releases background resources (torrent client, etc.).
func (m *Manager) Close() {
	m.mu.Lock()
	c := m.client
	m.client = nil
	m.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}

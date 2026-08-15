// Package model defines the shared data structures used across cerebro:
// search results, download jobs, category filters and live status info.
package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Result categories. Each category maps to a colored badge in the TUI.
const (
	CatTorrent  = "torrent"
	CatGame     = "game"
	CatYouTube  = "youtube"
	CatPDF      = "pdf" // books & documents of any format: pdf, epub, mobi, djvu, azw3, doc, txt, cbz…
	CatAudio    = "audio"
	CatMP4      = "mp4"
	CatDirect   = "direct"
	CatSoftware = "software" // GitHub repos/assets, ISOs, apps
)

// CategoryFilter is a UI-level category filter used by the Tab key.
type CategoryFilter int

const (
	FilterAll CategoryFilter = iota
	FilterBooks
	FilterGames
	FilterSoftware
	FilterVideo
	FilterAudio
	FilterArchives
)

// String returns the uppercase label used in the tab bar.
func (f CategoryFilter) String() string {
	switch f {
	case FilterBooks:
		return "BOOKS/PAPERS"
	case FilterGames:
		return "GAMES/REPACKS"
	case FilterSoftware:
		return "SOFTWARE/DEV"
	case FilterVideo:
		return "VIDEO"
	case FilterAudio:
		return "AUDIO"
	case FilterArchives:
		return "ARCHIVES"
	default:
		return "ALL"
	}
}

// Matches reports whether a result category is visible under this filter.
func (f CategoryFilter) Matches(cat string) bool {
	switch f {
	case FilterAll:
		return true
	case FilterBooks:
		return cat == CatPDF
	case FilterGames:
		return cat == CatGame
	case FilterSoftware:
		return cat == CatSoftware
	case FilterVideo:
		return cat == CatTorrent || cat == CatYouTube || cat == CatMP4
	case FilterAudio:
		return cat == CatAudio
	case FilterArchives:
		return cat == CatDirect
	}
	return false
}

// SearchResult is a unified, engine-agnostic search hit.
type SearchResult struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Category     string            `json:"category"`
	Size         string            `json:"size"`
	SizeBytes    int64             `json:"size_bytes"`
	Seeders      int               `json:"seeders"`
	Leechers     int               `json:"leechers"`
	Source       string            `json:"source"`
	URL          string            `json:"url"`
	Magnet       string            `json:"magnet"`
	VideoID      string            `json:"video_id"`
	Author       string            `json:"author"`
	Ext          string            `json:"ext"`
	ThumbnailURL string            `json:"thumbnail_url,omitempty"`
	Owned        bool              `json:"owned,omitempty"`
	QualityMap   map[string]string `json:"quality_map,omitempty"`
}

// Download job statuses.
const (
	StatusQueued      = "queued"
	StatusResolving   = "resolving"
	StatusDownloading = "downloading"
	StatusMerging     = "merging"
	StatusSeeding     = "seeding"
	StatusStreaming   = "streaming"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
)

// DownloadJob tracks a single active (or finished) download.
type DownloadJob struct {
	ID         string
	Result     SearchResult
	Status     string
	Progress   float64 // 0..1
	Speed      float64 // smoothed bytes/sec, updated by the UI
	BytesDone  int64
	BytesTotal int64
	Peers      int
	Err        error
	StartedAt  time.Time
	FinishedAt time.Time
	OutputPath string
}

// IsActive reports whether the job is still running.
func (j *DownloadJob) IsActive() bool {
	switch j.Status {
	case StatusQueued, StatusResolving, StatusDownloading, StatusMerging, StatusSeeding, StatusStreaming:
		return true
	}
	return false
}

// StatusInfo summarizes live network/IO state for the header bar.
type StatusInfo struct {
	ActiveJobs int
	Speed      float64
	DiskFree   uint64
	DiskTotal  uint64
	Query      string
	Searching  bool
}

// NewID returns a short random hex id used for search results and jobs.
func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ResultHash returns a stable dedup key for a result: the torrent infohash
// for magnet links, the video id for YouTube, or a short hash of the URL for
// direct links. It powers the [OWNED] tags in the UI.
func ResultHash(r SearchResult) string {
	if r.VideoID != "" {
		return "yt:" + r.VideoID
	}
	if i := strings.Index(r.Magnet, "btih:"); i >= 0 {
		rest := r.Magnet[i+len("btih:"):]
		if j := strings.IndexAny(rest, "&"); j >= 0 {
			rest = rest[:j]
		}
		return "btih:" + strings.ToUpper(strings.TrimSpace(rest))
	}
	if r.URL != "" {
		sum := sha256.Sum256([]byte(r.URL))
		return "url:" + hex.EncodeToString(sum[:8])
	}
	return ""
}

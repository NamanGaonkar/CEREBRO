package tui

import (
	"testing"

	"cerebro/internal/model"

	"github.com/charmbracelet/bubbles/progress"
)

// TestRenderResultsNoPanic guards against the "index out of range [-N]"
// panic caused by a negative window start when there are fewer results
// than visible rows (tall terminal windows).
func TestRenderResultsNoPanic(t *testing.T) {
	one := []model.SearchResult{{Title: "x-men", Category: model.CatTorrent}}

	// Few results in a very tall window — previously panicked.
	if out := renderResults(one, 0, 100, 80); out == "" {
		t.Fatal("expected rendered output for 1 result in a tall window")
	}

	// Empty list.
	if out := renderResults(nil, 0, 100, 80); out == "" {
		t.Fatal("expected empty-state output")
	}

	// Many results in a small window.
	many := make([]model.SearchResult, 50)
	for i := range many {
		many[i] = model.SearchResult{Title: "title", Category: model.CatTorrent}
	}
	if out := renderResults(many, 25, 100, 10); out == "" {
		t.Fatal("expected rendered output for a windowed slice")
	}

	// Cursor out of bounds should be clamped, not panic.
	if out := renderResults(many, 999, 100, 10); out == "" {
		t.Fatal("expected rendered output with out-of-range cursor")
	}
}

// TestRenderDownloadsNoPanic guards the same windowing math in the
// downloads dashboard.
func TestRenderDownloadsNoPanic(t *testing.T) {
	p := progress.New(progress.WithDefaultGradient(), progress.WithWidth(40))
	if out := renderDownloads(nil, p, 0, 0, 100, 80); out == "" {
		t.Fatal("expected empty downloads output")
	}
	jobs := []*model.DownloadJob{{ID: "a", Status: model.StatusDownloading}}
	if out := renderDownloads(jobs, p, 0, 0, 100, 80); out == "" {
		t.Fatal("expected rendered job row")
	}
}

package db

import (
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "cerebro.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	if err := d.RecordDownload("j1", "Interstellar (2014) 1080p", "torrent", "C:\\dl\\interstellar", "btih:ABC", "https://x", 2100000000); err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}
	// Same download re-recorded (idempotent upsert).
	if err := d.RecordDownload("j1", "Interstellar (2014) 1080p", "torrent", "C:\\dl\\interstellar", "btih:ABC", "https://x", 2100000000); err != nil {
		t.Fatalf("RecordDownload re-record: %v", err)
	}
	if err := d.RecordDownload("j2", "a  video", "youtube", "C:\\dl\\a.mp4", "yt:dQw4w9WgXcQ", "", 1048576); err != nil {
		t.Fatalf("RecordDownload 2: %v", err)
	}

	owned, err := d.Owned()
	if err != nil {
		t.Fatalf("Owned: %v", err)
	}
	for _, key := range []string{"interstellar (2014) 1080p", "btih:ABC", "a video", "yt:dQw4w9WgXcQ"} {
		if !owned[key] {
			t.Errorf("Owned missing key %q (got %v)", key, owned)
		}
	}
	if owned["never downloaded"] {
		t.Error("Owned should not contain unknown titles")
	}

	if err := d.AddSearch("avengers doomsday"); err != nil {
		t.Fatalf("AddSearch: %v", err)
	}
	if err := d.AddSearch("avengers doomsday"); err != nil {
		t.Fatalf("AddSearch 2: %v", err)
	}
}

func TestOpenCreatesDirs(t *testing.T) {
	p := filepath.Join(t.TempDir(), "deep", "nested", "cerebro.db")
	d, err := Open(p)
	if err != nil {
		t.Fatalf("Open with missing dirs: %v", err)
	}
	d.Close()
}

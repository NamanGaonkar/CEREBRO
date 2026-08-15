// Package db embeds a pure-Go SQLite store (modernc.org/sqlite) tracking
// downloaded files and search history, powering the [OWNED] dedup tags so
// users can see at a glance what they already have.
package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"cerebro/internal/model"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection. All methods are safe for concurrent use
// (database/sql pools connections internally).
type DB struct {
	conn *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS downloads (
    id       TEXT PRIMARY KEY,
    title    TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    path     TEXT NOT NULL DEFAULT '',
    hash     TEXT NOT NULL DEFAULT '',
    url      TEXT NOT NULL DEFAULT '',
    size     INTEGER NOT NULL DEFAULT 0,
    added_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS searches (
    query   TEXT PRIMARY KEY,
    count   INTEGER NOT NULL DEFAULT 1,
    last_at TEXT NOT NULL
);
`

// Open opens (creating if needed) the cerebro database at path.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// WAL keeps reads/writes fast and avoids locking the file on Windows.
	_, _ = conn.Exec(`PRAGMA journal_mode=WAL;`)
	if _, err := conn.Exec(schema); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &DB{conn: conn}, nil
}

// Close releases the database.
func (d *DB) Close() error { return d.conn.Close() }

// RecordDownload stores a finished download for dedup and history. title is
// normalized with TitleCase so [OWNED] matching is case/whitespace tolerant.
func (d *DB) RecordDownload(id, title, category, path, hash, url string, size int64) error {
	_, err := d.conn.Exec(
		`INSERT INTO downloads (id, title, category, path, hash, url, size, added_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET title=excluded.title, path=excluded.path`,
		id, model.TitleCase(title), category, path, hash, url, size,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// Owned returns the set of ownership keys — normalized titles and hashes —
// that have been downloaded at least once. An empty set means nothing recorded.
func (d *DB) Owned() (map[string]bool, error) {
	rows, err := d.conn.Query(`SELECT title, hash FROM downloads`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var title, hash string
		if err := rows.Scan(&title, &hash); err != nil {
			return nil, err
		}
		if title != "" {
			out[title] = true
		}
		if hash != "" {
			out[hash] = true
		}
	}
	return out, rows.Err()
}

// Download is one recorded download (history entry).
type Download struct {
	ID       string
	Title    string
	Category string
	Path     string
	URL      string
	Size     int64
	AddedAt  time.Time
}

// Downloads lists recorded downloads, newest first — the history shown in the
// downloads dashboard.
func (d *DB) Downloads() ([]Download, error) {
	rows, err := d.conn.Query(`SELECT id, title, category, path, url, size, added_at FROM downloads ORDER BY added_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Download
	for rows.Next() {
		var dl Download
		var added string
		if err := rows.Scan(&dl.ID, &dl.Title, &dl.Category, &dl.Path, &dl.URL, &dl.Size, &added); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, added); err == nil {
			dl.AddedAt = t
		}
		out = append(out, dl)
	}
	return out, rows.Err()
}

// AddSearch records a query in the search history (upsert with a counter).
func (d *DB) AddSearch(q string) error {
	_, err := d.conn.Exec(
		`INSERT INTO searches (query, count, last_at) VALUES (?, 1, ?)
		 ON CONFLICT(query) DO UPDATE SET count = count + 1, last_at = excluded.last_at`,
		q, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

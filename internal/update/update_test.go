package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		name    string
		current string
		tag     string
		want    bool
	}{
		{"newer patch", "v1.5.0", "v1.5.1", true},
		{"newer minor", "v1.4.0", "v1.5.0", true},
		{"newer major", "v1.9.9", "v2.0.0", true},
		{"equal", "v1.5.0", "v1.5.0", false},
		{"older", "v1.6.0", "v1.5.0", false},
		{"no v prefix", "1.5.0", "1.6.0", true},
		{"dev current never outdated", "dev", "v1.5.0", false},
		{"garbage tag", "v1.5.0", "latest", false},
		{"garbage current", "vX.5.0", "v1.5.0", false},
		{"two-part version", "v1.5", "v1.5.1", false},
	}
	for _, c := range cases {
		if got := NewerVersion(c.current, c.tag); got != c.want {
			t.Errorf("%s: NewerVersion(%q, %q) = %v, want %v", c.name, c.current, c.tag, got, c.want)
		}
	}
}

// Tests here temporarily swap the package-level CurrentVersion/apiBase globals.
// Do NOT add t.Parallel() to any test in this file, or they will race.
func TestCheckLatest(t *testing.T) {
	oldVer, oldBase := CurrentVersion, apiBase
	defer func() { CurrentVersion, apiBase = oldVer, oldBase }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release{TagName: "v9.9.9"})
	}))
	defer srv.Close()
	apiBase = srv.URL

	// Newer release published.
	CurrentVersion = "v1.5.0"
	got, err := CheckLatest(context.Background(), srv.Client())
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if got != "9.9.9" {
		t.Errorf("CheckLatest = %q, want %q", got, "9.9.9")
	}

	// Local dev build stays quiet even when the server reports a newer tag.
	CurrentVersion = "dev"
	if got, err = CheckLatest(context.Background(), srv.Client()); err != nil {
		t.Fatalf("CheckLatest(dev): %v", err)
	}
	if got != "" {
		t.Errorf("CheckLatest(dev) = %q, want empty", got)
	}

	// Up to date → empty.
	CurrentVersion = "v9.9.9"
	if got, err = CheckLatest(context.Background(), srv.Client()); err != nil {
		t.Fatalf("CheckLatest(up-to-date): %v", err)
	}
	if got != "" {
		t.Errorf("CheckLatest(up-to-date) = %q, want empty", got)
	}

	// Non-200 response surfaces as an error (the UI swallows it silently).
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer bad.Close()
	apiBase = bad.URL
	CurrentVersion = "v1.5.0"
	if _, err := CheckLatest(context.Background(), bad.Client()); err == nil {
		t.Error("expected an error for a non-200 response")
	}
}

func TestNotice(t *testing.T) {
	oldVer := CurrentVersion
	defer func() { CurrentVersion = oldVer }()
	CurrentVersion = "v1.5.0"

	msg := Notice("1.6.0")
	if !strings.Contains(msg, "1.6.0") || !strings.Contains(msg, "1.5.0") {
		t.Errorf("Notice = %q, want it to mention both versions", msg)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(msg, "scoop update cerebro") || !strings.Contains(msg, "winget install ScoopInstaller.Scoop") {
			t.Errorf("Notice on Windows = %q, want Scoop + winget hints", msg)
		}
	} else if !strings.Contains(msg, "install.sh") {
		t.Errorf("Notice on %s = %q, want the installer re-run hint", runtime.GOOS, msg)
	}
}

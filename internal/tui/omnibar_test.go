package tui

import "testing"

func TestClassifyInput(t *testing.T) {
	cases := []struct {
		in   string
		want inputKind
	}{
		{"avengers doomsday", inputSearch},
		{"magnet:?xt=urn:btih:ABC&dn=test", inputMagnet},
		{"MAGNET:?xt=urn:btih:ABC", inputMagnet},
		{"https://youtube.com/watch?v=dQw4w9WgXcQ", inputYouTube},
		{"https://youtu.be/dQw4w9WgXcQ", inputYouTube},
		{"https://www.youtube.com/shorts/dQw4w9WgXcQ", inputYouTube},
		{"https://example.com/file.zip", inputURL},
		{"http://example.com/file.zip", inputURL},
	}
	for _, c := range cases {
		if got := classifyInput(c.in); got != c.want {
			t.Errorf("classifyInput(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestYouTubeID(t *testing.T) {
	if got := youtubeID("https://youtube.com/watch?v=dQw4w9WgXcQ&t=10s"); got != "dQw4w9WgXcQ" {
		t.Errorf("youtubeID = %q", got)
	}
	if got := youtubeID("https://example.com/not-youtube"); got != "" {
		t.Errorf("expected empty id, got %q", got)
	}
}

func TestMagnetName(t *testing.T) {
	if got := magnetName("magnet:?xt=urn:btih:ABC&dn=Interstellar+2014"); got != "Interstellar 2014" {
		t.Errorf("magnetName = %q", got)
	}
	if got := magnetName("magnet:?xt=urn:btih:ABC"); got != "magnet link" {
		t.Errorf("magnetName fallback = %q", got)
	}
}

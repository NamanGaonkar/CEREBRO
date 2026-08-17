package intel

import "testing"

func TestUnwrapDDG(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://github.com/torvalds", "https://github.com/torvalds"},
		{"/l/?uddg=https%3A%2F%2Fen.wikipedia.org%2Fwiki%2FLinus_Torvalds&rut=abc", "https://en.wikipedia.org/wiki/Linus_Torvalds"},
		{"//duckduckgo.com/l/?uddg=https%3A%2F%2Fx.com%2Ftorvalds", "https://x.com/torvalds"},
		{"javascript:void(0)", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := unwrapDDG(c.in); got != c.want {
			t.Errorf("unwrapDDG(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseMojeek(t *testing.T) {
	body := `<html><body>
<ul class="results-standard">
<li><h2><a class="title" href="https://en.wikipedia.org/wiki/Linus_Torvalds">Linus Torvalds - Wikipedia</a></h2>
<p class="s">Linus Benedict Torvalds is a Finnish-American software engineer.</p></li>
<li><h2><a class="title" href="https://github.com/torvalds">torvalds - GitHub</a></h2>
<p class="s">Linux kernel creator.</p></li>
</ul>
</body></html>`
	hits := parseMojeek(body, 10)
	if len(hits) != 2 {
		t.Fatalf("parseMojeek: got %d hits, want 2", len(hits))
	}
	if hits[0].URL != "https://en.wikipedia.org/wiki/Linus_Torvalds" {
		t.Errorf("hit 0 URL = %q", hits[0].URL)
	}
	if hits[0].Title != "Linus Torvalds - Wikipedia" {
		t.Errorf("hit 0 title = %q", hits[0].Title)
	}
	if hits[0].Snippet == "" {
		t.Error("hit 0 snippet should be extracted")
	}
	if hits[1].URL != "https://github.com/torvalds" {
		t.Errorf("hit 1 URL = %q", hits[1].URL)
	}
}

func TestParseMojeekLimit(t *testing.T) {
	body := `<html><body>`
	for i := 0; i < 5; i++ {
		body += `<h2><a class="title" href="https://example.com/` + string(rune('a'+i)) + `">title</a></h2><p class="s">snip</p>`
	}
	body += `</body></html>`
	if got := len(parseMojeek(body, 3)); got != 3 {
		t.Errorf("parseMojeek limit: got %d hits, want 3", got)
	}
}

func TestHostOf(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://github.com/torvalds", "github.com"},
		{"http://www.reddit.com/user/torvalds", "reddit.com"},
		{"https://en.wikipedia.org/wiki/Amoeba", "en.wikipedia.org"},
	}
	for _, c := range cases {
		if got := hostOf(c.in); got != c.want {
			t.Errorf("hostOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

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

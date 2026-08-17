package intel

import (
	"strings"
	"testing"
)

func TestPlatformRegistry(t *testing.T) {
	if platformCount() < 80 {
		t.Fatalf("expected 80+ platforms, got %d", platformCount())
	}
	for _, p := range platforms {
		if p.name == "" || p.url == nil {
			t.Errorf("platform missing name or url builder: %+v", p)
			continue
		}
		u := p.url("testuser")
		if !strings.HasPrefix(u, "http") {
			t.Errorf("platform %s produced bad url %q", p.name, u)
		}
	}
}

func TestStatusChecks(t *testing.T) {
	cases := []struct {
		name string
		chk  func(int, string) Status
		code int
		body string
		want Status
	}{
		{"default-404", defaultCheck, 404, "", StatusNotFound},
		{"default-410", defaultCheck, 410, "", StatusNotFound},
		{"default-200", defaultCheck, 200, "", StatusFound},
		{"default-302", defaultCheck, 302, "", StatusFound},
		{"default-403", defaultCheck, 403, "", StatusUnverified},
		{"github-404", githubCheck, 404, "", StatusNotFound},
		{"github-200", githubCheck, 200, "{}", StatusFound},
		{"yt-soft404", notFoundText("This channel doesn't exist"), 200, "This channel doesn't exist", StatusNotFound},
		{"yt-found", notFoundText("This channel doesn't exist"), 200, "welcome to my channel", StatusFound},
		{"json-empty", jsonListCheck, 200, "[]", StatusNotFound},
		{"json-found", jsonListCheck, 200, "[{\"id\":1}]", StatusFound},
		{"json-bad", jsonListCheck, 429, "rate limited", StatusUnverified},
		{"unverified-200", unverifiedCheck, 200, "login required", StatusUnverified},
		{"unverified-404", unverifiedCheck, 404, "", StatusNotFound},
	}
	for _, c := range cases {
		if got := c.chk(c.code, c.body); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestNormalizeHandle(t *testing.T) {
	cases := map[string]string{
		"John Doe":    "john-doe",
		"  Torvalds ": "torvalds",
		"Jane_Doe":    "jane_doe",
		"UPPER":       "upper",
	}
	for in, want := range cases {
		if got := NormalizeHandle(in); got != want {
			t.Errorf("NormalizeHandle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsPersonName(t *testing.T) {
	if !IsPersonName("John Doe") {
		t.Error("two alpha words should classify as a person name")
	}
	if !IsPersonName("Ada Lovelace King") {
		t.Error("three words should classify as a person name")
	}
	if IsPersonName("torvalds") {
		t.Error("single word is a handle, not a person name")
	}
	if IsPersonName("john123 doe") {
		t.Error("digits disqualify a person name")
	}
}

func TestClassifyKind(t *testing.T) {
	if got := ClassifyKind("John Doe"); got != KindPerson {
		t.Errorf("name → %s, want person", got)
	}
	if got := ClassifyKind("torvalds"); got != KindUsername {
		t.Errorf("handle → %s, want username", got)
	}
}

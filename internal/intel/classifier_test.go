package intel

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		in   string
		want Intent
	}{
		// @ and ? prefixes force the mode.
		{"@torvalds", IntentHandle},
		{"@jane_doe", IntentHandle},
		{"?quantum computing", IntentConcept},
		{"?what is amoeba", IntentConcept},
		// @ before a FULL NAME = person recon (strip @, reclassify).
		{"@Naman Gaonkar", IntentPerson},
		{"@elon musk", IntentPerson},
		{"@what is amoeba", IntentConcept},
		// Single bare token = handle.
		{"torvalds", IntentHandle},
		{"docker", IntentHandle},
		{"kartik", IntentHandle},
		{"jane.doe_1", IntentHandle},
		// Multi-word with concept stopwords = topic (never username probes).
		{"what is amoeba", IntentConcept},
		{"how does photosynthesis work", IntentConcept},
		{"definition of entropy", IntentConcept},
		{"best laptop for coding", IntentConcept},
		{"types of machine learning", IntentConcept},
		// Multi-word lowercase phrases = topic.
		{"quantum computing", IntentConcept},
		{"machine learning", IntentConcept},
		{"black holes", IntentConcept},
		// Multi-word, all name-like = person.
		{"Elon Musk", IntentPerson},
		{"Linus Torvalds", IntentPerson},
		{"Kartik Sharma", IntentPerson},
		{"Ada Lovelace King", IntentPerson},
		// Edge cases.
		{"", IntentHandle},
		{"  ", IntentHandle},
	}
	for _, c := range cases {
		if got := Classify(c.in); got != c.want {
			t.Errorf("Classify(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestClassifyNeverPersonForConcepts(t *testing.T) {
	// Regression: these must NEVER be treated as usernames/person names,
	// otherwise the recon would fabricate github.com/what-is-amoeba hits.
	for _, q := range []string{"what is amoeba", "?quantum computing", "how to learn golang", "best headphones 2024"} {
		if got := Classify(q); got == IntentHandle {
			t.Errorf("concept %q classified as handle — would fabricate username probes", q)
		}
	}
}

func platformByName(name string) platform {
	for _, p := range platforms {
		if p.name == name {
			return p
		}
	}
	return platform{}
}

func TestWikiMatch(t *testing.T) {
	cases := []struct {
		query string
		title string
		want  bool
	}{
		{"Elon Musk", "Elon Musk", true},
		{"torvalds", "Linus Torvalds", true},
		{"Kartik Sharma", "Kartik Sharma (actor)", true},
		{"quantum computing", "Quantum computing", true},
		{"Elon Musk", "Elon Musk Jr.", true},
		{"what is amoeba", "Amoeba (disambiguation)", true}, // stopwords skipped, significant token matches
		{"what is amoeba", "Quantum mechanics", false},      // unrelated top hit must not match
		{"John Smith", "John Smithers", false},
		{"Ada Lovelace King", "Ada Lovelace", false}, // missing a significant token
	}
	for _, c := range cases {
		if got := wikiMatch(c.query, c.title); got != c.want {
			t.Errorf("wikiMatch(%q, %q) = %v, want %v", c.query, c.title, got, c.want)
		}
	}
}

func TestSoft404FalsePositives(t *testing.T) {
	// Sites that answer 200 but embed a soft-404 marker must report NOT FOUND.
	soft404 := []struct {
		name string
		body string
	}{
		{"GitHub", "Page not found"},
		{"Reddit", "Sorry, nobody on Reddit goes by that name"},
		{"X / Twitter", "Hmm...this page doesn't exist"},
		{"Instagram", "Sorry, this page isn't available"},
		{"TikTok", "Couldn't find this account"},
		{"Steam", "The specified profile could not be found"},
		{"YouTube", "This channel doesn't exist"},
		{"Twitch", "Sorry. Unless you've got a time machine"},
		{"Hacker News", "No such user"},
	}
	for _, c := range soft404 {
		p := platformByName(c.name)
		if p.check == nil {
			t.Fatalf("platform %q not found in registry", c.name)
		}
		if got := p.check(200, c.body); got != StatusNotFound {
			t.Errorf("%s soft-404: got %s, want NOT FOUND", c.name, got)
		}
	}
	// A real profile page must still be FOUND.
	if got := platformByName("GitHub").check(200, "torvalds repositories and followers"); got != StatusFound {
		t.Errorf("GitHub real page: got %s, want FOUND", got)
	}
	if got := platformByName("Reddit").check(200, "torvalds on reddit"); got != StatusFound {
		t.Errorf("Reddit real page: got %s, want FOUND", got)
	}
}

func TestLoginWallNeverFalseFound(t *testing.T) {
	// Login-walled sites (Instagram, X, Facebook, LinkedIn) answer 200 for
	// everything — a bare 200 must be UNVERIFIED, never a false FOUND.
	for _, name := range []string{"X / Twitter", "Instagram", "Facebook", "LinkedIn"} {
		p := platformByName(name)
		if got := p.check(200, "we need you to log in to continue"); got != StatusUnverified {
			t.Errorf("%s login wall: got %s, want UNVERIFIED", name, got)
		}
		if got := p.check(404, ""); got != StatusNotFound {
			t.Errorf("%s hard 404: got %s, want NOT FOUND", name, got)
		}
	}
	// But a soft-404 marker still wins.
	if got := platformByName("Instagram").check(200, "Sorry, this page isn't available"); got != StatusNotFound {
		t.Errorf("Instagram soft-404 marker: got %s, want NOT FOUND", got)
	}
}

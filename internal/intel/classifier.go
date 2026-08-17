package intel

import "strings"

// Intent is the strict recon mode a query maps to before any network work.
type Intent int

const (
	// IntentHandle probes 90+ social/developer platforms for a username
	// (@handle or a single bare handle like "torvalds").
	IntentHandle Intent = iota
	// IntentConcept is a topic / definition / general-knowledge query
	// (?topic or phrases like "what is amoeba", "quantum computing"). NO
	// username probes run — only Wikipedia + DuckDuckGo synthesis.
	IntentConcept
	// IntentPerson is a full name / entity ("Elon Musk", "Kartik Sharma"):
	// probes profiles but disambiguates through Wikipedia verification.
	IntentPerson
)

// String returns the human label used in the dashboard kind badge.
func (i Intent) String() string {
	switch i {
	case IntentConcept:
		return "topic"
	case IntentPerson:
		return "person"
	default:
		return "username"
	}
}

// stopwords mark a query as a concept/definition rather than a handle, so
// "what is amoeba" never probes github.com/what-is-amoeba.
var conceptStopwords = []string{
	"what", "what's", "whats", "is", "are", "was", "were", "the", "a", "an",
	"of", "how", "why", "when", "where", "who", "does", "do", "define",
	"definition", "meaning", "explain", "explained", "about", "examples",
	"example", "history", "origins", "vs", "versus", "and", "or", "for",
	"to", "in", "on", "with", "without", "best", "top", "list", "types",
	"type", "science", "engineering", "concept", "theory", "formula",
}

// Classify maps a raw query to a strict recon Intent. The leading @/? hints
// force the mode; otherwise heuristics decide:
//
//   - single bare token (letters/digits/._-)            → Handle
//   - multi-word with concept stopwords or lowercase    → Concept
//   - multi-word, all words name-like (letters)         → Person
//
// A @ hint is stripped first, so "@Naman Gaonkar" classifies as a PERSON
// (full name) while "@torvalds" stays a username — the @ is a mode marker,
// not a promise that the rest is a bare handle.
func Classify(q string) Intent {
	trimmed := strings.TrimSpace(q)
	if trimmed == "" {
		return IntentHandle
	}
	if strings.HasPrefix(trimmed, "?") {
		return IntentConcept
	}
	if strings.HasPrefix(trimmed, "@") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
		if trimmed == "" {
			return IntentHandle
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 1 {
			return IntentHandle
		}
		// Multi-word @ query: the @ is an explicit recon request, so any
		// name-shaped phrase is a person ("@elon musk", "@Naman Gaonkar")
		// regardless of case; stopword phrases ("@what is amoeba") stay
		// topics and never probe username profiles.
		for _, f := range fields {
			if isConceptStopword(f) {
				return IntentConcept
			}
		}
		if allNameTokens(fields) {
			return IntentPerson
		}
		return IntentConcept
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 1 {
		return IntentHandle
	}

	// Multi-word. Concept stopwords ("what is", "best") always mean topic.
	for _, f := range fields {
		if isConceptStopword(f) {
			return IntentConcept
		}
	}
	// A name-shaped phrase is a PERSON only in Title Case ("Elon Musk",
	// "Kartik Sharma"); the same words lowercase are a topic phrase
	// ("black holes", "machine learning").
	if allNameTokens(fields) && isTitleCase(fields) {
		return IntentPerson
	}
	return IntentConcept
}

// isTitleCase reports whether every word starts with an uppercase letter —
// the shape of a proper name ("Elon Musk") vs a lowercase topic phrase
// ("black holes").
func isTitleCase(fields []string) bool {
	for _, f := range fields {
		first := rune(0)
		for _, r := range f {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				first = r
				break
			}
		}
		if first == 0 || first < 'A' || first > 'Z' {
			return false
		}
	}
	return true
}

// isConceptStopword reports whether the (lowercased) word is a concept marker.
func isConceptStopword(w string) bool {
	w = strings.ToLower(strings.Trim(w, ".,:;!?"))
	for _, s := range conceptStopwords {
		if w == s {
			return true
		}
	}
	return false
}

// allNameTokens reports whether every field is made of letters, periods or
// hyphens only (a person-name shape), mirroring IsPersonName.
func allNameTokens(fields []string) bool {
	for _, f := range fields {
		for _, r := range f {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '.' || r == '-' || r == '\'') {
				return false
			}
		}
	}
	return true
}

// ClassifyKind is the legacy string form kept for callers/tests.
func ClassifyKind(target string) string {
	return Classify(target).String()
}

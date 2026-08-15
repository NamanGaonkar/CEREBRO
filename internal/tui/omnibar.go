package tui

import (
	"net/url"
	"regexp"
	"strings"
)

// inputKind classifies what the user typed into the fuzzy omnibar.
type inputKind int

const (
	inputSearch inputKind = iota
	inputMagnet
	inputYouTube
	inputURL
	inputIntel
)

var ytIDRe = regexp.MustCompile(`(?:v=|youtu\.be/|shorts/|embed/)([A-Za-z0-9_-]{11})`)

// classifyInput decides what the omnibar text is: a plain query, a magnet
// link, a YouTube URL, or some other URL.
func classifyInput(q string) inputKind {
	// Cerebro Intel: @handle probes accounts across platforms, ?topic runs a
	// knowledge/entity deep search.
	if strings.HasPrefix(q, "@") || strings.HasPrefix(q, "?") {
		return inputIntel
	}
	switch {
	case strings.HasPrefix(strings.ToLower(q), "magnet:?"):
		return inputMagnet
	case strings.HasPrefix(q, "http://"), strings.HasPrefix(q, "https://"):
		if ytIDRe.MatchString(q) {
			return inputYouTube
		}
		return inputURL
	}
	return inputSearch
}

// youtubeID extracts a video id from a YouTube URL.
func youtubeID(q string) string {
	m := ytIDRe.FindStringSubmatch(q)
	if m == nil {
		return ""
	}
	return m[1]
}

// magnetName pulls the display name (dn=) out of a magnet link.
func magnetName(q string) string {
	for _, part := range strings.Split(q, "&") {
		if strings.HasPrefix(part, "dn=") {
			if name, err := url.QueryUnescape(part[3:]); err == nil && name != "" {
				return name
			}
			return part[3:]
		}
	}
	return "magnet link"
}

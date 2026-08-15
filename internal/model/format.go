package model

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// FormatBytes renders a byte count as a human readable size string.
func FormatBytes(n int64) string {
	if n < 0 {
		return "?"
	}
	f := float64(n)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

// FormatSpeed renders a bytes-per-second rate as a human readable string.
func FormatSpeed(bps float64) string {
	if bps < 0 {
		return "0 B/s"
	}
	return FormatBytes(int64(bps)) + "/s"
}

// Truncate shortens s to at most n runes, appending an ellipsis if cut.
// Control characters are collapsed so multi-line titles stay on one row.
func Truncate(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return ' '
		}
		return r
	}, s)
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(runes[:n-1]) + "…"
}

var (
	illegalRe = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	spaceRe   = regexp.MustCompile(`\s+`)
)

// SanitizeFilename turns an arbitrary title into a safe file name.
func SanitizeFilename(s string) string {
	s = illegalRe.ReplaceAllString(s, "_")
	s = spaceRe.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ". ")
	// Keep a sensible cap so paths never explode.
	runes := []rune(s)
	if len(runes) > 180 {
		s = string(runes[:180])
	}
	if s == "" {
		return "download"
	}
	return s
}

// TitleCase normalizes whitespace on titles for dedup comparisons.
func TitleCase(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// IsPrintable reports whether every rune in s is printable (titles with
// heavy control characters are rejected by the UI to keep the terminal clean).
func IsPrintable(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\t' {
			continue
		}
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

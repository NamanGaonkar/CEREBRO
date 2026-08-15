package tui

import (
	"fmt"
	"strings"

	"cerebro/internal/model"

	"github.com/charmbracelet/lipgloss"
)

// truncW truncates s to w cells, ANSI-aware (fit() counts raw runes and would
// slice through escape sequences on styled lines).
func truncW(s string, w int) string {
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}

// renderInfoPane renders the left preview pane of the MAX split layout as a
// crisp, high-contrast metadata card: category pill, wrapped bold title,
// source line, a bordered specs box (size / format / source / health) and
// action hints — pure text, so the results table can never be corrupted.
func renderInfoPane(r model.SearchResult, paneW, maxH int) string {
	artW := paneW - 4 // borders 2 + horizontal padding 2
	if artW < 12 {
		artW = 12
	}
	if maxH < 4 {
		maxH = 4
	}

	// Category pill + dedup tag.
	badge := BadgeStyle(r.Category).Render("[" + BadgeLabel(r.Category) + "]")
	if r.Owned {
		badge += "  " + okStyle.Render("OWNED")
	}
	badge = truncW(badge, artW)

	// Title: bold crisp white, wrapped to the pane width.
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Bold(true).Render(r.Title)
	titleLines := wrapLines(title, artW, 3)

	// Source line (author for YouTube, seeds+indexer for torrents…).
	source := truncW(accentStyle.Render(sourceLine(r)), artW)

	// Specs box, as its own section so it is dropped whole, never cut mid-box.
	specs := strings.Split(specsBox(r, artW), "\n")

	// Action hints (least important section — dropped first when tight).
	hints := []string{
		truncW(dimStyle.Render("p stream · m copy · enter ⤓"), artW),
	}

	// Section-based height budget: maxH-2 lines fit inside the box borders.
	// Whole sections are dropped from the bottom (hints → source → specs) and
	// the title shrinks line-by-line, so a box border is NEVER sliced through
	// — the card always closes cleanly on short terminals.
	avail := maxH - 2
	sections := [][]string{{badge}, titleLines, {source}, specs, hints}
	total := func() int {
		n := 0
		for _, s := range sections {
			n += len(s)
		}
		return n
	}
	// Drop whole trailing sections (hints, then source, then the specs box);
	// the badge and title are the card's core and stay.
	for total() > avail && len(sections) > 2 {
		sections = sections[:len(sections)-1]
	}
	// Only shrink the title as a last resort (still too tall).
	for len(sections[1]) > 1 && total() > avail {
		sections[1] = sections[1][:len(sections[1])-1]
	}
	var lines []string
	for _, s := range sections {
		lines = append(lines, s...)
	}
	// Width(paneW) is the total box width; no MaxWidth(paneW) — that slices
	// the right border + corners off (the misshapen-card bug). The section
	// budget above already bounds the height, so no MaxHeight is needed.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(subdued)).
		Padding(0, 1).
		Width(paneW).
		Height(len(lines)).
		Render(strings.Join(lines, "\n"))
}

// wrapLines wraps s to w cells and returns at most max lines, ellipsizing the
// final line when the text continues. ANSI escapes are handled by lipgloss.
func wrapLines(s string, w, max int) []string {
	if w < 1 {
		w = 1
	}
	if max < 1 {
		max = 1
	}
	wrapped := lipgloss.NewStyle().Width(w).Render(s)
	lines := strings.Split(wrapped, "\n")
	if len(lines) <= max {
		return lines
	}
	out := lines[:max]
	// Mark the overflow so the user knows the title continues.
	out[max-1] = truncW(strings.TrimRight(out[max-1], " ")+"…", w)
	return out
}

// sourceLine summarizes the result's provenance in one accent line.
func sourceLine(r model.SearchResult) string {
	switch r.Category {
	case model.CatTorrent, model.CatGame:
		return fmt.Sprintf("%d▲ %s", r.Seeders, sourceName(r.Source))
	case model.CatYouTube:
		if r.Author != "" {
			return r.Author
		}
		return "YouTube"
	default:
		return sourceName(r.Source)
	}
}

// sourceName prettifies a source slug ("yts" → "YTS", "libgen" → "LibGen"…).
func sourceName(s string) string {
	switch strings.ToLower(s) {
	case "yts":
		return "YTS"
	case "1337x":
		return "1337x"
	case "nyaa":
		return "Nyaa"
	case "tpb", "pirate bay":
		return "Pirate Bay"
	case "youtube":
		return "YouTube"
	case "libgen":
		return "LibGen"
	case "gutenberg":
		return "Gutenberg"
	case "ia", "archive", "internet archive":
		return "Internet Archive"
	case "fitgirl":
		return "FitGirl"
	case "github":
		return "GitHub"
	case "magnet":
		return "Magnet"
	}
	if s == "" {
		return "direct"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// specsBox renders the bordered specs panel: size, format, source, health.
func specsBox(r model.SearchResult, width int) string {
	inner := width - 4 // box borders 2 + padding 2
	if inner < 16 {
		inner = 16
	}
	rows := [][2]string{
		{"SIZE", r.Size},
		{"FORMAT", formatLabel(r)},
		{"SOURCE", sourceName(r.Source)},
		{"HEALTH", healthLabel(r)},
	}
	var sb strings.Builder
	for i, row := range rows {
		label := lipgloss.NewStyle().Foreground(lipgloss.Color(orange)).Bold(true).Render(row[0])
		value := lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render(row[1])
		sb.WriteString(truncW(col(label, 10)+value, inner))
		if i < len(rows)-1 {
			sb.WriteString("\n")
		}
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(accentDim)).
		Padding(0, 1).
		Width(width).
		Render(sb.String())
}

// formatLabel returns the file format shown in the specs box.
func formatLabel(r model.SearchResult) string {
	if r.Ext != "" {
		return strings.ToUpper(r.Ext)
	}
	switch r.Category {
	case model.CatTorrent, model.CatGame:
		return "TORRENT"
	case model.CatYouTube:
		return "STREAM"
	default:
		return "FILE"
	}
}

// healthLabel colors a status/health line: seed counts for torrents, verified
// links for direct sources.
func healthLabel(r model.SearchResult) string {
	switch r.Category {
	case model.CatTorrent, model.CatGame:
		switch {
		case r.Seeders >= 50:
			return okStyle.Render("● high seed")
		case r.Seeders > 0:
			return okStyle.Render(fmt.Sprintf("● %d seeders", r.Seeders))
		default:
			return errStyle.Render("○ no seeders")
		}
	case model.CatYouTube:
		return okStyle.Render("● direct stream")
	default:
		return okStyle.Render("● verified link")
	}
}

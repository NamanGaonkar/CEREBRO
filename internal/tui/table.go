package tui

import (
	"fmt"
	"strings"

	"cerebro/internal/model"

	"github.com/charmbracelet/lipgloss"
)

const (
	badgeW    = 11 // widest badge incl. chip padding: [TORRENT]
	sizeColW  = 10
	rightColW = 9
	srcColW   = 10
)

// renderResults renders results as a fixed-width, aligned table windowed
// around the cursor. width is the panel inner width in terminal cells.
func renderResults(results []model.SearchResult, cursor, width, maxRows int) string {
	if len(results) == 0 {
		return emptyStyle.Render("  No results yet — type a query and press Enter.")
	}
	if maxRows < 1 {
		maxRows = 1
	}
	if cursor >= len(results) {
		cursor = len(results) - 1
	}
	if cursor < 0 {
		cursor = 0
	}

	start := cursor - maxRows/2
	if start < 0 {
		start = 0
	}
	if start+maxRows > len(results) {
		start = len(results) - maxRows
	}
	if start < 0 {
		// Few results in a tall window: len-maxRows went negative.
		start = 0
	}
	end := start + maxRows
	if end > len(results) {
		end = len(results)
	}

	// The SOURCE column is dropped when the panel is narrow (split-pane
	// layout) so the title always has room and rows never overflow.
	hasSrc := width >= 70
	titleW := width - 49
	if !hasSrc {
		titleW = width - 38
	}
	if titleW < 12 {
		titleW = 12
	}

	var b strings.Builder

	// Column header + separator (indented 2 to align under the cursor arrow).
	hdr := col("CATEGORY", badgeW) + "  " +
		col("TITLE", titleW) + "  " +
		colR("SIZE", sizeColW) + " " +
		colR("INFO", rightColW)
	if hasSrc {
		hdr += " " + colR("SOURCE", srcColW)
	}
	// Opaque pill header keeps the column names legible on any background.
	b.WriteString("  " + headerStyle.Render(strings.TrimRight(hdr, " ")) + "\n")
	sepW := badgeW + 2 + titleW + 2 + sizeColW + 1 + rightColW
	if hasSrc {
		sepW += 1 + srcColW
	}
	b.WriteString("  " + dimStyle.Render(strings.Repeat("─", sepW)) + "\n")

	for i := start; i < end; i++ {
		r := results[i]
		badge := lipgloss.NewStyle().Width(badgeW).MaxWidth(badgeW).Render(BadgeStyle(r.Category).Render("[" + BadgeLabel(r.Category) + "]"))
		title := col(model.Truncate(r.Title, titleW), titleW)
		size := colR(r.Size, sizeColW)

		var right string
		switch {
		case r.Owned:
			// [OWNED] overrides the info column: you already have this one.
			right = colR(okStyle.Render("OWNED"), rightColW)
		case r.Category == model.CatTorrent:
			right = colR(fmt.Sprintf("%d▲", r.Seeders), rightColW)
		case r.Category == model.CatYouTube:
			right = colR(model.Truncate(r.Author, rightColW-1), rightColW)
		default:
			right = colR(r.Ext, rightColW)
		}

		line := strings.TrimRight(badge+"  "+title+"  "+size+" "+right, " ")
		if hasSrc {
			line = badge + "  " + title + "  " + size + " " + right + " " + colR(model.Truncate(strings.ToLower(r.Source), srcColW), srcColW)
		}
		if i == cursor {
			b.WriteString(cursorStyle.Render("▸ ") + rowSelected.Render(line) + "\n")
		} else {
			b.WriteString("  " + rowStyle.Render(line) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// resultsPanelWidth returns the capped panel width for a terminal width.
func resultsPanelWidth(termW int) int {
	panelW := min(termW-6, 112)
	if panelW < 40 {
		panelW = 40
	}
	return panelW
}

// resultsPanel wraps the table in a centered rounded box.
func resultsPanel(inner string, termW int) string {
	return centerBlock(resultsPanelAt(inner, resultsPanelWidth(termW)), termW)
}

// resultsPanelAt renders the rounded box at an explicit width (used by the
// split-pane layout, where the left metadata pane shares the panel's budget).
func resultsPanelAt(inner string, panelW int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(subdued)).
		Padding(0, 1).
		Width(panelW).
		Render(inner)
}

// col left-aligns s in a fixed-width column.
func col(s string, w int) string {
	return lipgloss.NewStyle().Width(w).Align(lipgloss.Left).Render(s)
}

// colR right-aligns s in a fixed-width column.
func colR(s string, w int) string {
	return lipgloss.NewStyle().Width(w).Align(lipgloss.Right).Render(s)
}

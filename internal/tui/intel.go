package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cerebro/internal/intel"
	"cerebro/internal/model"

	"github.com/charmbracelet/lipgloss"
)

// renderIntel renders the split-pane Cerebro Intel dashboard: a left summary
// card (target, kind, bio, reference links) beside a live findings feed
// ([PLATFORM] [STATUS] [URL/DETAIL]), with a status badge per row. status is
// the transient action feedback (copy / open / export results) so every
// button visibly responds.
func renderIntel(rep *intelReport, width, height int, status string) string {
	var b strings.Builder
	head := headingStyle.Render(fmt.Sprintf("  CEREBRO INTEL    ·    %s", rep.target)) + "  " + accentStyle.Render("["+rep.kind+"]")
	b.WriteString(centerLine(fit(head, width), width))
	if status != "" {
		b.WriteString("\n" + centerLine(fit(errStyle.Render("  "+status), width), width))
	}
	b.WriteString("\n\n")

	// Fixed-width left summary pane (32 cols); the right findings table takes
	// the rest of the terminal, clamped so the joined row never exceeds the
	// viewport (no horizontal overflow on narrow terminals).
	summaryW := 32
	if width < 70 {
		summaryW = 24
	}
	feedW := width - summaryW - 6
	if feedW < 28 {
		feedW = 28
	}
	if feedW > width-summaryW-6 {
		feedW = width - summaryW - 6
	}
	if feedW < 20 {
		feedW = 20
	}
	rows := height - 6
	if status != "" {
		rows-- // the feedback line takes one row
	}
	if rows < 3 {
		rows = 3
	}

	left := intelSummaryCard(rep, summaryW, rows)
	right := intelFindingsFeed(rep, feedW, rows)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	// centerBlock (never fit): fit/Truncate collapses newlines and would mash
	// the two panes into one truncated line.
	return b.String() + "\n" + centerBlock(joined, width)
}

// intelReport is the live intel state kept in the App.
type intelReport struct {
	target   string
	kind     string
	bio      string
	links    []string
	findings []intel.Finding
	done     bool
	cursor   int
}

// intelSummaryCard renders the left pane: target identity, kind, bio and the
// reference links from Wikipedia/DuckDuckGo.
func intelSummaryCard(rep *intelReport, w, rows int) string {
	var content []string
	content = append(content, accentStyle.Render(" TARGET"))
	content = append(content, lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Bold(true).Width(w-4).Render(rep.target))
	content = append(content, "")
	content = append(content, accentStyle.Render(" TYPE"))
	content = append(content, dimStyle.Render(" "+rep.kind))
	content = append(content, "")
	if rep.bio != "" {
		content = append(content, accentStyle.Render(" SUMMARY"))
		for _, l := range wrapLines(rep.bio, w-4, 6) {
			content = append(content, col(l, w-4))
		}
		content = append(content, "")
	}
	if len(rep.links) > 0 {
		content = append(content, accentStyle.Render(" REFERENCES"))
		for _, l := range rep.links {
			content = append(content, truncW(accentStyle.Render(" ↪ "+truncateURL(l, w-10)), w-4))
		}
		content = append(content, "")
	}
	if rep.done {
		content = append(content, okStyle.Render(fmt.Sprintf(" %d sources checked", len(rep.findings))))
	} else {
		content = append(content, accentStyle.Render(" ⏳ probing…"))
	}
	content = append(content, "")
	content = append(content, dimStyle.Render(" enter/p open · 1-9 jump · y copy"))
	content = append(content, dimStyle.Render(" e export · esc back · q quit"))

	card := lipgloss.JoinVertical(lipgloss.Left, content...)
	lines := strings.Split(card, "\n")
	if n := rows - 2; len(lines) > n {
		lines = lines[:n]
	}
	// Pad to the same height as the findings feed so the two panes end flush.
	for len(lines) < rows-2 {
		lines = append(lines, "")
	}
	// Width(32) defines the total box width; MaxWidth(32) would slice off the
	// right border + corners, so it is intentionally omitted.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(accentDim)).
		Padding(0, 1).
		Width(w).
		Height(len(lines)).
		Render(strings.Join(lines, "\n"))
}

// intelFindingsFeed renders the right pane: the live [PLATFORM][STATUS] table.
// w is the TOTAL box width (borders + padding included); content columns are
// derived from it so the box always closes flush at the viewport edge.
func intelFindingsFeed(rep *intelReport, w, rows int) string {
	inner := w - 4 // borders 2 + horizontal padding 2
	if inner < 16 {
		inner = 16
	}
	// Column layout: # (3) | PLATFORM (18) | STATUS (12) | DETAIL (rest).
	// The 2-cell cursor prefix + 3-cell row number leave room for the detail
	// column, so every row fits the box exactly and borders close flush.
	const numW = 3
	platW, statusW := 18, 12
	detailW := inner - 2 - numW - platW - statusW
	if detailW < 12 {
		detailW = 12
	}
	var content []string
	header := lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Bold(true).
		Render(col("#", numW) + col("PLATFORM", platW) + col("STATUS", statusW) + col("DETAIL", detailW))
	content = append(content, strings.Repeat(" ", 2)+header)
	content = append(content, col(strings.Repeat("─", inner), inner))

	if len(rep.findings) == 0 && !rep.done {
		content = append(content, dimStyle.Render(" probing…"))
	}

	// Visible window around the cursor.
	start := 0
	if len(rep.findings) > rows-3 {
		start = rep.cursor - (rows-3)/2
		if start < 0 {
			start = 0
		}
		if start+rows-3 > len(rep.findings) {
			start = len(rep.findings) - (rows - 3)
		}
	}
	end := start + rows - 3
	if end > len(rep.findings) {
		end = len(rep.findings)
	}

	for i := start; i < end; i++ {
		f := rep.findings[i]
		prefix := "  "
		if i == rep.cursor {
			prefix = cursorStyle.Render("▸ ")
		}
		num := col(fmt.Sprintf("%d.", i+1), numW)
		platform := truncW(f.Platform, platW)
		status := intelStatusBadge(f.Status)
		var detail string
		switch {
		case f.URL != "":
			detail = truncateURL(f.URL, detailW)
		case f.Detail != "":
			detail = f.Detail
		}
		line := num + col(platform, platW) + col(status, statusW) + truncW(detail, detailW)
		content = append(content, prefix+line)
	}

	feed := lipgloss.JoinVertical(lipgloss.Left, content...)
	lines := strings.Split(feed, "\n")
	if n := rows - 2; len(lines) > n {
		lines = lines[:n]
	}
	// Pad the feed to the same height as the summary card so both panes end
	// flush (a short findings list otherwise leaves the left card dangling).
	for len(lines) < rows-2 {
		lines = append(lines, "")
	}
	// Width(w) is the total box width; no MaxWidth — that would slice the
	// right border + corners off (the layout bleed bug).
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(subdued)).
		Padding(0, 1).
		Width(w).
		Height(len(lines)).
		Render(strings.Join(lines, "\n"))
}

// truncateURL shortens a long URL to w cells, keeping the scheme + host and
// replacing the tail with "…" so table borders never get pushed off-screen.
func truncateURL(u string, w int) string {
	if w <= 3 {
		return "…"
	}
	if lipgloss.Width(u) <= w {
		return u
	}
	schemeEnd := strings.Index(u, "://")
	head := u
	if schemeEnd >= 0 {
		head = u[:schemeEnd+3]
	}
	// Keep the host up to the first path segment.
	rest := u[len(head):]
	if slash := strings.Index(rest, "/"); slash >= 0 {
		head += rest[:slash]
		rest = rest[slash:]
	}
	// Show head + "…" + the tail, fitting exactly w cells.
	tail := rest
	if len(tail) > 24 {
		tail = tail[len(tail)-24:]
	}
	hw := lipgloss.Width(head)
	tw := lipgloss.Width(tail)
	if hw+1+tw <= w {
		return head + "…" + tail
	}
	// Budget: head gets half, tail gets the rest (minus the ellipsis).
	// model.Truncate appends its own "…", so drop it and add a single
	// separator to avoid a double ellipsis.
	budget := w - 1
	half := budget / 2
	hf := strings.TrimSuffix(model.Truncate(head, half), "…")
	room := budget - lipgloss.Width(hf) - 1
	tf := strings.TrimSuffix(model.Truncate(tail, room), "…")
	return hf + "…" + tf
}

// intelStatusBadge colors a finding status: FOUND green, UNVERIFIED amber,
// NOT FOUND dim red.
func intelStatusBadge(st intel.Status) string {
	switch st {
	case intel.StatusFound:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(emerald)).Bold(true).Render(string(st))
	case intel.StatusUnverified:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(orange)).Bold(true).Render(string(st))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(subdued)).Render(string(st))
	}
}

// exportIntelReport writes the recon report as Markdown to
// ~/.cerebro/reports/<target>.md and returns the path.
func exportIntelReport(rep *intelReport) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	dir := filepath.Join(home, ".cerebro", "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, model.SanitizeFilename(rep.target)+".md")
	var b strings.Builder
	b.WriteString("# CEREBRO INTEL REPORT\n\n")
	b.WriteString(fmt.Sprintf("**Target:** %s\n", rep.target))
	b.WriteString(fmt.Sprintf("**Type:** %s\n", rep.kind))
	b.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	if rep.bio != "" {
		b.WriteString("## Summary\n\n" + rep.bio + "\n\n")
	}
	if len(rep.links) > 0 {
		b.WriteString("## References\n\n")
		for _, l := range rep.links {
			b.WriteString("- " + l + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Findings\n\n| Platform | Status | URL |\n|---|---|---|\n")
	for _, f := range rep.findings {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", f.Platform, f.Status, f.URL))
	}
	return path, os.WriteFile(path, []byte(b.String()), 0o644)
}

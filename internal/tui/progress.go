package tui

import (
	"fmt"
	"strings"

	"cerebro/internal/model"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// renderDownloads renders the live downloads dashboard.
func renderDownloads(jobs []*model.DownloadJob, p progress.Model, aggSpeed float64, cursor, width, height int) string {
	active := 0
	for _, j := range jobs {
		if j.IsActive() {
			active++
		}
	}
	var b strings.Builder
	b.WriteString(headingStyle.Render(fmt.Sprintf("  DOWNLOADS    %d active    ·    %s", active, model.FormatSpeed(aggSpeed))))
	b.WriteString("\n\n")
	if len(jobs) == 0 {
		b.WriteString(emptyStyle.Render("  Nothing downloading — press Esc, pick a result, Enter to start."))
		return b.String()
	}

	// Each active job row renders as three lines (title, progress bar, stats)
	// while finished jobs take two; the heading and the panel borders consume
	// the rest of the caller-provided height budget, so the dashboard never
	// overflows the terminal.
	rows := (height - 4) / 3
	if rows < 1 {
		rows = 1
	}
	if cursor >= len(jobs) {
		cursor = len(jobs) - 1
	}
	start := cursor - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > len(jobs) {
		start = len(jobs) - rows
	}
	if start < 0 {
		// Few jobs in a tall window: len-rows went negative.
		start = 0
	}
	end := start + rows
	if end > len(jobs) {
		end = len(jobs)
	}

	for i := start; i < end; i++ {
		j := jobs[i]
		prefix := "  "
		if i == cursor {
			prefix = cursorStyle.Render("▸ ")
		}
		b.WriteString(prefix + renderJobRow(j, p))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderJobRow renders one download job: badge, title, status, progress bar.
func renderJobRow(j *model.DownloadJob, p progress.Model) string {
	badge := BadgeStyle(j.Result.Category).Render("[" + BadgeLabel(j.Result.Category) + "]")
	title := model.Truncate(j.Result.Title, 58)
	status := statusBadge(j.Status).Render(strings.ToUpper(j.Status))

	var body string
	switch j.Status {
	case model.StatusStreaming:
		body = accentStyle.Render("▶ streaming to mpv…") + "  " + dimStyle.Render(model.Truncate(j.Result.Magnet, 42))
	case model.StatusCompleted:
		if j.Result.Source == "history" {
			body = dimStyle.Render("⇩ past download") + "  " + dimStyle.Render("→ "+j.OutputPath)
		} else {
			body = okStyle.Render("✓ done") + "  " + dimStyle.Render("→ "+j.OutputPath)
		}
	case model.StatusFailed:
		msg := "error"
		if j.Err != nil {
			msg = j.Err.Error()
		}
		body = errStyle.Render("✗ " + model.Truncate(msg, 72))
	default:
		var bar, stats string
		if j.BytesTotal > 0 {
			bar = p.ViewAs(j.Progress)
			// Percent first so even tiny increments are visibly reflected — a
			// 46-cell bar shows nothing until ~2%, which reads as "stuck".
			stats = dimStyle.Render(fmt.Sprintf("%.1f%%   ·   %s / %s   ·   %s",
				j.Progress*100, model.FormatBytes(j.BytesDone), model.FormatBytes(j.BytesTotal), model.FormatSpeed(j.Speed)))
			if j.Result.Category == model.CatTorrent {
				stats = dimStyle.Render(fmt.Sprintf("%.1f%%   ·   %s / %s   ·   %s   ·   %d peers",
					j.Progress*100, model.FormatBytes(j.BytesDone), model.FormatBytes(j.BytesTotal), model.FormatSpeed(j.Speed), j.Peers))
			}
		} else {
			// Total size unknown: show an indeterminate state instead of a
			// static 0% bar (YouTube before the stream size is known).
			bar = dimStyle.Render("⟳ " + model.FormatBytes(j.BytesDone) + " downloaded…")
			stats = dimStyle.Render(model.FormatSpeed(j.Speed))
		}
		body = bar + "\n" + stats
	}
	return badge + "  " + title + "  " + status + "\n" + body
}

// statusBadge colors a job status label.
func statusBadge(status string) lipgloss.Style {
	switch status {
	case model.StatusDownloading:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true)
	case model.StatusResolving, model.StatusQueued:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(subdued)).Bold(true)
	case model.StatusMerging, model.StatusStreaming:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(purple)).Bold(true)
	case model.StatusSeeding, model.StatusCompleted:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(emerald)).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(danger)).Bold(true)
	}
}

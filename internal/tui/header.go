package tui

import (
	"fmt"
	"strings"

	"cerebro/internal/model"

	"github.com/charmbracelet/lipgloss"
)

// RenderHeader renders the CEREBRO title block plus the live network status
// bar, centered. When compact is true (a search is active or downloads are
// shown) the big banner collapses to a single centered title line so results
// get the screen space.
func RenderHeader(s model.StatusInfo, dir string, width, height int, compact bool) string {
	var block string
	if compact {
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true).Render("CEREBRO MAX") +
			lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Bold(true).Render("  ·  FIND EVERYTHING · DOWNLOAD ANYTHING")
		block = centerLine(fit(title, width), width)
	} else {
		var b strings.Builder
		if height < 22 {
			b.WriteString(centerLine(lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true).Render(fit("CEREBRO MAX  —  FIND EVERYTHING · DOWNLOAD ANYTHING", width)), width))
		} else {
			// Pad every banner line to the widest one so the ASCII block stays
			// perfectly aligned while being centered as a single unit.
			colored := make([]string, len(bannerLines))
			maxW := 0
			for i, line := range bannerLines {
				colored[i] = lipgloss.NewStyle().Foreground(bannerColors[i%len(bannerColors)]).Render(strings.TrimRight(line, " "))
				if w := lipgloss.Width(colored[i]); w > maxW {
					maxW = w
				}
			}
			for i := range colored {
				colored[i] = lipgloss.NewStyle().Width(maxW).Render(colored[i])
			}
			b.WriteString(centerBlock(lipgloss.JoinVertical(lipgloss.Left, colored...), width) + "\n")
			// MAX sub-banner: the same blocky pixel font, colored in a red →
			// orange → yellow fire gradient. Hidden on short terminals to
			// protect the height budget (each block row adds 6 lines).
			if height >= 28 {
				fire := make([]string, len(maxLines))
				maxW = 0
				for i, line := range maxLines {
					fire[i] = lipgloss.NewStyle().Foreground(fireColors[i%len(fireColors)]).Render(strings.TrimRight(line, " "))
					if w := lipgloss.Width(fire[i]); w > maxW {
						maxW = w
					}
				}
				for i := range fire {
					fire[i] = lipgloss.NewStyle().Width(maxW).Render(fire[i])
				}
				b.WriteString(centerBlock(lipgloss.JoinVertical(lipgloss.Left, fire...), width) + "\n")
			}
			b.WriteString(centerLine(lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Bold(true).Render(fit("FIND EVERYTHING  ·  DOWNLOAD ANYTHING", width)), width) + "\n")
			b.WriteString(centerLine(lipgloss.NewStyle().Foreground(lipgloss.Color(orange)).Render(fit("books · games · software · video · audio · archives — one search, every source", width)), width) + "\n")
			b.WriteString(centerLine(dimStyle.Render(fit("built by Naman Gaonkar · contact the developer: naman-gaonkar.vercel.app", width)), width))
		}
		block = b.String()
	}

	state := "idle"
	stateStyle := dimStyle
	if s.Searching {
		state = "searching…"
		stateStyle = accentStyle
	}
	var parts []string
	parts = append(parts, keyStyle.Render("jobs")+" "+lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render(fmt.Sprint(s.ActiveJobs)))
	parts = append(parts, keyStyle.Render("speed")+" "+lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render(model.FormatSpeed(s.Speed)))
	if s.DiskTotal > 0 {
		pct := 100 * float64(s.DiskFree) / float64(s.DiskTotal)
		parts = append(parts, keyStyle.Render("disk")+" "+lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render(fmt.Sprintf("%s free (%.0f%%)", model.FormatBytes(int64(s.DiskFree)), pct)))
	}
	parts = append(parts, keyStyle.Render("dir")+" "+lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render(dir))
	statusBar := stateStyle.Render("● "+state) +
		dimStyle.Render("   ·   ") +
		strings.Join(parts, dimStyle.Render("   ·   "))

	return strings.Join([]string{
		block,
		centerLine(fit(statusBar, width), width),
	}, "\n")
}

// fit truncates a rendered string to fit the terminal width.
func fit(s string, width int) string {
	if width <= 0 {
		return s
	}
	return model.Truncate(strings.TrimRight(s, " "), width)
}

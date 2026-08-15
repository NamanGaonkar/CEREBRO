package tui

import (
	"strings"

	"cerebro/internal/model"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// renderTabs renders the category filter tabs as opaque pills, highlighting
// the active one. Labels and pill padding adapt to the terminal width so the
// bar never overflows: spaced-out labels on wide terminals, compact labels
// with pills on medium ones, bare labels on narrow ones.
func renderTabs(filter model.CategoryFilter, active bool, width int) string {
	filters := []model.CategoryFilter{
		model.FilterAll, model.FilterBooks, model.FilterGames,
		model.FilterSoftware, model.FilterVideo, model.FilterAudio, model.FilterArchives,
	}
	// Measure both label forms, then add the pill padding (2 cells per tab)
	// and the inter-tab gaps to pick the widest form that still fits.
	fancy, compact := 0, 0
	for _, f := range filters {
		fancy += lipgloss.Width("[ " + f.String() + " ]")
		compact += lipgloss.Width("[" + f.String() + "]")
	}
	const pillPad = 2 * 7 // tabActive/tabInactive each add 1 cell left + right
	spaced, gap, padded := false, " ", true
	switch {
	case fancy+pillPad+(len(filters)-1)*2 <= width:
		spaced, gap, padded = true, "  ", true
	case compact+pillPad+(len(filters)-1) <= width:
		spaced, gap, padded = false, " ", true
	default:
		spaced, gap, padded = false, " ", false
	}

	var b strings.Builder
	for i, f := range filters {
		label := "[" + f.String() + "]"
		if spaced {
			label = "[ " + f.String() + " ]"
		}
		var style lipgloss.Style
		switch {
		case f == filter && active:
			style = tabActive
		case f == filter:
			style = tabCursor
		default:
			style = tabInactive
		}
		if !padded {
			style = style.UnsetPadding()
		}
		b.WriteString(style.Render(label))
		if i < len(filters)-1 {
			b.WriteString(gap)
		}
	}
	return b.String()
}

// renderSearchBar renders the fuzzy omnibar: a clean single-line search input
// with a magnifier prompt and an inline searching spinner. It accepts plain
// queries, magnet links and URLs (handled in resolveInput). It is
// deliberately single-line so it always centers perfectly at any terminal width.
func renderSearchBar(input *textinput.Model, searching bool, spinnerView string) string {
	// Guard against a zero width before the first WindowSizeMsg (non-tty boot)
	// so the input never wraps vertically.
	if input.Width < 20 {
		input.Width = 48
	}
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(purple)).Bold(true).Render("omnibar"))
	sb.WriteString("  ")
	sb.WriteString(input.View())
	if searching {
		sb.WriteString("  ")
		sb.WriteString(accentStyle.Render(spinnerView))
		sb.WriteString(" ")
		sb.WriteString(dimStyle.Render("querying engines…"))
	}
	return sb.String()
}

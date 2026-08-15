package tui

import (
	"strings"

	"cerebro/internal/model"

	"github.com/charmbracelet/lipgloss"
)

// Linear-style dark palette with neon accents — brightened for legibility on
// transparent, vibrant or busy terminal backgrounds.
const (
	accent    = "#00F0FF" // electric cyan (status, prompts)
	purple    = "#BD93F9" // bright purple (headings)
	magenta   = "#FF79C6" // torrents
	crimson   = "#FF5555" // youtube
	amber     = "#F1FA8C" // pdf / docs
	emerald   = "#50FA7B" // direct / ok / status
	orange    = "#FFB86C" // audio / helpers
	sky       = "#8BE9FD" // mp4 / video
	gold      = "#FFD866" // games
	softBlue  = "#82AAFF" // software
	bg        = "#16161E"
	panel     = "#222838"
	subdued   = "#8A93B8" // borders & separators (brighter)
	text      = "#FFFFFF" // crisp bold white
	danger    = "#FF6B6B"
	accentDim = "#0F4C5C"
)

// bannerColors: neon cyan → green gradient across the CEREBRO banner lines.
var bannerColors = []lipgloss.Color{
	lipgloss.Color("#00FFFF"),
	lipgloss.Color("#00F5D6"),
	lipgloss.Color("#00F5AE"),
	lipgloss.Color("#30F59A"),
	lipgloss.Color("#50FA7B"),
	lipgloss.Color("#7DFA9B"),
}

// fireColors: red → orange → yellow fire gradient for the MAX sub-banner.
var fireColors = []lipgloss.Color{
	lipgloss.Color("#FF0055"),
	lipgloss.Color("#FF5500"),
	lipgloss.Color("#FFAA00"),
	lipgloss.Color("#FFFF00"),
	lipgloss.Color("#FFAA00"),
	lipgloss.Color("#FF5500"),
}

// bannerLines is the CEREBRO ASCII banner (6 lines, colored per line).
var bannerLines = []string{
	" ██████╗███████╗██████╗ ███████╗██████╗ ██████╗  ██████╗ ",
	"██╔════╝██╔════╝██╔══██╗██╔════╝██╔══██╗██╔══██╗██╔═══██╗",
	"██║     █████╗  ██████╔╝█████╗  ██████╔╝██████╔╝██║   ██║",
	"██║     ██╔══╝  ██╔══██╗██╔══╝  ██╔══██╗██╔══██╗██║   ██║",
	"╚██████╗███████╗██║  ██║███████╗██████╔╝██║  ██║╚██████╔╝",
	" ╚═════╝╚══════╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ",
}

// maxLines is the MAX sub-banner in the same blocky pixel font as CEREBRO,
// scaled to roughly half the banner's width (33 chars vs 69).
var maxLines = []string{
	"███╗   ███╗   █████╗   ██╗  ██╗",
	"████╗ ████║  ██╔══██╗  ╚██╗██╔╝",
	"██╔████╔██║  ███████║   ╚███╔╝ ",
	"██║╚██╔╝██║  ██╔══██║   ██╔██╗ ",
	"██║ ╚═╝ ██║  ██║  ██║  ██╔╝ ██╗",
	"╚═╝     ╚═╝  ╚═╝  ╚═╝  ╚═╝  ╚═╝",
}

var (
	// Category badges.
	badgeTorrent = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(magenta)).Bold(true).Padding(0, 1)
	badgeYouTube = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(crimson)).Bold(true).Padding(0, 1)
	badgePDF     = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(amber)).Bold(true).Padding(0, 1)
	badgeAudio   = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(orange)).Bold(true).Padding(0, 1)
	badgeMP4     = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(sky)).Bold(true).Padding(0, 1)
	badgeGame    = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(gold)).Bold(true).Padding(0, 1)
	badgeDirect  = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(emerald)).Bold(true).Padding(0, 1)
	badgeSoft    = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(softBlue)).Bold(true).Padding(0, 1)
	badgeOther   = lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Background(lipgloss.Color(panel)).Bold(true).Padding(0, 1)

	// Filter tabs — opaque pills so labels stay legible on any wallpaper.
	tabActive   = lipgloss.NewStyle().Foreground(lipgloss.Color(bg)).Background(lipgloss.Color(accent)).Bold(true).Padding(0, 1)
	tabCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color(accentDim)).Bold(true).Padding(0, 1)
	tabInactive = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#1E1E2E")).Padding(0, 1)

	// Rows.
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	rowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(text))
	rowSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#FF007F")).Bold(true)

	// Text helpers — bright, high-contrast.
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	keyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(danger)).Bold(true)
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(emerald)).Bold(true)
	// searchModeStyle flags when the omnibar owns the keyboard — bright cyan on
	// an opaque pill so it is impossible to miss on any terminal background.
	searchModeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00F0FF")).Bold(true).Background(lipgloss.Color("#1E1E2E"))
	accentStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true)
	updateStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(orange)).Bold(true)
	emptyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(orange))
	headingStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(purple)).Bold(true)
	headerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2")).Background(lipgloss.Color("#26263A")).Bold(true)
	panelStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(subdued)).Padding(1, 2).Background(lipgloss.Color(panel))
)

// BadgeStyle returns the badge style for a result category.
func BadgeStyle(cat string) lipgloss.Style {
	switch cat {
	case model.CatTorrent:
		return badgeTorrent
	case model.CatGame:
		return badgeGame
	case model.CatYouTube:
		return badgeYouTube
	case model.CatPDF:
		return badgePDF
	case model.CatAudio:
		return badgeAudio
	case model.CatMP4:
		return badgeMP4
	case model.CatDirect:
		return badgeDirect
	case model.CatSoftware:
		return badgeSoft
	}
	return badgeOther
}

// BadgeLabel returns the short label shown inside a category badge.
func BadgeLabel(cat string) string {
	switch cat {
	case model.CatTorrent:
		return "TORRENT"
	case model.CatGame:
		return "GAME"
	case model.CatYouTube:
		return "YOUTUBE"
	case model.CatPDF:
		return "DOC"
	case model.CatAudio:
		return "MUSIC"
	case model.CatMP4:
		return "MP4"
	case model.CatDirect:
		return "DIRECT"
	case model.CatSoftware:
		return "SOFTWARE"
	}
	return strings.ToUpper(cat)
}

// centerLine centers a single rendered line within width.
func centerLine(s string, width int) string {
	if s == "" || width <= 0 {
		return s
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", (width-w)/2) + s
}

// centerBlock centers every line of a multi-line block within width.
func centerBlock(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = centerLine(l, width)
	}
	return strings.Join(lines, "\n")
}

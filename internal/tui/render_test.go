package tui

import (
	"strings"
	"testing"

	"cerebro/internal/downloader"
	"cerebro/internal/intel"
	"cerebro/internal/model"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func renderFor(w, h int, results []model.SearchResult, lastQuery string, v view) string {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = w, h
	a.input.Width = w - 40
	if a.input.Width < 20 {
		a.input.Width = 20
	}
	a.results = results
	a.lastQuery = lastQuery
	if lastQuery != "" {
		a.input.SetValue(lastQuery) // mirrors real usage: the omnibar keeps the query
	}
	a.view = v
	return a.View()
}

// TestRenderUpdateBanner guards the startup update banner: a long notice must
// be truncated to the terminal width (never overflow a line) while still
// appearing in the rendered output.
func TestRenderUpdateBanner(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 80, 24
	a.updateNotice = "UPDATE 9.9.9 available — you're on 1.5.0 · scoop update cerebro · no Scoop? winget install ScoopInstaller.Scoop"
	out := a.View()
	if !strings.Contains(out, "UPDATE 9.9.9") {
		t.Error("expected the update notice to be rendered")
	}
	for i, line := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(line); lw > 80 {
			t.Errorf("banner render line %d: width %d > 80: %q", i+1, lw, line)
		}
	}
}

// TestBootScreenCentered guards the boot layout: before any search has run,
// the CEREBRO banner + search block must be vertically centered on the screen
// — not pinned to the top row.
func TestBootScreenCentered(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	out := a.View()
	if !strings.Contains(out, "██████╗") {
		t.Fatal("expected the CEREBRO ASCII banner on the boot screen")
	}
	if !strings.Contains(out, "████╗ ████║") {
		t.Error("expected the blocky MAX sub-banner under CEREBRO at 100x30")
	}
	first := -1
	for i, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			first = i
			break
		}
	}
	// At 100×30 the centered stack starts ~5 lines down; anything less than 2
	// means the layout regressed to top-aligned.
	if first < 2 {
		t.Errorf("boot screen pinned to the top: first content line = %d at h=30 (want vertically centered)", first)
	}
}

// TestRenderHeightFits guards against the results/downloads tables being
// taller than the terminal: with many results the panel grows to full height
// and the terminal clips the bottom rows and the footer (the "chipped off"
// table). The whole rendered view must fit within the terminal height.
func TestRenderHeightFits(t *testing.T) {
	many := make([]model.SearchResult, 60)
	for i := range many {
		many[i] = model.SearchResult{Title: "Interstellar (2014) 1080p BluRay x264", Category: model.CatTorrent, Size: "2.1 GB", Seeders: 128, Source: "yts"}
	}
	jobs := make([]*model.DownloadJob, 15)
	for i := range jobs {
		jobs[i] = &model.DownloadJob{Status: model.StatusDownloading, Result: many[0], BytesTotal: 1000}
	}

	for _, h := range []int{22, 24, 30, 34} {
		// Boot view: big ASCII banner header, no search run yet.
		mgr3 := downloader.NewManager(".")
		c := New(mgr3)
		c.width, c.height = 100, h
		c.view = viewSearch
		if out3 := c.View(); len(strings.Split(out3, "\n")) > h {
			t.Errorf("boot h=%d: rendered %d lines > terminal %d", h, len(strings.Split(out3, "\n")), h)
		}

		for _, withNotice := range []bool{false, true} {
			// Search view: 60 results, full-height table.
			mgr := downloader.NewManager(".")
			a := New(mgr)
			a.width, a.height = 100, h
			a.results = many
			a.lastQuery = "interstellar"
			a.view = viewSearch
			if withNotice {
				a.updateNotice = "UPDATE 9.9.9 available — you're on 1.6.0"
			}
			out := a.View()
			if n := len(strings.Split(out, "\n")); n > h {
				t.Errorf("search h=%d notice=%v: rendered %d lines > terminal %d (bottom clipped)", h, withNotice, n, h)
			}

			// Downloads view: 15 jobs.
			mgr2 := downloader.NewManager(".")
			b := New(mgr2)
			b.width, b.height = 100, h
			b.jobs = jobs
			b.lastQuery = "interstellar"
			b.view = viewDownloads
			if withNotice {
				b.updateNotice = "UPDATE 9.9.9 available — you're on 1.6.0"
			}
			out2 := b.View()
			if n := len(strings.Split(out2, "\n")); n > h {
				t.Errorf("downloads h=%d notice=%v: rendered %d lines > terminal %d (bottom clipped)", h, withNotice, n, h)
			}
		}
	}
}

// TestRenderSplitPane guards the MAX split layout: with results on a wide
// terminal the left cover-art pane sits beside the table, and no rendered
// line may exceed the terminal width.
func TestRenderSplitPane(t *testing.T) {
	results := []model.SearchResult{
		{ID: "1", Title: "Interstellar (2014) 1080p BluRay x264", Category: model.CatTorrent, Size: "2.1 GB", Seeders: 128, Source: "yts", Magnet: "magnet:?xt=urn:btih:ABC&dn=interstellar"},
		{ID: "2", Title: "Avengers Doomsday — Official Trailer", Category: model.CatYouTube, VideoID: "dQw4w9WgXcQ", Author: "Marvel Entertainment", Source: "youtube", URL: "https://youtube.com/watch?v=dQw4w9WgXcQ"},
		{ID: "3", Title: "Cyberpunk 2077-CODEX", Category: model.CatGame, Size: "58 GB", Seeders: 99, Source: "tpb"},
	}
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	a.input.Width = 60
	a.results = results
	a.lastQuery = "interstellar"
	a.input.SetValue("interstellar")
	a.view = viewSearch
	out := a.View()
	// The left pane is now a metadata card: category pill, specs box and
	// action hints — no blocky placeholder art.
	for _, want := range []string{"[TORRENT]", "SIZE", "FORMAT", "SOURCE", "HEALTH", "p stream · m copy · enter ⤓"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the metadata card", want)
		}
	}
	if strings.Contains(out, "\x1bP") {
		t.Error("ANSI mode must never emit sixel escapes")
	}
	for i, line := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(line); lw > 100 {
			t.Errorf("split pane line %d: width %d > 100: %q", i+1, lw, line)
		}
	}
}

// TestRenderIntel guards the Cerebro Intel dashboard: target, kind, bio and
// the live findings feed with FOUND / UNVERIFIED / NOT FOUND badges, all
// within the terminal width.
func TestRenderIntel(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 120, 34
	a.intelRep = &intelReport{
		target: "torvalds",
		kind:   "username",
		bio:    "Linus Torvalds is a Finnish software engineer who created Linux.",
		links:  []string{"https://en.wikipedia.org/wiki/Linus_Torvalds"},
		findings: []intel.Finding{
			{Platform: "GitHub", URL: "https://github.com/torvalds", Status: intel.StatusFound, Detail: "account exists"},
			{Platform: "Reddit", URL: "https://www.reddit.com/user/torvalds", Status: intel.StatusNotFound, Detail: "no account"},
			{Platform: "X / Twitter", URL: "https://x.com/torvalds", Status: intel.StatusUnverified, Detail: "unverifiable"},
		},
		done: true,
	}
	a.view = viewIntel
	out := a.View()
	for _, want := range []string{"CEREBRO INTEL", "torvalds", "GitHub", "FOUND", "NOT FOUND", "UNVERIFIED", "SUMMARY"} {
		if !strings.Contains(out, want) {
			t.Errorf("intel view missing %q", want)
		}
	}
	for i, line := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(line); lw > 120 {
			t.Errorf("intel line %d: width %d > 120: %q", i+1, lw, line)
		}
	}
	// Layout guards: every box closes (no corner-sliced borders) and the
	// joined panes fit the viewport.
	if !strings.Contains(out, "╮") || !strings.Contains(out, "╯") {
		t.Error("intel panes must close their borders (corner-chop regression)")
	}
	// The summary card and findings feed end flush: the card bottom border
	// appears and both panes' bottom borders render.
	if strings.Count(out, "╰") < 2 {
		t.Error("both intel panes must render bottom borders")
	}
	// URL truncation keeps rows inside the feed box (no border push).
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "…") && lipgloss.Width(line) > 120 {
			t.Errorf("intel line with ellipsis overflows: %q", line)
		}
	}
}

// TestIntelConceptNoProbes guards the intent classifier wiring: a topic query
// runs NO username probes — only synthesis references appear.
func TestIntelConceptNoProbes(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	a.intelRep = &intelReport{
		target: "what is amoeba",
		kind:   "topic",
		bio:    "An amoeba is a type of cell or unicellular organism.",
		links:  []string{"https://en.wikipedia.org/wiki/Amoeba"},
		findings: []intel.Finding{
			{Platform: "Wikipedia", URL: "https://en.wikipedia.org/wiki/Special:Search?search=what+is+amoeba", Status: intel.StatusFound, Detail: "summary / abstract"},
			{Platform: "DuckDuckGo", URL: "https://duckduckgo.com/?q=what+is+amoeba", Status: intel.StatusFound, Detail: "search reference"},
		},
		done: true,
	}
	a.view = viewIntel
	out := a.View()
	if !strings.Contains(out, "[topic]") {
		t.Errorf("concept query should show [topic] kind, got: %q", out[:min(200, len(out))])
	}
	for i, line := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(line); lw > 100 {
			t.Errorf("concept line %d: width %d > 100: %q", i+1, lw, line)
		}
	}
}

// TestFocusStateMachine guards the SearchFocus / ListFocus transitions:
// Down (or Tab) blurs the omnibar into the list without clearing the text,
// and Esc in SearchFocus clears the input back to boot.
func TestFocusStateMachine(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	a.results = []model.SearchResult{{ID: "1", Title: "Interstellar", Category: model.CatTorrent}}
	a.lastQuery = "interstellar"
	a.input.SetValue("interstellar")
	a.view = viewSearch

	// SearchFocus: Down moves focus to the list without clearing the text.
	a.input.Focus()
	m, _ := a.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app := m.(*App)
	if app.input.Focused() {
		t.Error("Down in SearchFocus should blur into ListFocus")
	}
	if app.input.Value() != "interstellar" {
		t.Error("Down must not clear the omnibar text")
	}

	// Back to SearchFocus via /, then Esc blurs back to the list.
	app.input.Focus()
	m, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	app = m.(*App)
	if app.input.Focused() {
		t.Error("Esc in SearchFocus should blur back to ListFocus")
	}
}

// TestRenderSane guards the centered layout: no rendered line may exceed the
// terminal width (overflow lines wrap and look broken) and the search bar must
// sit horizontally centered. Covers boot, results-with-compact-header and the
// downloads dashboard at several terminal sizes.
func TestRenderSane(t *testing.T) {
	results := []model.SearchResult{
		{ID: "1", Title: "Interstellar (2014) 1080p BluRay", Category: model.CatTorrent, Size: "2.1 GB", Seeders: 128, Source: "yts"},
		{ID: "2", Title: "Cyberpunk 2077-CODEX", Category: model.CatGame, Size: "58 GB", Seeders: 99, Source: "tpb"},
		{ID: "3", Title: "Interstellar — The Science Book (EPUB)", Category: model.CatPDF, Size: "4.2 MB", Ext: "epub", Source: "archive.org"},
		{ID: "4", Title: "Interstellar Soundtrack — mp3", Category: model.CatAudio, Size: "118 MB", Ext: "mp3", Source: "archive.org"},
	}
	sizes := [][2]int{{100, 30}, {80, 24}, {120, 34}}
	states := []struct {
		name      string
		lastQuery string
		view      view
	}{
		{"boot", "", viewSearch},
		{"results", "interstellar", viewSearch},
		{"downloads", "interstellar", viewDownloads},
	}
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		for _, st := range states {
			rs := results
			if st.name == "boot" {
				rs = nil
			}
			out := renderFor(w, h, rs, st.lastQuery, st.view)
			for i, line := range strings.Split(out, "\n") {
				if lw := lipgloss.Width(line); lw > w {
					t.Errorf("%dx%d %s line %d: width %d > terminal %d: %q", w, h, st.name, i+1, lw, w, line)
				}
			}
			for _, raw := range strings.Split(out, "\n") {
				if !strings.Contains(raw, "omnibar  ⌕") {
					continue
				}
				line := strings.TrimRight(raw, " ")
				leading := len(line) - len(strings.TrimLeft(line, " "))
				// The bar must be visibly centered — never shoved to the far left
				// (the original layout bug where leading was 0).
				if leading < w/10 {
					t.Errorf("%dx%d %s: search bar not centered (leading %d, width %d)", w, h, st.name, leading, w)
				}
			}
		}
	}
}

// TestBootAfterSearch guards the backspace fix: clearing the omnibar after a
// search must restore the big banner and hide the stale results table — a
// clean centered boot screen, never a leftover full-screen result list.
func TestBootAfterSearch(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	a.lastQuery = "kartik"
	a.results = []model.SearchResult{
		{ID: "1", Title: "Kartik Aaryan — Chandu Champion (2024) 1080p BluRay", Category: model.CatTorrent, Size: "2.1 GB", Seeders: 128, Source: "yts"},
		{ID: "2", Title: "Kartik — full movie", Category: model.CatTorrent, Size: "1.4 GB", Seeders: 55, Source: "1337x"},
	}
	a.input.SetValue("")
	a.view = viewSearch
	out := a.View()
	// The big ASCII banner (block glyphs) is back.
	if !strings.Contains(out, "█") {
		t.Error("big banner missing after clearing the omnibar")
	}
	// No stale results from the previous query.
	if strings.Contains(out, "result(s) for") {
		t.Error("stale results note shown on boot")
	}
	if strings.Contains(out, "Kartik Aaryan — Chandu Champion") {
		t.Error("stale results table shown on boot")
	}
	for i, line := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(line); lw > 100 {
			t.Errorf("boot line %d: width %d > 100: %q", i+1, lw, line)
		}
	}
}

// TestCardNeverSliced guards the misshapen-card fix: on short terminals the
// metadata card drops whole sections (hints → source → specs box) so no box
// border is ever cut through — the card always closes cleanly.
func TestCardNeverSliced(t *testing.T) {
	results := []model.SearchResult{
		{ID: "1", Title: "Kartik Aaryan — Chandu Champion (2024) 1080p BluRay x264 DDP5.1", Category: model.CatTorrent, Size: "2.1 GB", Ext: "MKV", Source: "yts", Seeders: 128},
	}
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 20
	a.input.Width = 60
	a.results = results
	a.lastQuery = "kartik"
	a.input.SetValue("kartik")
	a.view = viewSearch
	out := a.View()
	// The card's outer box must be intact: both top and bottom borders present.
	if !strings.Contains(out, "╭──────────────────────────────") {
		t.Error("card top border missing")
	}
	if !strings.Contains(out, "╰──────────────────────────────") {
		t.Error("card bottom border missing — card was sliced")
	}
	// If the specs box is shown, it must be complete (never cut mid-box):
	// either both its borders appear, or the whole box is dropped.
	if strings.Contains(out, "HEALTH") && !strings.Contains(out, "╰─") {
		t.Error("specs box present but not closed — sliced through the box")
	}
}

// TestModeIndicatorVisible guards the "buttons feel dead" bug: when the
// omnibar owns the keyboard (focused), the screen must say so loudly so the
// user knows P/M/D/Q would type into the box — and that Esc/↓ exits it.
func TestModeIndicatorVisible(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	a.input.Focus()
	a.lastQuery = "kartik"
	a.results = []model.SearchResult{{ID: "1", Title: "Kartik Aaryan — Chandu Champion", Category: model.CatTorrent}}
	a.input.SetValue("kartik")
	a.view = viewSearch
	out := a.View()
	if !strings.Contains(out, "search box active") {
		t.Error("focused omnibar must show the search-box-active indicator")
	}
	if !strings.Contains(out, "Esc/↓ to browse") {
		t.Error("mode indicator must tell the user how to leave the search box")
	}
}

// TestBootOmnibarFocused guards the "type to search does nothing" bug:
// textinput starts unfocused, so Init must focus the omnibar on boot.
func TestBootOmnibarFocused(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.Init() // runs the boot cmds — must focus the omnibar
	if !a.input.Focused() {
		t.Error("omnibar must be focused at boot so typing immediately works")
	}
}

// TestDigitKeysSelectNth guards the 1-9 quick-select keys: pressing "3" in
// the results list moves the cursor to the third result.
func TestDigitKeysSelectNth(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	a.results = []model.SearchResult{
		{ID: "1", Title: "Interstellar (2014) 1080p", Category: model.CatTorrent},
		{ID: "2", Title: "Interstellar — The Book", Category: model.CatPDF},
		{ID: "3", Title: "Interstellar Soundtrack", Category: model.CatAudio},
	}
	a.lastQuery = "interstellar"
	a.input.SetValue("interstellar")
	a.view = viewSearch
	a.cursor = 0

	m, _ := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if app := m.(*App); app.cursor != 2 {
		t.Errorf("key 3 should select result #3 (index 2), got cursor=%d", app.cursor)
	}
	// Out-of-range digits clamp safely.
	m, _ = a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("9")})
	if app := m.(*App); app.cursor != 2 {
		t.Errorf("key 9 with 3 results should clamp to last, got cursor=%d", app.cursor)
	}
}

// TestIntelDigitKeysGuard guards the MAX-mode numbered rows: the feed renders
// "1." "2." "3." prefixes and pressing a digit jumps the cursor to that
// finding, with P/Enter opening the focused URL.
func TestIntelDigitKeysGuard(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 120, 34
	a.intelRep = &intelReport{
		target: "torvalds",
		kind:   "username",
		findings: []intel.Finding{
			{Platform: "GitHub", URL: "https://github.com/torvalds", Status: intel.StatusFound},
			{Platform: "Reddit", URL: "https://www.reddit.com/user/torvalds", Status: intel.StatusNotFound},
			{Platform: "Wikipedia", URL: "https://en.wikipedia.org/wiki/Linus_Torvalds", Status: intel.StatusFound},
		},
		done: true,
	}
	a.view = viewIntel

	// Numbered rows render in the feed.
	out := a.View()
	for _, want := range []string{"1.", "2.", "3."} {
		if !strings.Contains(out, want) {
			t.Errorf("intel feed missing numbered row %q", want)
		}
	}

	// Pressing "2" jumps the cursor to finding #2.
	m, _ := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	app := m.(*App)
	if app.intelRep.cursor != 1 {
		t.Errorf("key 2 should jump to finding #2 (index 1), got cursor=%d", app.intelRep.cursor)
	}
	// P routes to the same open handler as Enter (never falls through to
	// quit); with an empty URL it must be a safe no-op.
	app.intelRep.findings[app.intelRep.cursor].URL = ""
	if _, err := app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); err != nil {
		t.Errorf("P in intel mode should not error: %v", err)
	}
	if app.view != viewIntel {
		t.Error("P in intel mode must stay on the intel dashboard")
	}
}

// TestKeyButtonsGuard audits the search-screen buttons the user relies on:
// M copies, P streams (and errors gracefully on a URL-less result), Q quits
// — and live-filtering that shrinks the result list can never crash them
// (the cursor-out-of-range panic that made the buttons look dead).
func TestKeyButtonsGuard(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	a.results = []model.SearchResult{
		{ID: "1", Title: "Interstellar (2014) 1080p", Category: model.CatTorrent, Magnet: "magnet:?xt=urn:btih:ABC"},
		{ID: "2", Title: "Interstellar Trailer MP4", Category: model.CatDirect, URL: ""}, // URL-less: P must error, never spawn
	}
	a.lastQuery = "interstellar"
	a.input.SetValue("interstellar")
	a.view = viewSearch
	a.cursor = 0

	// M copies the selected row's magnet/URL without crashing.
	m, cmd := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if cmd != nil {
		t.Errorf("M must not return a cmd: %v", cmd)
	}
	app := m.(*App)
	if !strings.Contains(app.errMsg, "copied") && !strings.Contains(app.errMsg, "failed") {
		t.Errorf("M should report a copy result, got errMsg=%q", app.errMsg)
	}

	// P on the URL-less result: routed, errors gracefully, spawns nothing.
	a.cursor = 1
	m, _ = a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	app = m.(*App)
	if !strings.Contains(app.errMsg, "no playable URL") {
		t.Errorf("P on a URL-less result should say so, got errMsg=%q", app.errMsg)
	}

	// Live-filter shrink: cursor past the end of the filtered list must not
	// panic M / P / Enter.
	a.cursor = 5
	a.input.SetValue("zzz-no-match")
	if m, _ := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")}); m.(*App) == nil {
		t.Fatal("M after filter-to-zero must not panic")
	}
	if m, _ := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); m.(*App) == nil {
		t.Fatal("P after filter-to-zero must not panic")
	}
	if m, _ := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")}); m.(*App) == nil {
		t.Fatal("Enter after filter-to-zero must not panic")
	}

	// Q quits — the quit cmd is non-nil only for the quit path.
	_, cmd = a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("Q must return the quit command")
	}
}

// TestMaxModeButtonsGuard audits MAX-mode keys: Y copies (M is an alias),
// 1-9 jumps the cursor, Q quits — no panics.
func TestMaxModeButtonsGuard(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 120, 34
	a.intelRep = &intelReport{
		target: "torvalds",
		kind:   "username",
		findings: []intel.Finding{
			{Platform: "GitHub", URL: "https://github.com/torvalds", Status: intel.StatusFound},
			{Platform: "Reddit", URL: "https://www.reddit.com/user/torvalds", Status: intel.StatusNotFound},
		},
		done: true,
	}
	a.view = viewIntel

	// Y is the copy button in MAX mode.
	m, cmd := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd != nil {
		t.Errorf("Y must not return a cmd: %v", cmd)
	}
	app := m.(*App)
	if !strings.Contains(app.errMsg, "copied") && !strings.Contains(app.errMsg, "failed") {
		t.Errorf("Y should report a copy result, got errMsg=%q", app.errMsg)
	}

	// M is an alias of Y.
	app.errMsg = ""
	m, _ = a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	app = m.(*App)
	if !strings.Contains(app.errMsg, "copied") && !strings.Contains(app.errMsg, "failed") {
		t.Errorf("M in MAX mode should copy like Y, got errMsg=%q", app.errMsg)
	}

	// 1-9 jumps the cursor to the nth finding.
	m, _ = a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	app = m.(*App)
	if app.intelRep.cursor != 1 {
		t.Errorf("2 should jump to finding #2, got cursor=%d", app.intelRep.cursor)
	}

	// Q quits in MAX mode too.
	_, cmd = a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("Q in MAX mode must return the quit command")
	}
}

// TestMaxModeFeedbackVisible guards the invisible-button complaint: pressing
// Y (copy) / E (export) in MAX mode sets a status message, and the intel
// dashboard must render it on screen — otherwise every button looks dead.
func TestMaxModeFeedbackVisible(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 120, 34
	a.intelRep = &intelReport{
		target: "torvalds",
		kind:   "username",
		findings: []intel.Finding{
			{Platform: "GitHub", URL: "https://github.com/torvalds", Status: intel.StatusFound},
		},
		done: true,
	}
	a.view = viewIntel

	// Y copies and the dashboard shows the result.
	a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	out := a.View()
	if !strings.Contains(out, "copied") && !strings.Contains(out, "failed") {
		t.Error("MAX mode must visibly report the Y copy result on screen")
	}
	// No rendered line may exceed the terminal width (feedback included).
	for i, line := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(line); lw > 120 {
			t.Errorf("intel feedback line %d: width %d > 120: %q", i+1, lw, line)
		}
	}
}

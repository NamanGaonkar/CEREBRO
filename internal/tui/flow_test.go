package tui

import (
	"testing"

	"cerebro/internal/downloader"
	"cerebro/internal/model"

	tea "github.com/charmbracelet/bubbletea"
)

// key builds the right tea.KeyMsg for a key name ("enter", "esc", "p", "1"...).
func key(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
}

func seedResults(a *App) {
	a.results = []model.SearchResult{
		{ID: "1", Title: "Interstellar", Category: model.CatTorrent, Magnet: ""}, // no magnet → Stream errors, no mpv
		{ID: "2", Title: "Avengers Doomsday", Category: model.CatYouTube, VideoID: ""},
		{ID: "3", Title: "Cyberpunk 2077", Category: model.CatGame, Magnet: ""},
	}
	a.searching = false
	a.cursor = 0
}

// The user's flow: boot → search → P → close → Esc → P/M/D/Q must keep working.
func TestFlowUserReport(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	var m tea.Model = a

	// 1. boot — Init auto-focuses the omnibar so typing works
	a.Init()
	if !a.input.Focused() {
		t.Fatalf("boot: input NOT focused — typing will go nowhere")
	}

	// 2. type query + Enter → search runs, input blurs
	for _, r := range "avengers" {
		m, _ = m.Update(key(string(r)))
	}
	m, _ = m.Update(key("enter"))
	aa := m.(*App)
	if aa.input.Focused() {
		t.Fatalf("after Enter: input STILL focused — letter keys are trapped in the box")
	}
	t.Logf("after Enter: focused=%v view=%v", aa.input.Focused(), aa.view)

	// 3. results arrive
	seedResults(aa)
	aa.errMsg = ""
	m = aa

	// 4. press P on a result — must reach the stream handler (job starts,
	// view switches to the downloads dashboard). Stream is async: the job's
	// error (or success) arrives via jobMsg afterwards.
	t.Logf("pre-P: focused=%v view=%v inputVal=%q results=%d filtered=%d cursor=%d",
		aa.input.Focused(), aa.view, aa.input.Value(), len(aa.results), len(aa.filteredResults()), aa.cursor)
	m, _ = m.Update(key("p"))
	aa = m.(*App)
	if aa.view != viewDownloads {
		t.Fatalf("P did not start a stream job — view=%v want viewDownloads (focused=%v)", aa.view, aa.input.Focused())
	}
	t.Logf("P fired: view=%v (stream job started)", aa.view)

	// 5. user returns from the player → back to search view
	aa.view = viewSearch
	m = aa

	// 6. press P again — must fire again (same result: no magnet → job errors async)
	m, _ = m.Update(key("p"))
	aa = m.(*App)
	if aa.view != viewDownloads {
		t.Fatalf("second P did not fire — view=%v", aa.view)
	}
	t.Logf("second P fired: view=%v", aa.view)
	aa.view = viewSearch
	m = aa

	// 7. press M — must copy (sets errMsg feedback)
	aa.errMsg = ""
	m = aa
	m, _ = m.Update(key("m"))
	aa = m.(*App)
	if aa.errMsg == "" {
		t.Fatalf("M did not fire — errMsg empty")
	}
	t.Logf("M fired: errMsg=%q", aa.errMsg)

	// 8. press Q — must return tea.Quit
	m, cmd := m.Update(key("q"))
	_ = m
	if cmd == nil {
		t.Fatalf("Q did not return a quit command")
	}
	t.Logf("Q fired: cmd=%v", cmd)
}

// The empty-Enter trap: pressing Enter on the empty box must BLUR so letter
// keys become buttons — otherwise every P/M/D/Q types into the box.
func TestFlowEmptyEnterBlurs(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	var m tea.Model = a
	a.Init() // simulate boot
	if !a.input.Focused() {
		t.Fatalf("boot: not focused")
	}
	m, _ = m.Update(key("enter")) // empty Enter — the trap
	aa := m.(*App)
	if aa.input.Focused() {
		t.Fatalf("empty Enter left the box focused — every letter key is now trapped")
	}
	seedResults(aa)
	m = aa
	m, _ = m.Update(key("p"))
	aa = m.(*App)
	if aa.view != viewDownloads {
		t.Fatalf("P did not fire after empty-Enter blur — view=%v", aa.view)
	}
	t.Logf("empty-Enter blur OK; P fired: view=%v", aa.view)
}

// TestUppercaseButtons guards the Caps-Lock / Shift bug: the keyboard may
// send "P", "M", "D", "Q" (uppercase) but the handlers only matched
// lowercase, so every button silently did nothing while typing still worked.
func TestUppercaseButtons(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	seedResults(a)
	var m tea.Model = a

	// P (uppercase) must stream → view switches to downloads.
	m, _ = m.Update(key("P"))
	aa := m.(*App)
	if aa.view != viewDownloads {
		t.Fatalf("uppercase P did not stream — view=%v", aa.view)
	}
	aa.view = viewSearch
	m = aa

	// M (uppercase) must copy → sets feedback.
	m, _ = m.Update(key("M"))
	aa = m.(*App)
	if aa.errMsg == "" {
		t.Fatalf("uppercase M did not fire — errMsg empty")
	}
	t.Logf("uppercase M fired: %q", aa.errMsg)
	aa.errMsg = ""
	m = aa

	// D (uppercase) must open downloads.
	m, _ = m.Update(key("D"))
	aa = m.(*App)
	if aa.view != viewDownloads {
		t.Fatalf("uppercase D did not open downloads — view=%v", aa.view)
	}
	aa.view = viewSearch
	m = aa

	// J / K (uppercase) must navigate.
	m, _ = m.Update(key("J"))
	aa = m.(*App)
	if aa.cursor != 1 {
		t.Fatalf("uppercase J did not navigate — cursor=%d", aa.cursor)
	}
	m, _ = m.Update(key("K"))
	aa = m.(*App)
	if aa.cursor != 0 {
		t.Fatalf("uppercase K did not navigate — cursor=%d", aa.cursor)
	}

	// Q (uppercase) must quit.
	_, cmd := m.Update(key("Q"))
	if cmd == nil {
		t.Fatalf("uppercase Q did not quit")
	}
}

// TestSDownloadsToSearch guards the S key in downloads: it must ALWAYS return
// to search — even when reached via a pasted magnet/URL (which leaves
// lastQuery empty and previously made S silently do nothing).
func TestSDownloadsToSearch(t *testing.T) {
	// Case 1: no query (pasted a link) → S returns to search, no crash.
	a := New(downloader.NewManager("."))
	a.view = viewDownloads
	m, _ := a.Update(key("S"))
	aa := m.(*App)
	if aa.view != viewSearch {
		t.Fatalf("S with no query must return to search — view=%v", aa.view)
	}
	// Case 2: with a query → S returns to search and re-runs it.
	a2 := New(downloader.NewManager("."))
	a2.view = viewDownloads
	a2.lastQuery = "kartik"
	m2, cmd := a2.Update(key("s"))
	aa2 := m2.(*App)
	if aa2.view != viewSearch {
		t.Fatalf("S with query must return to search — view=%v", aa2.view)
	}
	if cmd == nil {
		t.Fatal("S with query must re-run the search (cmd expected)")
	}
}

// TestPasteLinkClassification guards pasting into the omnibar: magnets and
// URLs start downloads directly, YouTube URLs open the quality menu, and
// plain text searches.
func TestPasteLinkClassification(t *testing.T) {
	cases := []struct {
		in   string
		want inputKind
	}{
		{"magnet:?xt=urn:btih:abc123&dn=Movie+2024", inputMagnet},
		{"MAGNET:?xt=urn:btih:xyz&dn=Show", inputMagnet},
		{"https://example.com/file.zip", inputURL},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", inputYouTube},
		{"https://youtu.be/dQw4w9WgXcQ", inputYouTube},
		{"@torvalds", inputIntel},
		{"?quantum computing", inputIntel},
		{"avengers", inputSearch},
	}
	for _, c := range cases {
		if got := classifyInput(c.in); got != c.want {
			t.Errorf("classifyInput(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// j/k must navigate in the list when the box is not focused.
func TestFlowJKNav(t *testing.T) {
	mgr := downloader.NewManager(".")
	a := New(mgr)
	a.width, a.height = 100, 30
	seedResults(a)
	var m tea.Model = a
	m, _ = m.Update(key("j"))
	aa := m.(*App)
	if aa.cursor != 1 {
		t.Fatalf("j did not move cursor: got %d want 1", aa.cursor)
	}
	m, _ = m.Update(key("k"))
	aa = m.(*App)
	if aa.cursor != 0 {
		t.Fatalf("k did not move cursor: got %d want 0", aa.cursor)
	}
}

package tui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"cerebro/internal/doh"
	"cerebro/internal/downloader"
	"cerebro/internal/intel"
	"cerebro/internal/model"
	"cerebro/internal/scraper"
	"cerebro/internal/update"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type view int

const (
	viewSearch view = iota
	viewDownloads
	viewQuality
	viewIntel
)

// Status-refresh cadence. While downloads are active the ticker runs fast
// (4x/sec) so progress glides smoothly — the tick pulls authoritative job
// snapshots, so no update can ever be dropped. When idle it slows back down.
const (
	idleTickInterval   = 1 * time.Second
	activeTickInterval = 250 * time.Millisecond
)

// ---- messages ----

type resultsMsg struct {
	r     model.SearchResult
	epoch int
}
type searchDoneMsg struct{ epoch int }
type jobMsg struct{ job *model.DownloadJob }
type ytMenuMsg struct {
	options []string
	menu    map[string]string
	err     error
}
type tickMsg struct{}
type updateMsg struct{ latest string }

type intelMsg struct{ f intel.Finding }
type intelDoneMsg struct{ rep *intelReport }

// App is the root Bubble Tea model.
type App struct {
	width  int
	height int
	view   view

	input        textinput.Model
	filter       model.CategoryFilter
	results      []model.SearchResult
	cursor       int
	searching    bool
	lastQuery    string
	errMsg       string
	updateNotice string
	searchEpoch  int
	cancel       context.CancelFunc
	resCh        chan model.SearchResult

	owned map[string]bool

	spinner  spinner.Model
	progress progress.Model

	manager   *downloader.Manager
	jobs      []*model.DownloadJob
	jobCursor int
	prevBytes map[string]float64
	prevTime  time.Time
	aggSpeed  float64
	status    model.StatusInfo

	qualityResult  *model.SearchResult
	qualityOptions []string
	qualityMenu    map[string]string
	qualityCursor  int
	resolvingYT    bool

	// Cerebro Intel state.
	intelTarget string
	intelRep    *intelReport
	intelCh     <-chan intel.Finding
	intelDoneCh <-chan *intelReport
}

// New builds the root model.
func New(mgr *downloader.Manager) *App {
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(accent))
	sp.Spinner = spinner.Points

	in := textinput.New()
	in.Width = 48 // sensible default before the first WindowSizeMsg
	in.Placeholder = "search everything… or magnet/URL · @/? = MAX mode"
	in.Prompt = "⌕ "
	in.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(accent))
	in.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(text))
	in.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(accent))
	// Bright amber placeholder so the omnibar hint is always legible, even on
	// light or busy terminal backgrounds.
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(orange)).Bold(true)

	return &App{
		input:     in,
		filter:    model.FilterAll,
		spinner:   sp,
		progress:  progress.New(progress.WithDefaultGradient(), progress.WithWidth(46)),
		manager:   mgr,
		prevBytes: make(map[string]float64),
		view:      viewSearch,
		owned:     mgr.OwnedSet(),
	}
}

// SetIntelTarget arms a recon target before the program starts (used by
// `cerebro intel <target>`); Init launches the recon on boot.
func (a *App) SetIntelTarget(target string) {
	a.intelTarget = target
	a.intelRep = &intelReport{target: target, kind: intel.ClassifyKind(target)}
}

// Init starts the spinner and the status-refresh ticker; when a recon target
// was armed via `cerebro intel <target>`, the recon kicks off immediately.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		a.spinner.Tick,
		tea.Tick(idleTickInterval, func(time.Time) tea.Msg { return tickMsg{} }),
		a.checkForUpdate(),
	}
	if a.intelTarget != "" {
		cmds = append(cmds, a.launchIntel())
	} else {
		// Focus the omnibar on boot so "type to search" actually types —
		// textinput starts unfocused, which silently swallowed every key.
		a.input.Focus()
	}
	return tea.Batch(cmds...)
}

// checkForUpdate queries GitHub Releases once at startup and surfaces a
// one-line banner when a newer version is available. Offline / API errors are
// deliberately silent — the app must never block or nag because of a network
// hiccup, and local "dev" builds skip the check entirely.
func (a *App) checkForUpdate() tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 5 * time.Second}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		latest, err := update.CheckLatest(ctx, client)
		if err != nil {
			return updateMsg{} // offline, rate-limited… stay quiet
		}
		return updateMsg{latest: latest}
	}
}

// Update is the Elm-architecture message handler.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.input.Width = max(20, msg.Width-40)
		a.progress.Width = max(20, min(60, msg.Width-40))
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

	case tickMsg:
		a.refreshStatus()
		// 4x/sec while downloading so the bar moves smoothly, 1/sec when idle.
		interval := idleTickInterval
		if a.status.ActiveJobs > 0 {
			interval = activeTickInterval
		}
		return a, tea.Tick(interval, func(time.Time) tea.Msg { return tickMsg{} })

	case resultsMsg:
		if msg.epoch != a.searchEpoch {
			return a, nil // stale result from a previous search
		}
		a.addResult(msg.r)
		return a, a.waitForResults()

	case searchDoneMsg:
		if msg.epoch != a.searchEpoch {
			return a, nil // stale completion from a previous search
		}
		a.searching = false
		return a, nil

	case intelMsg:
		if a.intelRep != nil {
			a.intelRep.findings = append(a.intelRep.findings, msg.f)
		}
		return a, a.waitForIntel(a.intelCh, a.intelDoneCh)

	case intelDoneMsg:
		if msg.rep != nil {
			a.intelRep = msg.rep
		}
		if a.intelRep != nil {
			a.intelRep.done = true
		}
		return a, nil

	case jobMsg:
		if msg.job != nil {
			a.upsertJob(msg.job)
			// A finished download means this result is now owned — mark it so
			// the [OWNED] tag shows up on the next render.
			if msg.job.Status == model.StatusCompleted {
				a.owned[model.TitleCase(msg.job.Result.Title)] = true
				if h := model.ResultHash(msg.job.Result); h != "" {
					a.owned[h] = true
				}
			}
		}
		return a, a.waitForJob()

	case ytMenuMsg:
		a.resolvingYT = false
		if a.qualityResult == nil {
			return a, nil // user cancelled while streams were resolving
		}
		if msg.err != nil {
			a.errMsg = "youtube: " + msg.err.Error()
			return a, nil
		}
		a.qualityOptions = msg.options
		a.qualityMenu = msg.menu
		a.qualityCursor = 0
		a.view = viewQuality
		return a, nil

	case updateMsg:
		if msg.latest != "" {
			a.updateNotice = update.Notice(msg.latest)
		}
		return a, nil
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// StateSearchFocus: the omnibar owns most keys. Enter submits (and blurs to
	// the list), Down/Tab move to the list without clearing the text, Esc blurs
	// back to the list, everything else types.
	if a.input.Focused() {
		switch msg.String() {
		case "enter":
			q := strings.TrimSpace(a.input.Value())
			a.input.Blur()
			if q == "" {
				return a, nil
			}
			return a, a.resolveInput(q)
		case "esc":
			a.input.Blur()
			return a, nil
		case "down", "tab":
			a.input.Blur() // move focus to the results list
			return a, nil
		case "ctrl+c":
			return a, a.quit()
		default:
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			return a, cmd
		}
	}

	switch a.view {
	case viewQuality:
		switch msg.String() {
		case "esc":
			a.view = viewSearch
			a.qualityResult = nil
			return a, nil
		case "j", "J", "down":
			if len(a.qualityOptions) > 0 {
				a.qualityCursor = (a.qualityCursor + 1) % len(a.qualityOptions)
			}
		case "k", "K", "up":
			if len(a.qualityOptions) > 0 {
				a.qualityCursor = (a.qualityCursor - 1 + len(a.qualityOptions)) % len(a.qualityOptions)
			}
		case "enter":
			if len(a.qualityOptions) == 0 {
				return a, nil
			}
			q := a.qualityOptions[a.qualityCursor]
			r := *a.qualityResult
			r.QualityMap = a.qualityMenu
			if _, err := a.manager.Start(r, q); err != nil {
				a.errMsg = err.Error()
				return a, nil
			}
			a.qualityResult = nil
			a.view = viewDownloads
			a.jobCursor = 0
			return a, a.waitForJob()
		case "q", "Q", "ctrl+c":
			return a, a.quit()
		}
		return a, nil

	case viewIntel:
		if a.intelRep == nil {
			return a, nil
		}
		switch msg.String() {
		case "j", "J", "down":
			if n := len(a.intelRep.findings); n > 0 {
				a.intelRep.cursor = min(a.intelRep.cursor+1, n-1)
			}
			return a, nil
		case "k", "K", "up":
			a.intelRep.cursor = max(0, a.intelRep.cursor-1)
			return a, nil
		case "enter", "p", "P":
			// Enter opens the focused finding; P is the "play/preview" alias —
			// in MAX mode it opens the focused profile/URL in the browser.
			if f := a.intelFinding(); f != nil && f.URL != "" {
				if err := openInViewer(f.URL); err != nil {
					a.errMsg = "open: " + err.Error()
				}
			}
			return a, nil
		case "y", "Y", "m", "M":
			if f := a.intelFinding(); f != nil && f.URL != "" {
				if err := clipboard.WriteAll(f.URL); err != nil {
					a.errMsg = "copy failed: " + err.Error()
				} else {
					a.errMsg = "✓ URL copied to clipboard"
				}
			}
			return a, nil
		case "e", "E":
			path, err := exportIntelReport(a.intelRep)
			if err != nil {
				a.errMsg = "export: " + err.Error()
			} else {
				a.errMsg = "✓ report → " + path
			}
			return a, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Numbered rows: 1-9 jump the cursor straight to the nth finding.
			if n := len(a.intelRep.findings); n > 0 {
				if idx := int(msg.String()[0] - '1'); idx < n {
					a.intelRep.cursor = idx
				}
			}
			return a, nil
		case "esc":
			a.view = viewSearch
			return a, nil
		case "q", "ctrl+c":
			return a, a.quit()
		}
		return a, nil

	case viewDownloads:
		switch msg.String() {
		case "esc":
			a.view = viewSearch
			return a, nil
		case "j", "J", "down":
			if len(a.jobs) > 0 {
				a.jobCursor = min(a.jobCursor+1, len(a.jobs)-1)
			}
		case "k", "K", "up":
			a.jobCursor = max(0, a.jobCursor-1)
		case "s", "S":
			// Always return to search — even when there is no query (e.g. the
			// downloads view was reached by pasting a magnet/URL, which leaves
			// lastQuery empty). Re-run the last query only if one exists.
			a.view = viewSearch
			if a.lastQuery != "" {
				return a, a.runSearch(a.lastQuery)
			}
			return a, nil
		case "q", "ctrl+c":
			return a, a.quit()
		}
		return a, nil

	default: // viewSearch
		switch msg.String() {
		case "/":
			a.input.Focus()
			a.input.CursorEnd()
			return a, nil
		case "tab":
			a.filter = nextFilter(a.filter)
			return a, nil
		case "enter":
			rows := a.filteredResults()
			if len(rows) == 0 {
				return a, nil
			}
			if a.cursor >= len(rows) {
				a.cursor = len(rows) - 1
			}
			return a, a.selectResult(rows[a.cursor])
		case "j", "J", "down":
			if n := len(a.filteredResults()); n > 0 {
				a.cursor = min(a.cursor+1, n-1)
			}
			return a, nil
		case "k", "K", "up":
			a.cursor = max(0, a.cursor-1)
			return a, nil
		case "g":
			a.cursor = 0
			return a, nil
		case "G":
			a.cursor = max(0, len(a.filteredResults())-1)
			return a, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// 1-9 jump the cursor to the nth result (rows are numbered).
			if n := len(a.filteredResults()); n > 0 {
				if idx := int(msg.String()[0] - '1'); idx < n {
					a.cursor = idx
				}
			}
			return a, nil
		case "p", "P":
			rows := a.filteredResults()
			if len(rows) == 0 {
				return a, nil
			}
			r := rows[a.cursor]
			switch r.Category {
			case model.CatTorrent, model.CatGame, model.CatYouTube:
				// streamable — torrents pipe sequentially, YouTube streams best format
			case model.CatAudio, model.CatMP4, model.CatDirect, model.CatPDF, model.CatSoftware:
				if r.URL == "" {
					a.errMsg = "no playable URL on this result"
					return a, nil
				}
			default:
				a.errMsg = "P streams — pick a torrent, game, YouTube or music result"
				return a, nil
			}
			if _, err := a.manager.Stream(r); err != nil {
				a.errMsg = err.Error()
				return a, nil
			}
			a.view = viewDownloads
			a.jobCursor = 0
			return a, a.waitForJob()
		case "m", "M":
			rows := a.filteredResults()
			if len(rows) == 0 {
				return a, nil
			}
			r := rows[a.cursor]
			target := r.Magnet
			if target == "" {
				target = r.URL
			}
			if target == "" {
				a.errMsg = "nothing to copy for this result"
				return a, nil
			}
			if err := clipboard.WriteAll(target); err != nil {
				a.errMsg = "copy failed: " + err.Error()
			} else {
				a.errMsg = "✓ magnet/URL copied to clipboard"
			}
			return a, nil
		case "d", "D":
			a.view = viewDownloads
			a.jobCursor = 0
			return a, nil
		case "r", "R":
			if a.lastQuery != "" {
				return a, a.runSearch(a.lastQuery)
			}
		case "q", "Q", "ctrl+c":
			return a, a.quit()
		}
		return a, nil
	}
}

// resolveInput routes the fuzzy omnibar text: magnet links start a torrent
// download directly, URLs start a direct download (or the YouTube quality
// modal), and everything else runs a full multi-engine search.
func (a *App) resolveInput(q string) tea.Cmd {
	a.input.SetValue(q)
	a.input.Blur()
	switch classifyInput(q) {
	case inputMagnet:
		r := model.SearchResult{
			Title:    magnetName(q),
			Category: model.CatTorrent,
			Magnet:   q,
			Source:   "magnet",
		}
		if _, err := a.manager.Start(r, ""); err != nil {
			a.errMsg = err.Error()
			return nil
		}
		a.view = viewDownloads
		a.jobCursor = 0
		return a.waitForJob()
	case inputIntel:
		target := strings.TrimPrefix(q, "@")
		target = strings.TrimPrefix(target, "?")
		target = strings.TrimSpace(target)
		if target == "" {
			a.errMsg = "intel needs a target — @handle or ?topic"
			return nil
		}
		a.intelTarget = target
		a.input.SetValue("")
		a.input.Blur()
		return a.launchIntel()
	case inputYouTube:
		r := model.SearchResult{
			Title:    "YouTube video " + youtubeID(q),
			Category: model.CatYouTube,
			VideoID:  youtubeID(q),
			URL:      q,
			Source:   "youtube",
		}
		return a.selectResult(r)
	case inputURL:
		r := model.SearchResult{
			Title:    model.SanitizeFilename(q[strings.LastIndex(q, "/")+1:]),
			Category: model.CatDirect,
			URL:      q,
			Source:   "direct",
		}
		if r.Title == "" || r.Title == "download" {
			r.Title = "direct link"
		}
		if _, err := a.manager.Start(r, ""); err != nil {
			a.errMsg = err.Error()
			return nil
		}
		a.view = viewDownloads
		a.jobCursor = 0
		return a.waitForJob()
	default:
		if strings.TrimSpace(q) == "" {
			return nil // backspaced to empty — nothing to search
		}
		return a.runSearch(q)
	}
}

// openInViewer opens path in the OS default handler without blocking the TUI
// (cmd /c start detaches on Windows; xdg-open/open detach elsewhere). Used by
// Cerebro Intel to open profile URLs.
func openInViewer(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
		if _, err := os.Stat("/usr/bin/open"); err == nil {
			cmd = exec.Command("open", path)
		}
	}
	return cmd.Start()
}

// runSearch fires all engines concurrently and streams results live.
func (a *App) runSearch(q string) tea.Cmd {
	a.manager.RecordSearch(q)
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	a.cancel = cancel
	a.lastQuery = q
	a.searching = true
	a.errMsg = ""
	a.results = nil
	a.cursor = 0
	a.searchEpoch++
	a.input.SetValue(q)
	a.input.Blur()
	a.view = viewSearch

	ch := make(chan model.SearchResult, 64)
	a.resCh = ch
	go func() {
		defer close(ch)
		scraper.Search(ctx, q, func(r model.SearchResult) {
			select {
			case ch <- r:
			case <-ctx.Done():
			}
		})
	}()
	return a.waitForResults()
}

func (a *App) waitForResults() tea.Cmd {
	ch := a.resCh
	epoch := a.searchEpoch
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return searchDoneMsg{epoch: epoch}
		}
		return resultsMsg{r: r, epoch: epoch}
	}
}

// addResult dedupes and appends a streamed result, tagging [OWNED] results
// from the local download history.
func (a *App) addResult(r model.SearchResult) {
	if !model.IsPrintable(r.Title) {
		return
	}
	r.Owned = a.owned[model.TitleCase(r.Title)] || a.owned[model.ResultHash(r)]
	key := strings.ToLower(r.Title)
	for i, ex := range a.results {
		if strings.ToLower(ex.Title) == key {
			if r.Magnet != "" && r.Seeders > ex.Seeders {
				a.results[i] = r
			}
			return
		}
	}
	a.results = append(a.results, r)
}

// selectResult starts a download or opens the quality modal for YouTube.
func (a *App) selectResult(r model.SearchResult) tea.Cmd {
	switch r.Category {
	case model.CatYouTube:
		a.resolvingYT = true
		a.qualityResult = &r
		a.errMsg = ""
		return func() tea.Msg {
			opts, menu, err := downloader.YouTubeQualityMenu(r.VideoID)
			return ytMenuMsg{options: opts, menu: menu, err: err}
		}
	default:
		if _, err := a.manager.Start(r, ""); err != nil {
			a.errMsg = err.Error()
			return nil
		}
		a.view = viewDownloads
		a.jobCursor = 0
		return a.waitForJob()
	}
}

func (a *App) waitForJob() tea.Cmd {
	return func() tea.Msg {
		select {
		case j := <-a.manager.Updates():
			return jobMsg{job: j}
		case <-time.After(30 * time.Second):
			return jobMsg{job: nil}
		}
	}
}

// launchIntel starts a Cerebro Intel recon: switches to the intel dashboard
// and streams findings into the state loop as they resolve.
func (a *App) launchIntel() tea.Cmd {
	a.view = viewIntel
	a.intelRep = &intelReport{target: a.intelTarget, kind: intel.ClassifyKind(a.intelTarget)}
	ch := make(chan intel.Finding, 256)
	doneCh := make(chan *intelReport, 1)
	target := a.intelTarget
	go func() {
		client := doh.NewClient(20 * time.Second)
		// intel.Run closes ch itself (defer close(out)); never close it again
		// here — a second close panics and kills the terminal.
		rep := intel.Run(context.Background(), client, target, ch)
		doneCh <- &intelReport{target: rep.Target, kind: rep.Kind, bio: rep.Bio, links: rep.Links, findings: rep.Findings, done: true}
	}()
	a.intelCh = ch
	a.intelDoneCh = doneCh
	return a.waitForIntel(ch, doneCh)
}

// intelFinding returns the finding under the cursor, or nil.
func (a *App) intelFinding() *intel.Finding {
	if a.intelRep == nil {
		return nil
	}
	n := len(a.intelRep.findings)
	if n == 0 {
		return nil
	}
	if a.intelRep.cursor >= n {
		a.intelRep.cursor = n - 1
	}
	return &a.intelRep.findings[a.intelRep.cursor]
}

// waitForIntel bridges the recon goroutine's channels into Bubble Tea msgs.
func (a *App) waitForIntel(ch <-chan intel.Finding, doneCh <-chan *intelReport) tea.Cmd {
	return func() tea.Msg {
		select {
		case f, ok := <-ch:
			if !ok {
				// Findings channel closed — the report always follows on doneCh.
				select {
				case r := <-doneCh:
					return intelDoneMsg{rep: r}
				case <-time.After(5 * time.Second):
					return intelDoneMsg{}
				}
			}
			return intelMsg{f: f}
		case r := <-doneCh:
			return intelDoneMsg{rep: r}
		case <-time.After(90 * time.Second):
			return intelDoneMsg{}
		}
	}
}

func (a *App) upsertJob(j *model.DownloadJob) {
	if j == nil {
		return
	}
	for i, ej := range a.jobs {
		if ej.ID == j.ID {
			a.jobs[i] = j
			return
		}
	}
	a.jobs = append(a.jobs, j)
}

// refreshStatus recomputes speeds, aggregates and disk space on each tick.
func (a *App) refreshStatus() {
	now := time.Now()
	dt := now.Sub(a.prevTime).Seconds()
	snap := a.manager.Jobs()
	a.jobs = snap

	active, total := 0, 0.0
	for _, j := range a.jobs {
		if !j.IsActive() {
			continue
		}
		active++
		if dt > 0 {
			cur := float64(j.BytesDone)
			prev, ok := a.prevBytes[j.ID]
			if !ok {
				prev = cur
			}
			inst := (cur - prev) / dt
			if inst < 0 {
				inst = 0
			}
			j.Speed = j.Speed*0.6 + inst*0.4
			total += j.Speed
			a.prevBytes[j.ID] = cur
		}
	}
	a.aggSpeed = total
	a.status = model.StatusInfo{ActiveJobs: active, Speed: a.aggSpeed, Query: a.lastQuery, Searching: a.searching}
	if free, totalDisk, err := downloader.DiskFree(a.manager.Dir()); err == nil {
		a.status.DiskFree = free
		a.status.DiskTotal = totalDisk
	}
	a.prevTime = now
}

// filteredResults applies the category filter and the live input text.
func (a *App) filteredResults() []model.SearchResult {
	ftext := strings.ToLower(strings.TrimSpace(a.input.Value()))
	out := make([]model.SearchResult, 0, len(a.results))
	for _, r := range a.results {
		if !a.filter.Matches(r.Category) {
			continue
		}
		if ftext != "" {
			hay := strings.ToLower(r.Title + " " + r.Author + " " + r.Source)
			if !matchesAllWords(hay, ftext) {
				continue
			}
		}
		out = append(out, r)
	}
	// Live-filtering shrinks the list; keep the cursor inside it so P / M /
	// Enter never index out of range (a crash that made the buttons appear
	// dead after typing in the omnibar).
	if len(out) > 0 && a.cursor >= len(out) {
		a.cursor = len(out) - 1
	}
	return out
}

// matchesAllWords reports whether every whitespace-separated word in q appears
// in hay (order-independent). An exact-phrase check would empty the list for
// multi-word queries like "007 first light" because few titles contain the
// whole phrase contiguously (e.g. "James Bond 007: First Light - Gameplay").
func matchesAllWords(hay, q string) bool {
	for _, w := range strings.Fields(q) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

func (a *App) quit() tea.Cmd {
	if a.cancel != nil {
		a.cancel()
	}
	go a.manager.Close()
	return tea.Quit
}

// View renders the current screen.
func (a *App) View() string {
	if a.width == 0 {
		a.width = 100
	}
	if a.height == 0 {
		a.height = 30
	}

	// Boot screen: the omnibar is empty — before the first search, or after
	// backspacing a query away. The big banner + search block gets vertically
	// centered below, so the body stays compact here instead of stretching a
	// stale full-height result list across the screen.
	boot := a.view == viewSearch && strings.TrimSpace(a.input.Value()) == ""

	// Once a search has run (or we're off the search screen), collapse the big
	// banner to a compact title so results/dashboard own the screen. Backspace
	// to an empty omnibar always restores the big banner — a cleared input
	// means boot, regardless of what the previous query was.
	compact := !boot && (a.lastQuery != "" || a.view != viewSearch)
	header := RenderHeader(a.status, a.manager.Dir(), a.width, a.height, compact)

	// Height budget for the body block. The terminal shows a.height lines, so
	// the header, the blank separator lines, the optional update banner and the
	// footer help line must all fit first — otherwise the table grows to the
	// full screen and the terminal clips the bottom rows (the "chipped off"
	// results table with many hits).
	bodyH := a.height - lipgloss.Height(header) - 4 // header + 3 blank gaps + footer
	if a.updateNotice != "" {
		bodyH-- // the update banner line
	}
	if bodyH < 1 {
		bodyH = 1
	}
	if boot {
		// Natural boot body: tabs + search bar + hint + a compact empty panel.
		// No need to stretch it to the screen — the whole block is centered.
		bodyH = min(bodyH, 8)
	}

	var body string
	switch a.view {
	case viewDownloads:
		inner := renderDownloads(a.jobs, a.progress, a.aggSpeed, a.jobCursor, a.width, bodyH)
		body = resultsPanel(inner, a.width)
	case viewQuality:
		body = a.renderQualityModal()
	case viewIntel:
		if a.intelRep != nil {
			body = renderIntel(a.intelRep, a.width, bodyH, a.errMsg)
		}
	default:
		body = a.renderSearchBody(header, bodyH)
	}
	// Clamp the help line to the terminal width: lipgloss.JoinVertical pads
	// every line to the widest one, so a footer wider than the screen would
	// shove the whole centered layout off-axis.
	footer := centerLine(lipgloss.NewStyle().MaxWidth(a.width).Render(renderHelp(a.view)), a.width)
	parts := []string{header, ""}
	if a.updateNotice != "" {
		parts = append(parts, centerLine(fit(updateStyle.Render("▲ "+a.updateNotice), a.width), a.width))
	}
	parts = append(parts, "", body)

	if boot {
		// Dead-center the boot block in the space above the footer instead of
		// pinning it to the top row of the screen.
		content := lipgloss.JoinVertical(lipgloss.Left, parts...)
		pad := max(0, (a.height-1-lipgloss.Height(content))/2)
		return strings.Repeat("\n", pad) + content + "\n" + footer
	}
	parts = append(parts, "", footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (a *App) renderSearchBody(header string, bodyH int) string {
	// Boot state (empty omnibar after clearing a search): hide stale results
	// so the screen is a clean banner + search bar, never a leftover table.
	boot := strings.TrimSpace(a.input.Value()) == ""
	rows := a.filteredResults()
	if boot {
		rows = nil
	}
	if len(rows) > 0 && a.cursor >= len(rows) {
		a.cursor = len(rows) - 1
	}
	// Omnibar on top, category pills right beneath it, then the results.
	bar := centerBlock(renderSearchBar(&a.input, a.searching, a.spinner.View()), a.width)
	tabs := centerBlock(renderTabs(a.filter, true, a.width), a.width)

	var note string
	switch {
	case a.errMsg != "":
		note = errStyle.Render("  " + a.errMsg)
	case a.input.Focused():
		// The omnibar owns letter keys while focused, so P/M/D/Q would type
		// into the box instead of acting. Say so loudly — the user must know
		// they're in the search box and how to get out.
		note = searchModeStyle.Render("⌨  search box active — type & Enter · @/? = MAX mode · Esc/↓ to browse (P/M/D/Q become buttons there)")
	case boot:
		note = dimStyle.Render("  type to search every source · @handle / ?topic = MAX mode (OSINT)")
	case a.lastQuery != "":
		note = dimStyle.Render(fmt.Sprintf("  %d result(s) for %q", len(rows), a.lastQuery))
	default:
		note = dimStyle.Render("  press / to focus search · enter to run all engines")
	}
	// centerLine never truncates — clamp so a long note can't overflow a
	// narrow terminal (the boot hint is the longest one).
	note = centerLine(fit(note, a.width), a.width)

	// bodyH covers tabs + search bar + note + the panel; the panel itself uses
	// 2 border lines + the column header + the separator, leaving the rest for
	// result rows. The floor is 1 so extremely short terminals degrade to a
	// single row instead of overflowing the screen.
	maxRows := max(1, bodyH-7)
	panelW := resultsPanelWidth(a.width)

	// MAX split layout: a left metadata pane beside the results
	// table on wide terminals, the classic full-width table everywhere else.
	// The table is sized to its own box so rows never wrap.
	if len(rows) > 0 && a.width >= 100 && maxRows >= 4 {
		// A wider pane on roomy terminals means a roomier metadata card.
		paneW := 35
		if a.width >= 112 {
			paneW = 38
		}
		tableW := panelW - paneW - 2
		list := renderResults(rows, a.cursor, tableW-4, maxRows)
		sel := rows[a.cursor]
		pane := renderInfoPane(sel, paneW, maxRows+4)
		table := resultsPanelAt(list, tableW)
		body := centerBlock(lipgloss.JoinHorizontal(lipgloss.Top, pane, table), a.width)
		return lipgloss.JoinVertical(lipgloss.Left, tabs, bar, note, body)
	}

	var list string
	if a.searching && len(rows) == 0 {
		list = accentStyle.Render("  " + a.spinner.View() + " searching engines…")
	} else {
		list = renderResults(rows, a.cursor, panelW-4, maxRows)
	}
	return lipgloss.JoinVertical(lipgloss.Left, tabs, bar, note, resultsPanel(list, a.width))
}

func (a *App) renderQualityModal() string {
	var b strings.Builder
	if a.resolvingYT {
		b.WriteString(accentStyle.Render(a.spinner.View() + " fetching available qualities…"))
	} else {
		b.WriteString(headingStyle.Render("Choose quality — "+model.Truncate(a.qualityResult.Title, 48)) + "\n\n")
		for i, opt := range a.qualityOptions {
			if i == a.qualityCursor {
				b.WriteString(cursorStyle.Render("▸ ") + accentStyle.Render(opt) + "\n")
			} else {
				b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Render(opt) + "\n")
			}
		}
	}
	return centerBlock(panelStyle.Render(b.String()), a.width)
}

func renderHelp(v view) string {
	var keys [][2]string
	switch v {
	case viewQuality:
		keys = [][2]string{{"↑/↓", "move"}, {"enter", "select"}, {"esc", "cancel"}, {"q", "quit"}}
	case viewDownloads:
		keys = [][2]string{{"↑/↓", "scroll"}, {"esc", "search"}, {"s", "re-search"}, {"q", "quit"}}
	case viewIntel:
		keys = [][2]string{{"↑/↓", "move"}, {"1-9", "jump"}, {"enter/p", "open"}, {"y", "copy"}, {"e", "export"}, {"esc", "search"}, {"q", "quit"}}
	default:
		keys = [][2]string{{"/", "search"}, {"tab", "filter"}, {"↑/↓", "move"}, {"enter", "download"}, {"p", "stream"}, {"m", "copy"}, {"d", "downloads"}, {"q", "quit"}}
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, keyStyle.Render(k[0])+" "+dimStyle.Render(k[1]))
	}
	return dimStyle.Render(strings.Join(parts, "  ·  "))
}

func nextFilter(f model.CategoryFilter) model.CategoryFilter {
	order := []model.CategoryFilter{model.FilterAll, model.FilterBooks, model.FilterGames, model.FilterSoftware, model.FilterVideo, model.FilterAudio, model.FilterArchives}
	for i, v := range order {
		if v == f {
			return order[(i+1)%len(order)]
		}
	}
	return model.FilterAll
}

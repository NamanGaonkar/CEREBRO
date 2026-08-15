// Command app is the cerebro entry point: a TUI universal search and
// download manager for torrents, YouTube, PDFs, comics, movies and TV.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"cerebro/internal/db"
	"cerebro/internal/downloader"
	"cerebro/internal/tui"
	"cerebro/internal/update"

	tea "github.com/charmbracelet/bubbletea"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	intelTarget := ""
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("CEREBRO MAX %s — FIND EVERYTHING · DOWNLOAD ANYTHING — built by Naman Gaonkar\n", version)
			return
		case "--help", "-h", "help":
			fmt.Println("CEREBRO MAX — universal search & download manager")
			fmt.Println("Usage:")
			fmt.Println("  cerebro               open the terminal UI (search / download / stream)")
			fmt.Println("  cerebro intel <t>     run Cerebro Intel recon on a handle, person or topic")
			fmt.Println("  cerebro max <t>       alias for Intel (MAX Mode reconnaissance dashboard)")
			fmt.Println("  cerebro --version     print the version")
			return
		case "intel", "max":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "cerebro "+os.Args[1]+" <target> — e.g. cerebro "+os.Args[1]+" torvalds")
				os.Exit(1)
			}
			intelTarget = os.Args[2]
		}
	}
	dir := "downloads"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "cerebro: "+err.Error())
		os.Exit(1)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	update.CurrentVersion = version

	// Local SQLite store for download history, search history and [OWNED]
	// dedup. Opening it is best-effort — the app runs fine without it.
	d := openDB()
	if d != nil {
		defer d.Close()
	}
	mgr := downloader.NewManager(abs)
	if d != nil {
		mgr.SetDB(d)
	}
	m := tui.New(mgr)
	if intelTarget != "" {
		m.SetIntelTarget(intelTarget)
	}
	// No mouse tracking: this is a keyboard-driven TUI, and mouse-cell-motion
	// mode makes the pointer blink / swallow clicks in some terminals.
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "cerebro: "+err.Error())
		os.Exit(1)
	}
}

// openDB opens the cerebro history database at ~/.cerebro/cerebro.db.
func openDB() *db.DB {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	d, err := db.Open(filepath.Join(home, ".cerebro", "cerebro.db"))
	if err != nil {
		return nil
	}
	return d
}

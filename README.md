# CEREBRO MAX

### FIND EVERYTHING · DOWNLOAD ANYTHING

A high-performance, single-binary **TUI universal search & download manager** built in Go.

CEREBRO MAX searches **torrents, YouTube, books, games, software, movies and
music** across multiple engines at once and streams or downloads them straight
from a vibrant, Linear-style terminal interface built on
[Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Stack](https://img.shields.io/badge/go-1.26+-blue) ![UI](https://img.shields.io/badge/TUI-bubbletea-00F5FF)

---

## Install

Prebuilt binaries are published to **GitHub Releases** — pick whichever option fits your setup:

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/NamanGaonkar/CEREBRO/main/install.ps1 | iex
```

**Windows (Scoop)**

```powershell
scoop bucket add cerebro https://github.com/NamanGaonkar/CEREBRO
scoop install cerebro
```

**macOS / Linux**

```bash
curl -fsSL https://raw.githubusercontent.com/NamanGaonkar/CEREBRO/main/install.sh | bash
```

The installers download the latest release, place `cerebro` in `~/.cerebro/bin`,
add it to your `PATH` and verify it with `cerebro --version`. The Scoop bucket
is kept in sync automatically, so `scoop update cerebro` always gets you the
latest version.

## Updates

CEREBRO never updates itself silently. At startup it checks GitHub Releases
**once** and, if a newer version is available, shows a one-line banner telling
you what to run — no prompts, no background processes. It stays quiet when
you're offline or running a local "dev" build.

To update, use the same method you installed with:

| Installed with | Update with |
| --- | --- |
| **Scoop** | `scoop update cerebro` |
| **PowerShell one-liner** | re-run the `irm ... \| iex` command |
| **macOS / Linux one-liner** | re-run the `curl ... \| bash` command |

> 💡 Don't have Scoop yet on Windows? Install it once with
> `winget install ScoopInstaller.Scoop`, then add the bucket
> (`scoop bucket add cerebro https://github.com/NamanGaonkar/CEREBRO`) and
> install (`scoop install cerebro`) — every future update is then a single
> `scoop update cerebro`.

## Quick start

Run `cerebro` — the **fuzzy omnibar** is focused automatically, so just start
typing a query (e.g. `interstellar`), press `Enter`, arrow through the results
and press `Enter` to download. The omnibar also accepts pasted **magnet links**
(downloads instantly), **YouTube URLs** (quality menu) and any **direct URL**
(downloads straight away). Prefix a query with `@` or `?` to run **Cerebro
Intel** (`@handle` probes accounts across 90+ platforms, `?topic` runs a
knowledge deep-search, `@Full Name` runs a person search across the whole
internet), or run `cerebro intel <target>` from the shell. Files land in
`./downloads/`. Best experienced in Windows Terminal, iTerm2, or any 256-color
terminal.

| Key | Action |
| --- | --- |
| `/` | focus the omnibar (query · magnet · URL) |
| `Enter` | run all engines / download selected row / confirm quality |
| `Tab` | cycle category filter `[ALL] [BOOKS/PAPERS] [GAMES/REPACKS] [SOFTWARE/DEV] [VIDEO] [AUDIO] [ARCHIVES]` |
| `↑/↓` `j/k` | move cursor |
| `g` / `G` | jump to top / bottom |
| `1`-`9` | jump the cursor straight to the nth result |
| `P` | **instant-stream** the selected row in mpv (torrents, YouTube, music, direct links) |
| `M` | copy the selected row's magnet / URL to the clipboard |
| `d` | open downloads dashboard |
| `s` | re-run last search (from dashboard) |
| `r` | re-run last search |
| `Esc` | back / cancel |
| `q` / `Ctrl+C` | quit |

## 🕵️ MAX Mode (OSINT & deep research)

MAX Mode is Cerebro's autonomous reconnaissance engine — it takes a username,
a person's name, or any topic and probes **90+ public platforms** concurrently
(GitHub, Reddit, X/Twitter, YouTube, Steam, LinkedIn, ORCID, LeetCode, NPM,
PyPI, HuggingFace…), then synthesizes a **Wikipedia + DuckDuckGo** summary.
Zero API keys, all traffic routed through the built-in DoH transport.

**Three ways to launch it:**

| Way | Command / input | Example |
| --- | --- | --- |
| From the shell | `cerebro max <target>` | `cerebro max torvalds` |
| From the shell (alias) | `cerebro intel <target>` | `cerebro intel torvalds` |
| Inside the omnibar | `@handle` or `?topic` then `Enter` | `@torvalds` · `?quantum computing` |

**What you get:** a split dashboard — left pane shows the target summary
(identity, type, bio, reference links), right pane streams live findings
categorized `[FOUND]` / `[UNVERIFIED]` / `[NOT FOUND]` as they resolve.

**Smart intent classification** — the query is understood before any request
is made:

| You type | Mode | What runs |
| --- | --- | --- |
| `@torvalds` or `torvalds` | **username** | 90+ platform profile probes |
| `?quantum computing`, `what is amoeba`, `black holes` | **topic** | Wikipedia + DuckDuckGo **only** — never username probes |
| `Elon Musk`, `Kartik Sharma`, `@Naman Gaonkar` | **person** | profile probes + web-wide search + verified Wikipedia bio |

False-positive protection is built in: login-walled sites (Instagram, X,
Facebook, LinkedIn) are reported `[UNVERIFIED]`, never fake `[FOUND]`; soft-404
markers ("Sorry, nobody on Reddit goes by that name", "Couldn't find this
account"…) resolve to `[NOT FOUND]`; and the Wikipedia hit is scored against
the query so "what is amoeba" returns the organism — not the record store that
merely contains the word.

| Key | Action |
| --- | --- |
| `↑/↓` `j/k` | move through findings (rows are numbered `1.` `2.` `3.`…) |
| `1`-`9` | jump straight to the nth finding |
| `Enter` / `P` | open the selected profile/URL in your browser |
| `y` / `m` | copy the profile URL to the clipboard |
| `e` | export the full report to `~/.cerebro/reports/<target>.md` |
| `Esc` | back to search (in TUI) or quit (from shell) |

> Tips: `@` probes a username/account across platforms; `@Full Name` (e.g.
> `@Naman Gaonkar`) searches the person across the whole internet — accounts,
> articles, news and mentions; `?` treats the input as a topic/knowledge
> deep-search. Multi-word input (e.g. `cerebro max "Albert Einstein"`) is
> auto-detected as a person and gets a full Wikipedia bio + references.

## Features

- **Fuzzy omnibar** — the search box accepts plain queries *and* pasted
  `magnet:` links (starts the torrent immediately), YouTube URLs (opens the
  quality menu) or any direct URL (starts a direct download).
- **Anti-censorship (DoH)** — every scraper and download request resolves DNS
  over HTTPS (Cloudflare `1.1.1.1` → Quad9 `9.9.9.9`) so ISP-level tracker
  blocks are bypassed without a VPN; it falls back to your normal DNS
  automatically if DoH is ever unreachable.
- **Universal async search** — one query fans out to **12 engines** concurrently
  and results stream into the list live (no UI blocking):
  - **Video**: YTS (API), 1337x, Nyaa, Pirate Bay (via apibay), YouTube
    innertube search (no API key), Internet Archive video (mp4, webm, mkv)
  - **Books/Papers**: Library Genesis, Project Gutenberg, Internet Archive
    (every document format: pdf, epub, mobi, djvu, azw3, doc, txt, cbz, cbr —
    direct links, no captcha)
  - **Games/Repacks**: FitGirl repacks (magnet links) + game torrents from the
    indexers
  - **Software/Dev**: GitHub repository search (repo zipballs, direct download)
  - **Audio**: Internet Archive audio (mp3, flac, m4a, ogg — direct downloads)
- **Magnet enhancement** — every magnet gets a curated list of high-speed
  public trackers appended automatically for faster peer discovery.
- **Instant streaming** — press `P` on any torrent result to pipe it straight
  into **mpv**'s stdin (sequential piece streaming), on a **YouTube result**
  to hand mpv the best playable stream, or on **music / direct links** to
  play them straight in mpv. CEREBRO prefers the best video-only + audio-only
  streams merged in mpv (`--audio-file`) — that's **1080p/4K** quality, since
  the muxed formats YouTube still ships are 240p at best. Playback starts in
  seconds, no waiting.
- **Cerebro Intel** — autonomous OSINT / deep research, zero API keys: run
  `cerebro intel <target>` or type `@handle` / `?topic` in the omnibar. It
  probes **90+ platforms** concurrently (GitHub, Reddit, X, YouTube, Steam,
  LinkedIn, ORCID…), pulling a **Wikipedia + DuckDuckGo** summary for people
  and topics, then shows a split dashboard with `FOUND` / `UNVERIFIED` /
  `NOT FOUND` badges. `Enter` opens the profile, `y` copies the URL, `e`
  exports the whole report to `~/.cerebro/reports/<target>.md`, `Esc` returns.
- **Metadata card preview** — the left pane is a crisp high-contrast details
  card: category pill, bold wrapped title, source line, and a bordered specs
  box (size, format, source, health/seeders) plus an `[OWNED]` tag — pure
  text, so it stays razor-sharp on any terminal theme.
- **Local history & `[OWNED]` tags** — a built-in SQLite store
  (`~/.cerebro/cerebro.db`) records every download and search; the downloads
  dashboard (`d`) doubles as a **persistent history view** — everything you've
  ever downloaded is listed as `⇩ past download` even across sessions — and
  results you already have are tagged with a green `[OWNED]` badge.
- **Interactive downloads** — every row shows a live percent + bytes/speed
  (updating 4×/sec), the bar creeps visibly during the ffmpeg merge phase
  instead of freezing, and resolving/connecting states are surfaced so a job
  never looks stalled at 0%.
  - YouTube: quality modal (`4K MP4`, `1080p MP4`, `720p MP4`, `320kbps MP3`),
    raw-stream download + automatic **ffmpeg** merge into a single `.mp4`.
  - Torrents: in-memory magnet parsing, sequential piece streaming into
    `./downloads/` with live peer counts.
  - PDFs/Comics/Direct: **16-worker** parallel chunked downloader (Range
    headers, auto-resume via `.part`/`.part.meta` state).
- **Vibrant dashboard** — neon-cyan accent, bright category badges
  (`[TORRENT] [YOUTUBE] [DOC] [MUSIC] [MP4] [DIRECT]`), gradient ASCII banner
  that compacts to a centered title while results stream in, live status bar
  (active jobs, aggregate speed, free disk), progress bars with per-job speed
  and peer counts.

## Requirements

CEREBRO MAX works with **zero extra tools** for searching and downloading.
Two optional tools unlock extra features:

- **mpv** — only needed for **instant streaming** (the `P` key). Everything
  else works without it.
  - Windows: `winget install shinchiro.mpv`
  - macOS: `brew install mpv`
  - Linux: `sudo apt install mpv`
- **yt-dlp** — CEREBRO uses it to resolve YouTube streams reliably (even when
  a video's direct URLs expire). Streaming YouTube without it will fail, so
  install it alongside mpv.
  - Windows / any OS: `pip install yt-dlp` (or `winget install yt-dlp`)
  - macOS: `brew install yt-dlp`
  - Linux: `sudo apt install yt-dlp`
- **ffmpeg** — only needed for YouTube 4K/1080p merging and MP3 conversion;
  everything else works without it.
  - Windows: `winget install Gyan.FFmpeg`
  - macOS: `brew install ffmpeg`
  - Linux: `sudo apt install ffmpeg`
- Go 1.26+ — only if you want to build from source (see below).

> 💡 Windows note: `winget install mpv` will *not* find a package — the correct
> package id is `shinchiro.mpv`. If a tool isn't picked up right after
> installing it, open a **new** terminal window so your PATH refreshes.

## Build from source

Requires Go 1.26+ (Windows: `winget install GoLang.Go` · macOS: `brew install go` · Linux: `sudo apt install golang`).

```sh
# Windows (cmd / PowerShell)
build.bat

# macOS / Linux
chmod +x build.sh && ./build.sh
```

Or build directly:

```sh
go build -o cerebro ./cmd/app
```

Cross-compile to all three platforms from one machine:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o cerebro.exe ./cmd/app
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o cerebro-linux ./cmd/app
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o cerebro-mac ./cmd/app
```

## Project layout

```text
cmd/app/            main.go — Bubble Tea program bootstrap
internal/model/     SearchResult, DownloadJob, StatusInfo, trackers
internal/scraper/   concurrent engines (torrent.go, youtube.go, pdf.go)
internal/downloader torrent_client.go, youtube_dl.go, http_dl.go, manager.go
internal/doh/       DNS-over-HTTPS anti-censorship transport
internal/db/        SQLite history + [OWNED] dedup store
internal/streamer/  sequential torrent → mpv streaming
internal/cover/     inline ANSI half-block cover art
internal/tui/       header.go, search.go, table.go, progress.go, pane.go, app.go
```

## Roadmap

- **v2.x — CEREBRO MAX** (current): the universal media + OSINT workstation.
  Updates continue here as fixes and polish land.
- **v3.0 — CEREBRO ULTRA** (planned): the next-gen tier with deeper
  intelligence, more engines and a fully redesigned dashboard.

## Notes

- Search engines are public indexers and may occasionally be unreachable or
  rate-limit; each engine fails independently and gracefully.
- This tool is for personal use. Respect the terms of service and copyright of
  the sources and content you download.

## Credits

Built with ❤️ by **[Naman Gaonkar](https://github.com/NamanGaonkar)**.

- **Portfolio / contact the developer:** https://naman-gaonkar.vercel.app/
- **GitHub:** https://github.com/NamanGaonkar

## License

CEREBRO is released under the **MIT License** — you are free to use, copy,
modify, merge, publish, distribute, sublicense, and sell copies of the
software, subject to the license terms. See the [LICENSE](LICENSE) file for
the full text.

Copyright (c) 2026 Naman Gaonkar.
For licensing inquiries, contact the developer:
https://naman-gaonkar.vercel.app/

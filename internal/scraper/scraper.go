// Package scraper runs all search engines concurrently and streams
// unified results to the caller.
package scraper

import (
	"context"
	"fmt"
	"os"
	"sync"

	"cerebro/internal/model"
)

// EmitFunc receives each search result as soon as it is discovered.
type EmitFunc func(model.SearchResult)

// engineFunc searches a single engine for a query, streaming hits through emit
// the moment they are found so the UI never looks stuck waiting on a slow
// engine (torrents and YouTube already stream; IA engines now stream per item).
type engineFunc func(context.Context, string, EmitFunc)

// Search runs every engine concurrently and streams all results through emit.
// All categories are always searched; the UI filters client-side so Tab
// switching is instant. Search blocks until every engine finishes, so callers
// should run it in their own goroutine. The context cancels all engines.
func Search(ctx context.Context, query string, emit EmitFunc) {
	var wg sync.WaitGroup

	run := func(name string, fn engineFunc) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					debugf("[%s] recovered panic: %v", name, r)
				}
			}()
			var mu sync.Mutex
			n := 0
			fn(ctx, query, func(r model.SearchResult) {
				if r.Title == "" {
					return
				}
				mu.Lock()
				n++
				mu.Unlock()
				emit(r)
			})
			debugf("[%s] done: %d results", name, n)
		}()
	}

	run("yts", stream(SearchYTS))
	run("1337x", stream(Search1337x))
	run("nyaa", stream(SearchNyaa))
	run("tpb", stream(SearchPirateBay))
	run("youtube", stream(SearchYouTube))
	run("libgen", stream(SearchLibGen))
	run("gutenberg", stream(SearchGutenberg))
	run("fitgirl", stream(SearchFitGirl))
	run("github", stream(SearchGitHub))
	run("archive.docs", SearchInternetArchiveDocs)
	run("archive.music", SearchInternetArchiveAudio)
	run("archive.movies", SearchInternetArchiveMP4)

	wg.Wait()
}

// stream adapts a collect-then-return engine to the streaming signature.
func stream(fn func(context.Context, string) []model.SearchResult) engineFunc {
	return func(ctx context.Context, q string, emit EmitFunc) {
		for _, r := range fn(ctx, q) {
			emit(r)
		}
	}
}

// debugf prints engine diagnostics only when CEREBRO_DEBUG=1 so the TUI stays clean.
func debugf(format string, args ...any) {
	if os.Getenv("CEREBRO_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[scraper] "+format+"\n", args...)
	}
}

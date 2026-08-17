package intel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// workerLimit caps concurrent platform probes so a recon run never hammers
// every site at once.
const workerLimit = 12

// IsPersonName reports whether target looks like a person's name: at least
// two words made of letters, spaces, periods or hyphens.
func IsPersonName(target string) bool {
	fields := strings.Fields(target)
	if len(fields) < 2 {
		return false
	}
	return allNameTokens(fields)
}

// NormalizeHandle lowercases a target and keeps only URL-safe characters so
// it can be probed across platforms.
func NormalizeHandle(target string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(target)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			sb.WriteRune(r)
		case r == ' ':
			sb.WriteRune('-')
		}
	}
	return sb.String()
}

// Run performs a full recon pass on target: probes every platform
// concurrently and synthesizes a Wikipedia/DuckDuckGo summary. Findings are
// streamed to out as they resolve (out may be nil); the returned Report holds
// everything including the summary. out is closed before Run returns.
func Run(ctx context.Context, client *http.Client, target string, out chan<- Finding) Report {
	if out != nil {
		defer close(out)
	}
	intent := Classify(target)
	handle := NormalizeHandle(target)

	var mu sync.Mutex
	var findings []Finding
	add := func(f Finding) {
		mu.Lock()
		// Skip exact duplicate URLs — a web hit may repeat a probe URL, and
		// nobody wants the same row twice on the dashboard.
		for _, ex := range findings {
			if ex.URL != "" && ex.URL == f.URL {
				mu.Unlock()
				return
			}
		}
		findings = append(findings, f)
		mu.Unlock()
		if out != nil {
			select {
			case out <- f:
			default:
			}
		}
	}

	// Concept queries ("what is amoeba", "quantum computing") never probe
	// username profiles — that would fabricate github.com/what-is-amoeba
	// style false positives. Only handles and person names get platform
	// checks.
	var wg sync.WaitGroup
	sem := make(chan struct{}, workerLimit)
	if intent != IntentConcept {
		for _, p := range platforms {
			wg.Add(1)
			go func(p platform) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				add(probe(ctx, client, p, handle))
			}(p)
		}
	}

	// Web-wide organic coverage runs for EVERY intent, in parallel with the
	// probes (or alone for topics): handles get mentions and traces, person
	// names get articles and profiles, and topics get everything matching
	// the term on the internet — not just a Wikipedia summary. This is what
	// makes @mode show every trace of a name and ?mode show every hit for a
	// word, all in one dashboard.
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			return
		}
		// 12 unique hits merged across the search engines (DuckDuckGo Lite +
		// HTML + Mojeek), so @/? mode shows a genuinely wide web footprint.
		for _, h := range webSearch(ctx, client, target, 12) {
			add(Finding{Platform: "Web · " + hostOf(h.URL), URL: h.URL, Status: StatusFound, Detail: h.Title})
		}
	}()

	// Synthesis runs in parallel with the probes. For person names, the
	// Wikipedia hit is verified against the query (similarity check) so a
	// wrong celebrity's bio is never served as "the" answer.
	bio, links := "", []string(nil)
	done := make(chan struct{})
	go func() {
		b, l := synthesize(ctx, client, target)
		bio, links = b, l
		close(done)
	}()

	wg.Wait()
	<-done

	if bio != "" {
		page := "https://en.wikipedia.org/wiki/Special:Search?search=" + strings.ReplaceAll(target, " ", "+")
		if len(links) > 0 {
			page = links[0]
		}
		add(Finding{Platform: "Wikipedia", URL: page, Status: StatusFound, Detail: "summary / abstract"})
	} else if intent == IntentPerson {
		// A person name with no verifiable Wikipedia entry: say so instead of
		// silently dropping the summary.
		add(Finding{Platform: "Wikipedia", URL: "https://en.wikipedia.org/wiki/Special:Search?search=" + strings.ReplaceAll(target, " ", "+"), Status: StatusNotFound, Detail: "no exact Wikipedia entry found"})
	}
	if intent == IntentConcept {
		add(Finding{Platform: "DuckDuckGo", URL: "https://duckduckgo.com/?q=" + strings.ReplaceAll(target, " ", "+"), Status: StatusFound, Detail: "search reference"})
		add(Finding{Platform: "Wikipedia", URL: "https://en.wikipedia.org/wiki/Special:Search?search=" + strings.ReplaceAll(target, " ", "+"), Status: StatusFound, Detail: "search reference"})
	}

	return Report{Target: target, Kind: intent.String(), Bio: bio, Links: links, Findings: findings}
}

// probe checks one platform for the handle and returns the finding.
func probe(ctx context.Context, client *http.Client, p platform, handle string) Finding {
	if handle == "" {
		return Finding{Platform: p.name, Status: StatusNotFound, Detail: "empty handle"}
	}
	u := p.url(handle)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Finding{Platform: p.name, URL: u, Status: StatusUnverified, Detail: "invalid target URL"}
	}
	req.Header.Set("User-Agent", "cerebro-intel/2.0")
	resp, err := client.Do(req)
	if err != nil {
		return Finding{Platform: p.name, URL: u, Status: StatusUnverified, Detail: "request failed"}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	st := defaultCheck(resp.StatusCode, string(body))
	if p.check != nil {
		st = p.check(resp.StatusCode, string(body))
	}
	return Finding{Platform: p.name, URL: u, Status: st, Detail: detailFor(p.name, st)}
}

// detailFor picks a human-readable note for a status on a platform.
func detailFor(platform string, st Status) string {
	switch st {
	case StatusFound:
		return "account exists"
	case StatusNotFound:
		return "no account"
	default:
		return fmt.Sprintf("unverifiable (%s)", platform)
	}
}

package intel

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// webHit is one organic web-search result (title + real URL + snippet).
type webHit struct {
	Title   string
	URL     string
	Snippet string
}

var (
	// Full HTML endpoint markup.
	reWebTitle   = regexp.MustCompile(`(?s)<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	reWebSnippet = regexp.MustCompile(`(?s)<a[^>]*class="result__snippet"[^>]*>(.*?)</a>`)
	// DuckDuckGo Lite markup: bare result links, far less bot-walled.
	reLiteLink = regexp.MustCompile(`(?s)<a rel="nofollow" href="([^"]+)"[^>]*>(.*?)</a>`)
	reTag      = regexp.MustCompile(`(?s)<[^>]+>`)
)

// webSearch runs organic web searches across fallback engines and returns the
// top hits with their real (unwrapped) URLs. DuckDuckGo Lite is tried first
// (simple, reliable markup that rarely rate-limits); the full HTML endpoint
// backs it up with snippets. This is what makes recon diverse: typing a name,
// handle or topic surfaces articles, profiles, news and mentions across the
// internet — not just platform probes or a Wikipedia summary. Failures
// return nil.
func webSearch(ctx context.Context, client *http.Client, q string, limit int) []webHit {
	if hits := webSearchLite(ctx, client, q, limit); len(hits) > 0 {
		return hits
	}
	return webSearchHTML(ctx, client, q, limit)
}

// webSearchLite scrapes DuckDuckGo Lite: a bare table of result links.
func webSearchLite(ctx context.Context, client *http.Client, q string, limit int) []webHit {
	body := fetch(ctx, client, "https://lite.duckduckgo.com/lite/?q="+url.QueryEscape(q))
	if body == "" {
		return nil
	}
	var hits []webHit
	for _, m := range reLiteLink.FindAllStringSubmatch(body, -1) {
		if len(m) < 3 || limit > 0 && len(hits) >= limit {
			break
		}
		real := unwrapDDG(m[1])
		if real == "" || isAdURL(real) {
			continue
		}
		title := cleanHTML(m[2])
		if title == "" {
			continue
		}
		hits = append(hits, webHit{Title: title, URL: real})
	}
	return hits
}

// webSearchHTML scrapes the full DuckDuckGo HTML endpoint (with snippets).
func webSearchHTML(ctx context.Context, client *http.Client, q string, limit int) []webHit {
	body := fetch(ctx, client, "https://html.duckduckgo.com/html/?q="+url.QueryEscape(q))
	if body == "" {
		return nil
	}
	titleM := reWebTitle.FindAllStringSubmatch(body, -1)
	snipM := reWebSnippet.FindAllStringSubmatch(body, -1)
	var hits []webHit
	for i, m := range titleM {
		if len(m) < 3 || limit > 0 && len(hits) >= limit {
			break
		}
		real := unwrapDDG(m[1])
		if real == "" || isAdURL(real) {
			continue
		}
		title := cleanHTML(m[2])
		if title == "" {
			continue
		}
		snip := ""
		if i < len(snipM) && len(snipM[i]) > 1 {
			snip = cleanHTML(snipM[i][1])
		}
		hits = append(hits, webHit{Title: title, URL: real, Snippet: snip})
	}
	return hits
}

// fetch GETs a URL with a browser User-Agent and returns the body (up to
// 1MB), or "" on any failure.
func fetch(ctx context.Context, client *http.Client, u string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	return string(b)
}

// isAdURL reports whether a search-result URL is a sponsored/ad redirect
// (DuckDuckGo lite and html endpoints embed ads with these markers).
func isAdURL(u string) bool {
	return strings.Contains(u, "ad_domain") || strings.Contains(u, "ad_provider") ||
		strings.Contains(u, "aclick") || strings.Contains(u, "y.js")
}

// unwrapDDG converts a DuckDuckGo redirect link (/l/?uddg=…) back to the real
// destination URL; plain http(s):// hrefs pass through untouched.
func unwrapDDG(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if i := strings.Index(href, "uddg="); i >= 0 {
		enc := href[i+len("uddg="):]
		if j := strings.IndexAny(enc, "&\"'"); j >= 0 {
			enc = enc[:j]
		}
		if dec, err := url.QueryUnescape(enc); err == nil && strings.HasPrefix(dec, "http") {
			return dec
		}
	}
	return ""
}

// cleanHTML strips tags and collapses whitespace from a scraped fragment.
func cleanHTML(s string) string {
	s = reTag.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&#x27;", "'")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return strings.Join(strings.Fields(s), " ")
}

// hostOf returns the bare host (github.com) of a URL, used as the platform
// label for web-search findings.
func hostOf(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexAny(u, "/?"); i >= 0 {
		u = u[:i]
	}
	return strings.TrimPrefix(u, "www.")
}

package intel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// wikiSummary is the relevant slice of the Wikipedia REST summary payload.
type wikiSummary struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Extract     string `json:"extract"`
	ContentURLs struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

// wikiSearchResult is the slice of the MediaWiki search API we need.
type wikiSearchResult struct {
	Query struct {
		Search []struct {
			Title string `json:"title"`
		} `json:"search"`
	} `json:"query"`
}

// ddgInstant is the slice of the DuckDuckGo Instant Answer API we need.
type ddgInstant struct {
	Abstract string `json:"Abstract"`
	Answer   string `json:"Answer"`
	Heading  string `json:"Heading"`
}

// synthesize gathers an abstract/answer for a person, entity or topic from
// Wikipedia and DuckDuckGo Instant Answers. It returns a short bio and any
// reference links worth pinning to the summary card. Failures are silent.
func synthesize(ctx context.Context, client *http.Client, target string) (bio string, links []string) {
	if s, link := wikiLookup(ctx, client, target); s != "" {
		bio = s
		if link != "" {
			links = append(links, link)
		}
	}
	if a := ddgLookup(ctx, client, target); a != "" {
		if bio == "" {
			bio = a
		} else {
			bio = a + " — " + bio
		}
	}
	return bio, links
}

// wikiLookup searches Wikipedia for the target and pulls the top hit's
// summary extract plus its canonical page URL. The hit is verified against
// the query (see wikiMatch) so a wrong celebrity's bio is never presented as
// the answer for a different person.
func wikiLookup(ctx context.Context, client *http.Client, target string) (string, string) {
	search := "https://en.wikipedia.org/w/api.php?action=query&list=search&srlimit=3&format=json&srsearch=" +
		url.QueryEscape(target)
	var sr wikiSearchResult
	if err := getJSON(ctx, client, search, &sr); err != nil {
		return "", ""
	}
	if len(sr.Query.Search) == 0 {
		return "", ""
	}
	// Score every candidate and take the best verifiable match (never blindly
	// the top fuzzy hit): "what is amoeba" must pick the organism article
	// over the record store that merely contains the word "amoeba".
	var title string
	best := 0.0
	for _, hit := range sr.Query.Search {
		if hit.Title == "" {
			continue
		}
		if s := wikiScore(target, hit.Title); s > best {
			best = s
			title = hit.Title
		}
	}
	if best < 0.75 || title == "" {
		return "", ""
	}
	summary := "https://en.wikipedia.org/api/rest_v1/page/summary/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	var ws wikiSummary
	if err := getJSON(ctx, client, summary, &ws); err != nil {
		return "", ""
	}
	extract := strings.TrimSpace(ws.Extract)
	if len(extract) > 600 {
		extract = extract[:600] + "…"
	}
	page := ws.ContentURLs.Desktop.Page
	if page == "" {
		page = "https://en.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	}
	return extract, page
}

// wikiMatch reports whether a candidate title verifiably matches the query
// (wikiScore >= 0.75).
func wikiMatch(query, title string) bool {
	return wikiScore(query, title) >= 0.75
}

// wikiScore returns a match confidence in [0,1] for a candidate Wikipedia
// page title against the query. 1.0 means every significant query token
// appears as a full word in the title; a small penalty subtracts for extra
// title words so "Amoeba" outscores "Amoeba Music" for "what is amoeba".
// Multi-token queries never fuzzy-match partial names ("John Smith" must not
// match "John Smithers"); single tokens get a Levenshtein fallback.
func wikiScore(query, title string) float64 {
	q := normalizeWiki(query)
	t := normalizeWiki(title)
	if q == "" || t == "" {
		return 0
	}
	if q == t {
		return 1
	}
	tWords := map[string]bool{}
	for _, w := range strings.Fields(t) {
		tWords[w] = true
	}
	significant := make([]string, 0, len(strings.Fields(q)))
	for _, tok := range strings.Fields(q) {
		if !isConceptStopword(tok) {
			significant = append(significant, tok)
		}
	}
	if len(significant) == 0 {
		return 0
	}
	matched := 0
	for _, tok := range significant {
		if tWords[tok] {
			matched++
		}
	}
	if matched == len(significant) {
		extra := len(tWords) - len(significant)
		if extra < 0 {
			extra = 0
		}
		score := 1 - 0.1*float64(extra)
		if score < 0.75 {
			return 0.75
		}
		return score
	}
	// Multi-token queries: token overlap is the authority — no fuzzy names.
	if len(significant) > 1 {
		return 0
	}
	// Single-token queries get a Levenshtein fallback for near-identical
	// spellings.
	return levenshteinRatio(q, t)
}

// normalizeWiki lowercases and strips punctuation/parenthetical suffixes for
// comparison ("Kartik Sharma (actor)" → "kartik sharma actor").
func normalizeWiki(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == ' ':
			sb.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// levenshteinRatio returns normalized similarity in [0,1]: 1 - distance/maxlen.
func levenshteinRatio(a, b string) float64 {
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	dist := prev[len(b)]
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 0
	}
	return 1 - float64(dist)/float64(maxLen)
}

// ddgLookup queries DuckDuckGo's Instant Answer API.
func ddgLookup(ctx context.Context, client *http.Client, target string) string {
	u := "https://api.duckduckgo.com/?format=json&no_html=1&skip_disambig=1&q=" + url.QueryEscape(target)
	var d ddgInstant
	if err := getJSON(ctx, client, u, &d); err != nil {
		return ""
	}
	out := strings.TrimSpace(d.Abstract)
	if out == "" {
		out = strings.TrimSpace(d.Answer)
	}
	if out == "" {
		return ""
	}
	if len(out) > 400 {
		out = out[:400] + "…"
	}
	return out
}

// getJSON fetches u and decodes the JSON body into v. Non-2xx or bad JSON are
// treated as a silent miss.
func getJSON(ctx context.Context, client *http.Client, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "cerebro-intel/2.0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return io.ErrUnexpectedEOF
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}

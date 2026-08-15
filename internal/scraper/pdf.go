package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"cerebro/internal/model"

	"github.com/PuerkitoBio/goquery"
)

// ---- LibGen (HTML scrape: books, papers, comics) ----

var libgenHosts = []string{"https://libgen.is", "https://libgen.rs", "https://libgen.st"}

// SearchLibGen scrapes Library Genesis for books, papers and comics.
// The URL stored on results is an ads.php?md5= page; the downloader resolves
// it to the actual file endpoint at download time.
// All mirrors are probed concurrently under a deadline so a blocked mirror
// (these hosts are Cloudflare-protected from many networks) can never stall
// the whole search for tens of seconds.
func SearchLibGen(ctx context.Context, query string) []model.SearchResult {
	sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	type out struct {
		res []model.SearchResult
	}
	ch := make(chan out, len(libgenHosts))
	var wg sync.WaitGroup
	for _, host := range libgenHosts {
		host := host
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- out{res: scrapeLibGenHost(sctx, host, query)}
		}()
	}
	wg.Wait()
	close(ch)
	for o := range ch {
		if len(o.res) > 0 {
			return o.res
		}
	}
	return nil
}

// scrapeLibGenHost queries a single mirror and parses the results table.
func scrapeLibGenHost(ctx context.Context, host, query string) []model.SearchResult {
	u := host + "/search.php?req=" + url.QueryEscape(query) + "&res=25&view=simple&column=def"
	body, status, err := get(ctx, u)
	if err != nil || status != 200 {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	var out []model.SearchResult
	doc.Find("table.catalog tr").Each(func(_ int, s *goquery.Selection) {
		titleLink := s.Find("td:nth-child(3) a").First()
		title := strings.TrimSpace(titleLink.Text())
		href, _ := titleLink.Attr("href")
		if title == "" || href == "" {
			return
		}
		md5 := extractMD5(href)
		page := href
		if md5 != "" {
			page = host + "/ads.php?md5=" + md5
		}
		ext := strings.ToLower(strings.TrimSpace(s.Find("td:nth-child(9)").Text()))
		if ext == "" {
			ext = guessExt(title)
		}
		cover := ""
		if md5 != "" {
			cover = fmt.Sprintf("https://libgen.is/covers/%s/%s/%s/%s/%s.jpg",
				md5[:1], md5[1:2], md5[2:3], md5[3:4], md5)
		}
		out = append(out, model.SearchResult{
			ID:           model.NewID(),
			Title:        title,
			Category:     model.CatPDF,
			Source:       "libgen",
			URL:          page,
			ThumbnailURL: cover,
			Size:         strings.TrimSpace(s.Find("td:nth-child(8)").Text()),
			Author:       strings.TrimSpace(s.Find("td:nth-child(2)").Text()),
			Ext:          ext,
		})
	})
	return out
}

// extractMD5 pulls the md5 query value out of a libgen link.
func extractMD5(s string) string {
	i := strings.Index(s, "md5=")
	if i < 0 {
		return ""
	}
	rest := s[i+4:]
	if j := strings.IndexAny(rest, "&\"'"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

// guessExt infers a file extension from a title when libgen's table is vague.
func guessExt(title string) string {
	low := strings.ToLower(title)
	for _, e := range []string{"pdf", "epub", "cbz", "cbr", "mobi", "djvu", "azw3"} {
		if strings.Contains(low, "."+e) {
			return e
		}
	}
	return ""
}

// ---- Project Gutenberg (via Gutendex JSON API, direct downloads) ----

// SearchGutenberg queries the Gutendex API for public-domain books with
// direct download links (epub/txt).
func SearchGutenberg(ctx context.Context, query string) []model.SearchResult {
	u := "https://gutendex.com/books?search=" + url.QueryEscape(query)
	var resp struct {
		Count   int `json:"count"`
		Results []struct {
			ID      int    `json:"id"`
			Title   string `json:"title"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
			Formats map[string]string `json:"formats"`
		} `json:"results"`
	}
	if err := getJSON(ctx, u, &resp); err != nil {
		return nil
	}
	var out []model.SearchResult
	for _, b := range resp.Results {
		dl, ext := pickGutenbergFormat(b.Formats)
		if dl == "" || b.Title == "" {
			continue
		}
		author := ""
		if len(b.Authors) > 0 {
			author = b.Authors[0].Name
		}
		out = append(out, model.SearchResult{
			ID:           fmt.Sprintf("gut-%d", b.ID),
			Title:        b.Title,
			Category:     model.CatDirect,
			Source:       "gutenberg",
			URL:          dl,
			ThumbnailURL: b.Formats["image/jpeg"],
			Author:       author,
			Ext:          ext,
		})
	}
	return out
}

// pickGutenbergFormat prefers epub, then plain text, then mobi.
func pickGutenbergFormat(formats map[string]string) (string, string) {
	for _, mime := range []string{
		"application/epub+zip",
		"text/plain",
		"text/plain; charset=us-ascii",
		"application/octet-stream",
	} {
		if u, ok := formats[mime]; ok {
			switch mime {
			case "application/epub+zip":
				return u, "epub"
			case "application/octet-stream":
				return u, "mobi"
			default:
				return u, "txt"
			}
		}
	}
	return "", ""
}

// ---- Internet Archive (books, comics, music, movies — official JSON API) ----

// The Internet Archive's advancedsearch + metadata endpoints are plain JSON —
// no captcha, no browser — and every download URL supports Range requests, so
// all IA hits download directly. Three engines share one implementation:
// docs (every document format), music (mp3/flac/…), and mp4 (video files).

// SearchInternetArchiveDocs finds books, comics and documents in any format:
// pdf, epub, mobi, djvu, azw3, doc, docx, txt, cbz, cbr…
func SearchInternetArchiveDocs(ctx context.Context, query string, emit EmitFunc) {
	iaSearch(ctx, query, "texts", model.CatPDF, emit,
		"pdf", "epub", "mobi", "djvu", "azw3", "docx", "doc", "txt", "cbz", "cbr")
}

// SearchInternetArchiveAudio finds music and audio items (mp3, flac, m4a…).
func SearchInternetArchiveAudio(ctx context.Context, query string, emit EmitFunc) {
	iaSearch(ctx, query, "audio", model.CatAudio, emit,
		"mp3", "flac", "m4a", "ogg", "wav", "opus", "aac")
}

// SearchInternetArchiveMP4 finds movies and videos (mp4, webm, mkv…).
func SearchInternetArchiveMP4(ctx context.Context, query string, emit EmitFunc) {
	iaSearch(ctx, query, "movies", model.CatMP4, emit,
		"mp4", "webm", "mkv", "avi", "mov")
}

// iaSearch runs one advancedsearch query, then resolves each item's best file
// URL through the metadata API using a small worker pool and emits each hit as
// soon as it resolves — results appear progressively instead of all at the end.
func iaSearch(ctx context.Context, query, mediatype, category string, emit EmitFunc, prefs ...string) {
	// Bound the whole engine so slow metadata lookups can't stall the search
	// tail; whatever resolved before the deadline is still emitted.
	sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	ctx = sctx
	q := url.QueryEscape(query + " AND mediatype:" + mediatype)
	u := "https://archive.org/advancedsearch.php?q=" + q +
		"&fl%5B%5D=identifier&fl%5B%5D=title&fl%5B%5D=creator&rows=8&page=1&output=json"
	var resp struct {
		Response struct {
			Docs []struct {
				Identifier string          `json:"identifier"`
				Title      string          `json:"title"`
				Creator    json.RawMessage `json:"creator"`
			} `json:"docs"`
		} `json:"response"`
	}
	if err := getJSON(ctx, u, &resp); err != nil {
		debugf("archive.org %s: %v", mediatype, err)
		return
	}

	var wg sync.WaitGroup
	sema := make(chan struct{}, 4)
	for _, d := range resp.Response.Docs {
		if ctx.Err() != nil {
			break // a newer search superseded this one
		}
		if d.Identifier == "" || d.Title == "" {
			continue
		}
		d := d
		wg.Add(1)
		sema <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sema }()
			dl, ext := iaDownloadURL(ctx, d.Identifier, prefs...)
			if dl == "" || ctx.Err() != nil {
				return
			}
			emit(model.SearchResult{
				ID:           "ia-" + d.Identifier,
				Title:        d.Title,
				Category:     category,
				Source:       "archive.org",
				URL:          dl,
				ThumbnailURL: "https://archive.org/services/img/" + url.PathEscape(d.Identifier),
				Author:       creatorString(d.Creator),
				Ext:          ext,
			})
		}()
	}
	wg.Wait()
}

// creatorString parses archive.org's creator field, which is a plain string
// when there is a single author and an array when there are several.
func creatorString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		return arr[0]
	}
	return ""
}

// iaDownloadURL fetches an item's metadata and returns the download URL of the
// best file matching the preference order (e.g. pdf before epub before txt).
// One retry guards against archive.org's occasional slow metadata responses.
func iaDownloadURL(ctx context.Context, id string, prefs ...string) (string, string) {
	u := "https://archive.org/metadata/" + url.PathEscape(id)
	var meta struct {
		Files []struct {
			Name   string `json:"name"`
			Format string `json:"format"`
		} `json:"files"`
	}
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if err = getJSON(ctx, u, &meta); err == nil {
			break
		}
		if attempt == 0 {
			time.Sleep(300 * time.Millisecond)
		}
	}
	if err != nil {
		return "", ""
	}
	best, bestRank, ext := "", 999, ""
	for _, f := range meta.Files {
		low := strings.ToLower(f.Format + " " + f.Name)
		for i, pref := range prefs {
			if strings.Contains(low, pref) {
				if i < bestRank {
					best, bestRank, ext = f.Name, i, pref
				}
				break
			}
		}
	}
	if best == "" {
		return "", ""
	}
	parts := strings.Split(best, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "https://archive.org/download/" + url.PathEscape(id) + "/" + strings.Join(parts, "/"), ext
}

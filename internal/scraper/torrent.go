package scraper

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"cerebro/internal/model"

	"github.com/PuerkitoBio/goquery"
)

// ---- YTS (official JSON API) ----

var ytsHosts = []string{"https://yts.mx", "https://yts.am", "https://yts.li"}

type ytsTorrent struct {
	Quality string `json:"quality"`
	Type    string `json:"type"`
	Size    string `json:"size"`
	Seed    int    `json:"seeds"`
	Peers   int    `json:"peers"`
	URL     string `json:"url"`
}

type ytsMovie struct {
	ID          int          `json:"id"`
	TitleLong   string       `json:"title_long"`
	Year        int          `json:"year"`
	URL         string       `json:"url"`
	MediumCover string       `json:"medium_cover_image"`
	Torrents    []ytsTorrent `json:"torrents"`
}

// SearchYTS queries the YTS public API for movie torrents.
func SearchYTS(ctx context.Context, query string) []model.SearchResult {
	var out []model.SearchResult
	for _, host := range ytsHosts {
		u := fmt.Sprintf("%s/api/v2/list_movies.json?query_term=%s&limit=20&sort_by=seeds",
			host, url.QueryEscape(query))
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				Movies []ytsMovie `json:"movies"`
			} `json:"data"`
		}
		if err := getJSON(ctx, u, &resp); err != nil || resp.Status != "ok" {
			continue
		}
		for _, mv := range resp.Data.Movies {
			if mv.TitleLong == "" {
				continue
			}
			best := pickBestTorrent(mv.Torrents)
			if best.URL == "" {
				continue
			}
			out = append(out, model.SearchResult{
				ID:           fmt.Sprintf("yts-%d", mv.ID),
				Title:        fmt.Sprintf("%s (%d)", mv.TitleLong, mv.Year),
				Category:     model.CatTorrent,
				Source:       "yts",
				URL:          mv.URL,
				Magnet:       best.URL, // .torrent file URL, resolved by the downloader
				ThumbnailURL: mv.MediumCover,
				Size:         best.Size,
				Seeders:      best.Seed,
				Leechers:     best.Peers,
			})
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

// categorizeTorrent decides whether a torrent is a video game, using the
// indexer's own category when available (tpb category 400 / "Games", 1337x
// "Games") and falling back to well-known repack / pirate-group tags.
func categorizeTorrent(title, indexerCat string) string {
	low := strings.ToLower(strings.TrimSpace(indexerCat))
	if low == "400" || strings.Contains(low, "game") {
		return model.CatGame
	}
	low = strings.ToLower(title)
	// Only scene/pirate-group tags and unambiguous game markers — generic words
	// like "cracked", "steam" or "deluxe edition" also appear in movie/TV
	// releases and would mislabel them.
	for _, kw := range []string{
		"fitgirl", "repack", "dodi", "codex", "rune", "empress", "skidrow", "steamrip",
		" gog", "pc game", "goty", "game of the year", "dlc", "crackwatch",
	} {
		if strings.Contains(low, kw) {
			return model.CatGame
		}
	}
	return model.CatTorrent
}

// pickBestTorrent prefers 1080p, then the most seeded release.
func pickBestTorrent(ts []ytsTorrent) ytsTorrent {
	var best ytsTorrent
	for _, t := range ts {
		if t.Quality == "1080p" {
			return t
		}
	}
	for _, t := range ts {
		if t.Seed > best.Seed {
			best = t
		}
	}
	return best
}

// ---- 1337x (HTML scrape) ----

var l337Hosts = []string{"https://1337x.to", "https://1337x.gd", "https://1337x.st", "https://1337x.is"}

// Search1337x scrapes the 1337x search page and extracts magnet links from
// detail pages (bounded concurrency).
func Search1337x(ctx context.Context, query string) []model.SearchResult {
	var out []model.SearchResult
	for _, host := range l337Hosts {
		u := host + "/search/" + url.PathEscape(query) + "/1/"
		body, status, err := get(ctx, u)
		if err != nil || status != 200 {
			continue
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
		if err != nil {
			continue
		}
		type row struct {
			title, href, size, cat string
			seed                   int
		}
		var rows []row
		doc.Find("tbody tr").Each(func(_ int, s *goquery.Selection) {
			sel := s.Find(`a[href*="/torrent/"]`).First()
			title := strings.TrimSpace(sel.Text())
			href, _ := sel.Attr("href")
			if title == "" || href == "" {
				return
			}
			rows = append(rows, row{
				title: title,
				href:  href,
				size:  strings.TrimSpace(s.Find("td:nth-child(5)").Text()),
				seed:  parseInt(s.Find("td:nth-child(2)").Text()),
				cat:   strings.TrimSpace(s.Find("td:nth-child(1)").Text()),
			})
		})
		if len(rows) == 0 {
			continue
		}
		const limit = 6
		if len(rows) > limit {
			rows = rows[:limit]
		}
		sem := make(chan struct{}, 4)
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, r := range rows {
			wg.Add(1)
			go func(r row) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				magnet := fetchMagnet(ctx, host+r.href)
				if magnet == "" {
					return
				}
				mu.Lock()
				out = append(out, model.SearchResult{
					ID:       model.NewID(),
					Title:    r.title,
					Category: categorizeTorrent(r.title, r.cat),
					Source:   "1337x",
					URL:      host + r.href,
					Magnet:   magnet,
					Size:     r.size,
					Seeders:  r.seed,
				})
				mu.Unlock()
			}(r)
		}
		wg.Wait()
		if len(out) > 0 {
			break
		}
	}
	return out
}

// fetchMagnet grabs the magnet link from a 1337x detail page.
func fetchMagnet(ctx context.Context, page string) string {
	body, status, err := get(ctx, page)
	if err != nil || status != 200 {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return ""
	}
	magnet, _ := doc.Find(`a[href^="magnet:"]`).First().Attr("href")
	return magnet
}

// ---- Nyaa (HTML scrape, magnets in the table) ----

// SearchNyaa scrapes nyaa.si (anime/manga fansub index) for torrents and comics.
func SearchNyaa(ctx context.Context, query string) []model.SearchResult {
	u := "https://nyaa.si/?f=0&c=0_0&q=" + url.QueryEscape(query) + "&s=seeders&o=desc"
	body, status, err := get(ctx, u)
	if err != nil || status != 200 {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	var out []model.SearchResult
	doc.Find("tbody tr").Each(func(_ int, s *goquery.Selection) {
		magnet, ok := s.Find(`a[href^="magnet:"]`).First().Attr("href")
		if !ok {
			return
		}
		title := strings.TrimSpace(s.Find(`a[href*="/view/"]`).First().Text())
		if title == "" {
			return
		}
		cat := categorizeTorrent(title, "")
		low := strings.ToLower(title)
		if strings.Contains(low, ".pdf") || strings.Contains(low, ".epub") ||
			strings.Contains(low, ".cbz") || strings.Contains(low, ".cbr") {
			cat = model.CatPDF
		}
		out = append(out, model.SearchResult{
			ID:       model.NewID(),
			Title:    title,
			Category: cat,
			Source:   "nyaa",
			URL:      "https://nyaa.si",
			Magnet:   magnet,
			Size:     strings.TrimSpace(s.Find("td:nth-child(4)").Text()),
			Seeders:  parseInt(s.Find("td:nth-child(6)").Text()),
			Leechers: parseInt(s.Find("td:nth-child(7)").Text()),
		})
	})
	return out
}

// ---- Pirate Bay (via apibay.org JSON) ----

// SearchPirateBay queries the Pirate Bay through the apibay.org JSON gateway.
func SearchPirateBay(ctx context.Context, query string) []model.SearchResult {
	u := "https://apibay.org/q.php?q=" + url.QueryEscape(query) + "&cat=0"
	var items []struct {
		Name     string `json:"name"`
		InfoHash string `json:"info_hash"`
		Size     string `json:"size"`
		Seeders  string `json:"seeders"`
		Leechers string `json:"leechers"`
		Category string `json:"category"`
	}
	if err := getJSON(ctx, u, &items); err != nil {
		return nil
	}
	var out []model.SearchResult
	for _, it := range items {
		if it.InfoHash == "" || it.Name == "" {
			continue
		}
		magnet := model.MagnetFromHash(it.InfoHash, it.Name)
		if magnet == "" {
			continue
		}
		sizeBytes := int64(parseInt(it.Size))
		out = append(out, model.SearchResult{
			ID:        "tpb-" + it.InfoHash[:8],
			Title:     it.Name,
			Category:  categorizeTorrent(it.Name, it.Category),
			Source:    "tpb",
			URL:       "https://thepiratebay.org/search/" + url.QueryEscape(query),
			Magnet:    magnet,
			Size:      model.FormatBytes(sizeBytes),
			SizeBytes: sizeBytes,
			Seeders:   parseInt(it.Seeders),
			Leechers:  parseInt(it.Leechers),
		})
	}
	return out
}

package scraper

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"cerebro/internal/model"

	"github.com/PuerkitoBio/goquery"
)

var fgSizeRe = regexp.MustCompile(`(?i)\d+(?:\.\d+)?\s*(gb|mb)`)

// SearchFitGirl finds game repacks from fitgirl-repacks.site (goquery parse).
// The site is frequently behind Cloudflare, so failures degrade to zero hits
// like every other engine.
func SearchFitGirl(ctx context.Context, query string) []model.SearchResult {
	var out []model.SearchResult
	u := "https://fitgirl-repacks.site/?s=" + url.QueryEscape(query)
	body, status, err := get(ctx, u)
	if err != nil || status != 200 {
		return out
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return out
	}
	doc.Find("article .entry-title a, article h2 a").Each(func(i int, s *goquery.Selection) {
		if len(out) >= 10 {
			return
		}
		title := strings.TrimSpace(s.Text())
		href, _ := s.Attr("href")
		if title == "" || href == "" {
			return
		}
		size, magnet, thumb := "", "", ""
		if article := s.ParentsFiltered("article").First(); article.Length() > 0 {
			content := article.Find(".entry-content").Text()
			if m := fgSizeRe.FindString(content); m != "" {
				size = strings.ToUpper(m)
			}
			article.Find("a[href^='magnet:']").Each(func(_ int, a *goquery.Selection) {
				if magnet == "" {
					magnet, _ = a.Attr("href")
				}
			})
			if src, ok := article.Find(".entry-content img").First().Attr("src"); ok && thumb == "" {
				thumb = src
			}
		}
		out = append(out, model.SearchResult{
			ID:           "fg-" + href,
			Title:        title,
			Category:     model.CatGame,
			Source:       "fitgirl",
			URL:          href,
			Magnet:       magnet,
			ThumbnailURL: thumb,
			Size:         size,
		})
	})
	return out
}

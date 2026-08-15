package scraper

import (
	"context"
	"fmt"
	"net/url"

	"cerebro/internal/model"
)

// ghRepo mirrors the fields we need from the GitHub search API.
type ghRepo struct {
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Size          int64  `json:"size"` // KB
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type ghSearchResponse struct {
	Items []ghRepo `json:"items"`
}

// SearchGitHub finds software, dev tools and apps via the GitHub repository
// search API (no token needed for 10 requests/min). Each hit downloads as a
// zipball of the repo's default branch — a direct link the 16-worker chunked
// downloader handles natively.
func SearchGitHub(ctx context.Context, query string) []model.SearchResult {
	var out []model.SearchResult
	u := "https://api.github.com/search/repositories?q=" + url.QueryEscape(query) + "&per_page=12"
	var resp ghSearchResponse
	if err := getJSON(ctx, u, &resp); err != nil {
		return out
	}
	for _, r := range resp.Items {
		if r.FullName == "" || r.DefaultBranch == "" {
			continue
		}
		zipURL := fmt.Sprintf("https://github.com/%s/archive/refs/heads/%s.zip", r.FullName, url.PathEscape(r.DefaultBranch))
		out = append(out, model.SearchResult{
			ID:           "gh-" + r.FullName,
			Title:        r.FullName,
			Category:     model.CatSoftware,
			Source:       "github",
			Author:       r.Owner.Login,
			Ext:          "zip",
			URL:          zipURL,
			ThumbnailURL: "https://github.com/" + url.PathEscape(r.Owner.Login) + ".png",
			Size:         model.FormatBytes(r.Size * 1024), // repo size is reported in KB
			SizeBytes:    r.Size * 1024,
		})
	}
	return out
}

package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"cerebro/internal/model"
)

// SearchYouTube searches YouTube by parsing the ytInitialData JSON embedded in
// the results page (no API key required).
func SearchYouTube(ctx context.Context, query string) []model.SearchResult {
	u := "https://www.youtube.com/results?search_query=" + url.QueryEscape(query)
	body, status, err := get(ctx, u)
	if err != nil || status != 200 {
		debugf("youtube search: status %d err %v", status, err)
		return nil
	}
	data := extractInitialData(body)
	if data == nil {
		debugf("youtube search: no ytInitialData found")
		return nil
	}
	var out []model.SearchResult
	walkJSON(data, func(m map[string]any) {
		vr, ok := m["videoRenderer"].(map[string]any)
		if !ok {
			return
		}
		id, _ := vr["videoId"].(string)
		if id == "" {
			return
		}
		title := runsText(vr["title"])
		if title == "" {
			return
		}
		out = append(out, model.SearchResult{
			ID:           "yt-" + id,
			Title:        title,
			Category:     model.CatYouTube,
			Source:       "youtube",
			VideoID:      id,
			Author:       runsText(vr["ownerText"]),
			URL:          "https://www.youtube.com/watch?v=" + id,
			ThumbnailURL: "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg",
			Size:         simpleText(vr["lengthText"]),
		})
	})
	return out
}

// extractInitialData pulls the ytInitialData JSON blob out of the results HTML.
func extractInitialData(body []byte) any {
	const marker = "var ytInitialData = "
	i := bytes.Index(body, []byte(marker))
	if i < 0 {
		return nil
	}
	rest := body[i+len(marker):]
	if j := bytes.Index(rest, []byte(";</script>")); j >= 0 {
		rest = rest[:j]
	}
	var v any
	if err := json.Unmarshal(rest, &v); err != nil {
		return nil
	}
	return v
}

// walkJSON depth-first visits every map in a decoded JSON tree.
func walkJSON(v any, visit func(map[string]any)) {
	switch t := v.(type) {
	case map[string]any:
		visit(t)
		for _, val := range t {
			walkJSON(val, visit)
		}
	case []any:
		for _, val := range t {
			walkJSON(val, visit)
		}
	}
}

// runsText joins YouTube's {runs:[{text}]} rich text nodes.
func runsText(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	runs, ok := m["runs"].([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, r := range runs {
		if rm, ok := r.(map[string]any); ok {
			if s, ok := rm["text"].(string); ok {
				b.WriteString(s)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// simpleText extracts {simpleText} or {runs} text from a YouTube text node.
func simpleText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return runsText(v)
}

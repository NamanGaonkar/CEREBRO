package model

import (
	"net/url"
	"strings"
)

// DefaultTrackers is a curated list of high-speed public trackers appended
// to every magnet link for faster peer discovery.
var DefaultTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://open.demonii.com:1337/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"udp://exodus.desync.com:6969/announce",
	"udp://tracker.torrent.eu.org:451/announce",
	"udp://explodie.org:6969/announce",
	"udp://tracker.moeking.me:6969/announce",
	"udp://open.stealth.si:80/announce",
	"udp://tracker.dler.org:6969/announce",
	"udp://tracker.leechers-paradise.org:6969/announce",
	"https://tracker.tamersunion.org:443/announce",
	"wss://tracker.btorrent.xyz",
	"wss://tracker.openwebtorrent.com",
}

// EnhanceMagnet appends the curated tracker list to a magnet link.
// Existing trackers are preserved and the operation is idempotent.
func EnhanceMagnet(magnet string) string {
	if !strings.HasPrefix(magnet, "magnet:") {
		return magnet
	}
	parts := strings.Split(magnet, "&")
	if len(parts) == 0 {
		return magnet
	}
	seen := make(map[string]bool, len(parts))
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "tr=") {
			if tr, err := url.QueryUnescape(strings.TrimPrefix(p, "tr=")); err == nil {
				seen[tr] = true
			}
		}
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, tr := range DefaultTrackers {
		if !seen[tr] {
			b.WriteString("&tr=" + url.QueryEscape(tr))
		}
	}
	for _, p := range parts[1:] {
		b.WriteString("&" + p)
	}
	return b.String()
}

// MagnetFromHash builds an enhanced magnet link from a raw info-hash hex
// string and an optional display name.
func MagnetFromHash(infoHash, name string) string {
	infoHash = strings.TrimSpace(strings.ToLower(infoHash))
	if len(infoHash) != 40 {
		return ""
	}
	m := "magnet:?xt=urn:btih:" + infoHash
	if name != "" {
		m += "&dn=" + url.QueryEscape(name)
	}
	return EnhanceMagnet(m)
}

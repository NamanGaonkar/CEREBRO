// Package doh provides an HTTP transport that resolves DNS names over HTTPS
// (Cloudflare 1.1.1.1 with Quad9 9.9.9.9 fallback) instead of the ISP
// resolver. Scraper and downloader requests routed through it bypass
// ISP-level tracker blocks without a VPN. Every failure degrades silently to
// the system resolver, so connectivity is never lost.
package doh

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// endpoints are the DoH JSON endpoints tried in order (first success wins).
var endpoints = []string{
	"https://cloudflare-dns.com/dns-query?name=%s&type=A",
	"https://dns.quad9.net/dns-query?name=%s&type=A",
}

type cacheEntry struct {
	ips     []string
	expires time.Time
}

// resolver queries DoH endpoints and caches A records briefly.
type resolver struct {
	mu     sync.Mutex
	cache  map[string]cacheEntry
	client *http.Client // plain client used to reach the DoH endpoints
}

func newResolver() *resolver {
	return &resolver{
		cache:  make(map[string]cacheEntry),
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// lookup resolves host via DoH. On total failure it returns the last error so
// the caller can fall back to the system resolver.
func (r *resolver) lookup(ctx context.Context, host string) ([]string, error) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	r.mu.Lock()
	if e, ok := r.cache[host]; ok && time.Now().Before(e.expires) {
		ips := e.ips
		r.mu.Unlock()
		return ips, nil
	}
	r.mu.Unlock()

	var ips []string
	var lastErr error
	for _, tmpl := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(tmpl, host), nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/dns-json")
		req.Header.Set("User-Agent", "cerebro-max/2.0")
		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		ips, err = parseResponse(resp)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if len(ips) > 0 {
			break
		}
	}
	if len(ips) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("doh: no A records for %s", host)
	}

	r.mu.Lock()
	r.cache[host] = cacheEntry{ips: ips, expires: time.Now().Add(5 * time.Minute)}
	r.mu.Unlock()
	return ips, nil
}

type dnsAnswer struct {
	Type int    `json:"type"`
	Data string `json:"data"`
}

type dnsResponse struct {
	Status int         `json:"status"`
	Answer []dnsAnswer `json:"answer"`
}

// parseResponse decodes a DoH JSON response into A-record IPs.
func parseResponse(resp *http.Response) ([]string, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh: HTTP %d", resp.StatusCode)
	}
	var d dnsResponse
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	if d.Status != 0 {
		return nil, fmt.Errorf("doh: dns status %d", d.Status)
	}
	var ips []string
	for _, a := range d.Answer {
		if a.Type == 1 && net.ParseIP(a.Data) != nil {
			ips = append(ips, a.Data)
		}
	}
	return ips, nil
}

// NewTransport builds an http.Transport whose dialer resolves hostnames via
// DoH, falling back to the system resolver on any DoH failure.
func NewTransport() *http.Transport {
	r := newResolver()
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil || net.ParseIP(host) != nil {
				// Already an IP (or malformed): dial directly.
				return d.DialContext(ctx, network, addr)
			}
			ips, err := r.lookup(ctx, host)
			if err != nil {
				// DoH unreachable (offline / blocked): keep connectivity by
				// using the system resolver.
				return d.DialContext(ctx, network, addr)
			}
			var lastErr error
			for _, ip := range ips {
				conn, derr := d.DialContext(ctx, network, net.JoinHostPort(ip, port))
				if derr == nil {
					return conn, nil
				}
				lastErr = derr
			}
			return nil, lastErr
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NewClient returns an http.Client using the DoH transport with the given
// overall timeout.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Transport: NewTransport(), Timeout: timeout}
}

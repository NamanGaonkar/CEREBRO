package doh

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseResponse(t *testing.T) {
	body := `{"Status":0,"Answer":[{"name":"example.com.","type":1,"TTL":300,"data":"93.184.216.34"},{"name":"example.com.","type":28,"TTL":300,"data":"2606:2800:220:1:248:1893:25c8:1946"},{"name":"example.com.","type":1,"TTL":300,"data":"93.184.216.35"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	ips, err := parseResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if len(ips) != 2 || ips[0] != "93.184.216.34" || ips[1] != "93.184.216.35" {
		t.Errorf("expected 2 IPv4 answers, got %v", ips)
	}
}

func TestParseResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"Status":3,"Answer":[]}`) // NXDOMAIN
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseResponse(resp); err == nil {
		t.Error("expected error for non-zero dns status")
	}
	resp.Body.Close()
}

func TestLookupEndpointsAndFallback(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"Status":0,"Answer":[{"name":"x.","type":1,"TTL":300,"data":"1.2.3.4"}]}`)
	}))
	defer good.Close()

	orig := endpoints
	defer func() { endpoints = orig }()

	// First endpoint fails, second succeeds -> fallback must work.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer dead.Close()
	endpoints = []string{dead.URL + "/?name=%s&type=A", good.URL + "/?name=%s&type=A"}

	r := newResolver()
	ips, err := r.lookup(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("lookup with fallback: %v", err)
	}
	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Errorf("expected fallback IP, got %v", ips)
	}

	// Cache hit: query count should not grow on a second lookup.
	queries := 0
	counter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries++
		fmt.Fprint(w, `{"Status":0,"Answer":[{"name":"x.","type":1,"TTL":300,"data":"5.6.7.8"}]}`)
	}))
	defer counter.Close()
	endpoints = []string{counter.URL + "/?name=%s&type=A"}
	r2 := newResolver()
	if _, err := r2.lookup(context.Background(), "cached.example"); err != nil {
		t.Fatal(err)
	}
	if _, err := r2.lookup(context.Background(), "cached.example"); err != nil {
		t.Fatal(err)
	}
	if queries != 1 {
		t.Errorf("expected 1 DoH query with caching, got %d", queries)
	}
}

func TestTransportFallsBackOnDoHFailure(t *testing.T) {
	orig := endpoints
	defer func() { endpoints = orig }()
	endpoints = []string{"http://127.0.0.1:1/?name=%s&type=A"} // unreachable

	// The DoH resolver fails (endpoint is dead), but the transport must still
	// resolve "localhost" via the system resolver and reach the server.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer target.Close()

	tr := NewTransport()
	client := &http.Client{Transport: tr, Timeout: 15 * time.Second}
	resp, err := client.Get(target.URL) // host is localhost — not an IP literal
	if err != nil {
		t.Fatalf("transport with dead DoH endpoint should fall back to system DNS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected status %s", resp.Status)
	}
}

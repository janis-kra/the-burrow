package immo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func serveFixture(t *testing.T, name string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("User-Agent"), "Mozilla") {
			t.Errorf("expected browser-like User-Agent, got %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}))
}

func TestWeserlandScrape(t *testing.T) {
	server := serveFixture(t, "weserland.html")
	defer server.Close()

	s := NewWeserland(http.DefaultClient, server.URL+"/immobilien/")
	listings, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(listings) != 21 {
		t.Fatalf("expected 21 listings, got %d", len(listings))
	}

	first := listings[0]
	if first.Title != "Ihr Rückzugsort: Kleines Haus mit Wohlfühlfaktor" {
		t.Errorf("unexpected first title: %q", first.Title)
	}
	if first.URL != "https://weserland-immobilien.de/objekte/ihr-rueckzugsort-kleines-haus-mit-wohlfuehlfaktor/" {
		t.Errorf("unexpected first URL: %q", first.URL)
	}
	if first.PriceRaw != "60.000€" {
		t.Errorf("unexpected first price raw: %q", first.PriceRaw)
	}
	if first.PriceCents != 6000000 {
		t.Errorf("unexpected first price cents: %d", first.PriceCents)
	}
	if first.Source != "Weserland Immobilien" {
		t.Errorf("unexpected source: %q", first.Source)
	}

	for _, l := range listings {
		if !strings.HasPrefix(l.URL, "http") {
			t.Errorf("URL not absolute: %q", l.URL)
		}
		if l.Title == "" {
			t.Error("listing with empty title")
		}
	}

	// HTML entities in titles must be decoded.
	for _, l := range listings {
		if strings.Contains(l.Title, "&amp;") {
			t.Errorf("title contains undecoded entity: %q", l.Title)
		}
	}
}

func TestWeserlandScrapeHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	s := NewWeserland(http.DefaultClient, server.URL)
	if _, err := s.Scrape(context.Background()); err == nil {
		t.Error("expected error on HTTP 503")
	}
}

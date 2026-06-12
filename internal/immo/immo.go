// Package immo scrapes real-estate broker listing pages, filters them by
// search criteria, and diffs them against a persisted state so that only
// new listings and price drops trigger an email.
package immo

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

type Listing struct {
	Source     string   // e.g. "Hapke Immobilien"
	Title      string
	URL        string   // absolute; dedup key
	PriceRaw   string   // "688.000 €" or "auf Anfrage" (rendered as-is)
	PriceCents int64    // 0 = unknown → no price filter, no drop tracking
	Location   string   // "" if the site does not show it
	Type       string   // property type, "" if unknown
	Details    []string // "Wohnfläche: 150 m²", "Objektzustand: gepflegt", ...
}

type Scraper interface {
	Name() string
	Scrape(ctx context.Context) ([]Listing, error)
}

// ScrapeAll runs all scrapers sequentially. Per-site errors are logged, not
// fatal — one broken broker page must not take down the whole run.
func ScrapeAll(ctx context.Context, scrapers []Scraper) []Listing {
	var all []Listing
	for _, s := range scrapers {
		listings, err := s.Scrape(ctx)
		if err != nil {
			log.Printf("immo: scraping %s failed: %v", s.Name(), err)
			continue
		}
		if len(listings) == 0 {
			log.Printf("immo: %s returned 0 listings — markup may have changed", s.Name())
			continue
		}
		all = append(all, listings...)
	}
	return all
}

// Small brokers sometimes run naive bot blockers, so requests carry a
// browser-like User-Agent.
const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"

func fetchDocument(ctx context.Context, client *http.Client, rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: status %d", rawURL, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", rawURL, err)
	}
	return doc, nil
}

func resolveURL(base *url.URL, href string) string {
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

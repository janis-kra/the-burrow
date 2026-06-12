package immo

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Weserland scrapes https://weserland-immobilien.de/immobilien/ — a Divi
// blog grid where each listing is an article.type-objekte with title+link in
// h2.entry-title and the price as plain text in .post-content-inner. The
// overview shows only title and price, no location or details.
type Weserland struct {
	client *http.Client
	url    string
}

func NewWeserland(client *http.Client, rawURL string) *Weserland {
	return &Weserland{client: client, url: rawURL}
}

func (w *Weserland) Name() string { return "Weserland Immobilien" }

func (w *Weserland) Scrape(ctx context.Context) ([]Listing, error) {
	doc, err := fetchDocument(ctx, w.client, w.url)
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(w.url)
	if err != nil {
		return nil, err
	}

	var listings []Listing
	doc.Find("article.type-objekte").Each(func(_ int, s *goquery.Selection) {
		a := s.Find("h2.entry-title a").First()
		title := strings.TrimSpace(a.Text())
		href, _ := a.Attr("href")
		if title == "" || href == "" {
			return
		}

		priceRaw := strings.TrimSpace(s.Find(".post-content-inner").First().Text())
		cents, _ := ParsePrice(priceRaw)

		listings = append(listings, Listing{
			Source:     w.Name(),
			Title:      title,
			URL:        resolveURL(base, href),
			PriceRaw:   priceRaw,
			PriceCents: cents,
		})
	})

	return listings, nil
}

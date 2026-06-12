package immo

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Hapke scrapes https://www.hapke-immobilien.de/immobilien-hameln-kaufen — a
// UIkit page where each listing is an a.uk-link-reset wrapping a card with
// h3 (title), .uk-text-meta (location like "31785 Hameln") and key/value
// rows (.uk-flex.uk-flex-between) for Objekttyp, Fläche, Objektzustand and
// Kaufpreis. Every listing appears twice (list and grid layout variant), so
// results are deduped by URL.
type Hapke struct {
	client *http.Client
	url    string
}

func NewHapke(client *http.Client, rawURL string) *Hapke {
	return &Hapke{client: client, url: rawURL}
}

func (h *Hapke) Name() string { return "Hapke Immobilien" }

func (h *Hapke) Scrape(ctx context.Context) ([]Listing, error) {
	doc, err := fetchDocument(ctx, h.client, h.url)
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(h.url)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var listings []Listing
	doc.Find("a.uk-link-reset[href]").Each(func(_ int, a *goquery.Selection) {
		title := strings.TrimSpace(a.Find("h3").First().Text())
		href, _ := a.Attr("href")
		if title == "" || href == "" || strings.HasPrefix(href, "#") {
			return
		}

		// uk-link-reset is also used for nav anchors (tel:, mailto:) —
		// only http(s) links can be listings.
		abs := resolveURL(base, href)
		if u, err := url.Parse(abs); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return
		}
		if seen[abs] {
			return
		}
		seen[abs] = true

		l := Listing{
			Source:   h.Name(),
			Title:    title,
			URL:      abs,
			Location: strings.TrimSpace(a.Find(".uk-text-meta").First().Text()),
		}

		a.Find(".uk-flex.uk-flex-between").Each(func(_ int, row *goquery.Selection) {
			kids := row.Children()
			if kids.Length() != 2 {
				return
			}
			key := strings.TrimSpace(kids.Eq(0).Text())
			val := strings.TrimSpace(kids.Eq(1).Text())
			// Skip layout rows (e.g. the "Details" button footer) and rows
			// without a usable key/value pair.
			if key == "" || val == "" || val == "Details" {
				return
			}
			key = strings.TrimSuffix(key, ":")
			switch {
			case strings.HasPrefix(key, "Objekttyp"):
				l.Type = val
			case strings.HasPrefix(key, "Kaufpreis"):
				l.PriceRaw = val
				l.PriceCents, _ = ParsePrice(val)
			default:
				l.Details = append(l.Details, key+": "+val)
			}
		})

		listings = append(listings, l)
	})

	return listings, nil
}

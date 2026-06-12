package immo

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestHapkeScrape(t *testing.T) {
	server := serveFixture(t, "hapke.html")
	defer server.Close()

	s := NewHapke(http.DefaultClient, server.URL+"/immobilien-hameln-kaufen")
	listings, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 10 unique listings; each appears twice in the markup (list + grid
	// layout) and must be deduped. Nav anchors (#, tel:, mailto:) and the
	// "Fragen zu einem Objekt?" teasers must not show up.
	if len(listings) != 10 {
		t.Fatalf("expected 10 listings, got %d", len(listings))
	}

	first := listings[0]
	if first.Title != "Mehrfamilienhaus mit 5 Wohneinheiten in Hamelns Südstadt" {
		t.Errorf("unexpected first title: %q", first.Title)
	}
	if first.URL != "https://www.hapke-immobilien.de/immobilie-hameln-kaufen/3053094" {
		t.Errorf("unexpected first URL: %q", first.URL)
	}
	if first.Location != "31785 Hameln" {
		t.Errorf("unexpected location: %q", first.Location)
	}
	if first.Type != "Mehrfamilienhaus" {
		t.Errorf("unexpected type: %q", first.Type)
	}
	if first.PriceRaw != "319.000 €" {
		t.Errorf("unexpected price raw: %q", first.PriceRaw)
	}
	if first.PriceCents != 31900000 {
		t.Errorf("unexpected price cents: %d", first.PriceCents)
	}
	if first.Source != "Hapke Immobilien" {
		t.Errorf("unexpected source: %q", first.Source)
	}

	wantDetails := map[string]bool{}
	for _, d := range first.Details {
		wantDetails[d] = true
	}
	if !wantDetails["Vermietbare Fläche: 315 m²"] {
		t.Errorf("missing Fläche detail, got %v", first.Details)
	}
	if !wantDetails["Objektzustand: geflegt"] {
		t.Errorf("missing Objektzustand detail, got %v", first.Details)
	}

	seen := map[string]bool{}
	for _, l := range listings {
		if seen[l.URL] {
			t.Errorf("duplicate listing URL: %q", l.URL)
		}
		seen[l.URL] = true
		if !strings.HasPrefix(l.URL, "https://www.hapke-immobilien.de/immobilie-hameln-kaufen/") {
			t.Errorf("unexpected listing URL: %q", l.URL)
		}
		if l.Title == "Fragen zu einem Objekt?" {
			t.Errorf("nav teaser scraped as listing: %+v", l)
		}
	}
}

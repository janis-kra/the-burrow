package immo

import (
	"os"
	"strings"
	"testing"
)

func TestRenderEmail(t *testing.T) {
	htmlTpl, err := os.ReadFile("../../templates/immo.html")
	if err != nil {
		t.Fatalf("reading HTML template: %v", err)
	}
	textTpl, err := os.ReadFile("../../templates/immo.txt")
	if err != nil {
		t.Fatalf("reading text template: %v", err)
	}

	data := EmailData{
		Date: "Freitag, 12. Juni 2026",
		PriceDrops: []PriceDrop{
			{
				Listing: Listing{
					Source: "Hapke Immobilien", Title: "Einfamilienhaus in Hameln",
					URL: "https://example.com/haus-1", PriceRaw: "319.000 €", PriceCents: 31900000,
					Location: "31785 Hameln", Type: "Einfamilienhaus", Details: []string{"Wohnfläche: 150 m²"},
				},
				OldPriceRaw: "349.000 €",
			},
		},
		New: []Listing{
			{
				Source: "Weserland Immobilien", Title: "Kleines Haus mit Wohlfühlfaktor",
				URL: "https://example.com/haus-2", PriceRaw: "60.000€", PriceCents: 6000000,
			},
			{
				Source: "Weserland Immobilien", Title: "Haus ohne Preis",
				URL: "https://example.com/haus-3", PriceRaw: "", PriceCents: 0,
			},
		},
	}

	html, text, err := RenderEmail(string(htmlTpl), string(textTpl), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{
		"Preissenkungen", "349.000 €", "319.000 €",
		"Neue Listings", "Kleines Haus mit Wohlfühlfaktor", "60.000€",
		"Preis auf Anfrage",
		"via Hapke Immobilien", "via Weserland Immobilien",
		"https://example.com/haus-1", "https://example.com/haus-2",
		"31785 Hameln · Einfamilienhaus · Wohnfläche: 150 m²",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}

	for _, want := range []string{
		"PREISSENKUNGEN", "349.000 € -> 319.000 €",
		"NEUE LISTINGS", "Kleines Haus mit Wohlfühlfaktor",
		"https://example.com/haus-2",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q", want)
		}
	}
}

func TestRenderEmailOnlyNew(t *testing.T) {
	htmlTpl, _ := os.ReadFile("../../templates/immo.html")
	textTpl, _ := os.ReadFile("../../templates/immo.txt")

	html, _, err := RenderEmail(string(htmlTpl), string(textTpl), EmailData{
		Date: "heute",
		New:  []Listing{{Source: "X", Title: "Haus", URL: "https://example.com", PriceRaw: "1 €"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(html, "Preissenkungen") {
		t.Error("price drop section rendered despite no drops")
	}
}

func TestSubject(t *testing.T) {
	l := Listing{}
	d := PriceDrop{}
	tests := []struct {
		diff DiffResult
		want string
	}{
		{DiffResult{New: []Listing{l, l, l}}, "Burrow Immobilien — 3 neue Treffer"},
		{DiffResult{New: []Listing{l}}, "Burrow Immobilien — 1 neuer Treffer"},
		{DiffResult{PriceDrops: []PriceDrop{d}}, "Burrow Immobilien — 1 Preissenkung"},
		{DiffResult{New: []Listing{l, l}, PriceDrops: []PriceDrop{d, d}}, "Burrow Immobilien — 2 neue Treffer, 2 Preissenkungen"},
	}
	for _, tt := range tests {
		if got := Subject(tt.diff); got != tt.want {
			t.Errorf("Subject(%+v) = %q, want %q", tt.diff, got, tt.want)
		}
	}
}

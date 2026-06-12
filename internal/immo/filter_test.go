package immo

import "testing"

func TestCriteriaMatch(t *testing.T) {
	criteria := Criteria{
		MinPriceCents: 10000000, // 100.000 €
		MaxPriceCents: 45000000, // 450.000 €
		Locations:     []string{"Hameln", "Aerzen"},
	}

	tests := []struct {
		name    string
		listing Listing
		want    bool
	}{
		{
			name:    "match within price range and location",
			listing: Listing{Title: "Einfamilienhaus", PriceCents: 31900000, Location: "31785 Hameln"},
			want:    true,
		},
		{
			name:    "price too high",
			listing: Listing{Title: "Villa", PriceCents: 68800000, Location: "Hameln"},
			want:    false,
		},
		{
			name:    "price too low",
			listing: Listing{Title: "Garage", PriceCents: 1500000, Location: "Hameln"},
			want:    false,
		},
		{
			name:    "unknown price passes (fail-open)",
			listing: Listing{Title: "Haus", PriceRaw: "auf Anfrage", PriceCents: 0, Location: "Hameln"},
			want:    true,
		},
		{
			name:    "missing location passes (fail-open)",
			listing: Listing{Title: "Haus", PriceCents: 20000000, Location: ""},
			want:    true,
		},
		{
			name:    "wrong location rejected",
			listing: Listing{Title: "Haus", PriceCents: 20000000, Location: "30159 Hannover"},
			want:    false,
		},
		{
			name:    "location match is case-insensitive substring",
			listing: Listing{Title: "Haus", PriceCents: 20000000, Location: "31855 AERZEN OT Groß Berkel"},
			want:    true,
		},
		{
			name:    "apartment in title rejected",
			listing: Listing{Title: "Schöne Eigentumswohnung in Hameln", PriceCents: 20000000, Location: "Hameln"},
			want:    false,
		},
		{
			name:    "apartment in type rejected",
			listing: Listing{Title: "Gemütliches Zuhause", Type: "Dachgeschosswohnung", PriceCents: 20000000, Location: "Hameln"},
			want:    false,
		},
		{
			name:    "rental rejected",
			listing: Listing{Title: "Haus zu vermieten", PriceCents: 20000000, Location: "Hameln"},
			want:    false,
		},
		{
			name:    "monthly rent signal rejected",
			listing: Listing{Title: "Haus — 1.200 € / Monat", PriceCents: 120000, Location: "Hameln"},
			want:    false,
		},
		{
			name:    "plot passes (fail-open, not filtered for now)",
			listing: Listing{Title: "Baugrundstück in ruhiger Lage", Type: "Grundstück", PriceCents: 12000000, Location: "Hameln"},
			want:    true,
		},
		{
			name:    "ETW abbreviation rejected",
			listing: Listing{Title: "3 Zi.- ETW Hameln Ostermeyerstr. mit Fahrstuhl", PriceCents: 15500000},
			want:    false,
		},
		{
			name:    "ETW substring inside word not rejected",
			listing: Listing{Title: "Haus mit Glasfasernetzwerk", PriceCents: 20000000, Location: "Hameln"},
			want:    true,
		},
		{
			name:    "rental price label rejected",
			listing: Listing{Title: "Attraktive Gewerbefläche im Postgebäude", PriceRaw: "Mietpreis auf Anfrage", PriceCents: 0},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := criteria.Match(tt.listing); got != tt.want {
				t.Errorf("Match(%+v) = %v, want %v", tt.listing, got, tt.want)
			}
		})
	}
}

func TestCriteriaMatchNoBounds(t *testing.T) {
	c := Criteria{}
	if !c.Match(Listing{Title: "Haus", PriceCents: 99900000, Location: "Berlin"}) {
		t.Error("empty criteria should match any non-rejected listing")
	}
}

func TestFilter(t *testing.T) {
	listings := []Listing{
		{Title: "Haus A", PriceCents: 20000000},
		{Title: "Wohnung B", PriceCents: 20000000},
		{Title: "Haus C", PriceCents: 99900000},
	}
	got := Filter(listings, Criteria{MaxPriceCents: 45000000})
	if len(got) != 1 || got[0].Title != "Haus A" {
		t.Errorf("Filter returned %+v, want only 'Haus A'", got)
	}
}

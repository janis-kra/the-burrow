package immo

import (
	"regexp"
	"strings"
)

type Criteria struct {
	MinPriceCents int64    // 0 = no lower bound
	MaxPriceCents int64    // 0 = no upper bound
	Locations     []string // case-insensitive substring match; empty = all
}

// Unambiguous reject signals in type+title+price (lowercase). Apartments
// and rentals are never wanted; everything else passes fail-open (plots and
// commercial property are not filtered for now).
var rejectSignals = []string{"wohnung", "miete", "mietpreis", "zu vermieten", "pacht", "/ monat"}

// "ETW" (Eigentumswohnung) needs word boundaries — a plain substring would
// also hit words like "Netzwerk".
var rejectWordRe = regexp.MustCompile(`(?i)\betw\b`)

// Match decides whether a listing passes the criteria. Filtering is
// fail-open: missing attributes (unknown price, no location shown) never
// cause a reject — only unambiguous non-matches are dropped.
func (c Criteria) Match(l Listing) bool {
	// PriceRaw is included so rentals showing "Mietpreis auf Anfrage" are
	// caught even when the title gives no signal.
	haystack := strings.ToLower(l.Type + " " + l.Title + " " + l.PriceRaw)
	for _, sig := range rejectSignals {
		if strings.Contains(haystack, sig) {
			return false
		}
	}
	if rejectWordRe.MatchString(l.Type + " " + l.Title) {
		return false
	}

	if l.PriceCents > 0 {
		if c.MinPriceCents > 0 && l.PriceCents < c.MinPriceCents {
			return false
		}
		if c.MaxPriceCents > 0 && l.PriceCents > c.MaxPriceCents {
			return false
		}
	}

	if len(c.Locations) > 0 && l.Location != "" {
		loc := strings.ToLower(l.Location)
		found := false
		for _, want := range c.Locations {
			if strings.Contains(loc, strings.ToLower(want)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// Filter returns the listings that match the criteria.
func Filter(listings []Listing, c Criteria) []Listing {
	var out []Listing
	for _, l := range listings {
		if c.Match(l) {
			out = append(out, l)
		}
	}
	return out
}

package immo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// pruneAfter is how long a listing may stay unseen before its record is
// dropped on Save. Pruning is age-based only — a listing merely missing from
// one (possibly flaky) scrape never causes re-notification.
const pruneAfter = 90 * 24 * time.Hour

type StateRecord struct {
	Title          string    `json:"title"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	LastPriceCents int64     `json:"last_price_cents"`
	LastPriceRaw   string    `json:"last_price_raw"`
	NotifiedAt     time.Time `json:"notified_at"`
}

type State struct {
	Version  int                     `json:"version"`
	Listings map[string]*StateRecord `json:"listings"`
}

type PriceDrop struct {
	Listing
	OldPriceRaw string
}

type DiffResult struct {
	New        []Listing
	PriceDrops []PriceDrop
}

func (d DiffResult) Empty() bool {
	return len(d.New) == 0 && len(d.PriceDrops) == 0
}

// LoadState reads the state file. A missing file yields an empty state, not
// an error (first run).
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{Version: 1, Listings: map[string]*StateRecord{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	if s.Listings == nil {
		s.Listings = map[string]*StateRecord{}
	}
	return &s, nil
}

// Save writes the state atomically (temp file + rename), pruning records
// not seen for pruneAfter.
func (s *State) Save(path string) error {
	s.prune(time.Now())

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".immo-state-*.json")
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("writing temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("closing temp state file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("renaming state file: %w", err)
	}
	return nil
}

func (s *State) prune(now time.Time) {
	for url, rec := range s.Listings {
		if now.Sub(rec.LastSeen) > pruneAfter {
			delete(s.Listings, url)
		}
	}
}

// Diff compares the scraped listings against the state and updates it:
// unknown URL → New; known URL with both prices > 0 and a lower new price →
// PriceDrops. last_price is always updated after the drop check, last_seen
// on every run. The caller persists the state via Save — only after a
// successful send, so a failed send retries on the next cron run.
func (s *State) Diff(listings []Listing, now time.Time) DiffResult {
	var res DiffResult
	for _, l := range listings {
		rec, ok := s.Listings[l.URL]
		if !ok {
			s.Listings[l.URL] = &StateRecord{
				Title:          l.Title,
				FirstSeen:      now,
				LastSeen:       now,
				LastPriceCents: l.PriceCents,
				LastPriceRaw:   l.PriceRaw,
				NotifiedAt:     now,
			}
			res.New = append(res.New, l)
			continue
		}

		rec.LastSeen = now
		rec.Title = l.Title
		if l.PriceCents > 0 && rec.LastPriceCents > 0 && l.PriceCents < rec.LastPriceCents {
			res.PriceDrops = append(res.PriceDrops, PriceDrop{Listing: l, OldPriceRaw: rec.LastPriceRaw})
			rec.NotifiedAt = now
		}
		rec.LastPriceCents = l.PriceCents
		rec.LastPriceRaw = l.PriceRaw
	}
	return res
}

package immo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadStateMissingFile(t *testing.T) {
	s, err := LoadState(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Listings == nil || len(s.Listings) != 0 {
		t.Errorf("expected empty state, got %+v", s)
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Now().UTC().Truncate(time.Second)

	s := &State{Version: 1, Listings: map[string]*StateRecord{
		"https://example.com/haus-1": {
			Title:          "Haus 1",
			FirstSeen:      now,
			LastSeen:       now,
			LastPriceCents: 68800000,
			LastPriceRaw:   "688.000 €",
			NotifiedAt:     now,
		},
	}}
	if err := s.Save(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	rec := loaded.Listings["https://example.com/haus-1"]
	if rec == nil {
		t.Fatal("record missing after roundtrip")
	}
	if rec.LastPriceCents != 68800000 || rec.LastPriceRaw != "688.000 €" || rec.Title != "Haus 1" {
		t.Errorf("record corrupted: %+v", rec)
	}
	if !rec.FirstSeen.Equal(now) || !rec.LastSeen.Equal(now) {
		t.Errorf("timestamps corrupted: %+v", rec)
	}
}

func TestSavePrunesOldRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Now()

	s := &State{Version: 1, Listings: map[string]*StateRecord{
		"https://example.com/old":    {Title: "Old", LastSeen: now.Add(-91 * 24 * time.Hour)},
		"https://example.com/recent": {Title: "Recent", LastSeen: now.Add(-1 * 24 * time.Hour)},
	}}
	if err := s.Save(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if _, ok := loaded.Listings["https://example.com/old"]; ok {
		t.Error("record older than 90 days should have been pruned")
	}
	if _, ok := loaded.Listings["https://example.com/recent"]; !ok {
		t.Error("recent record should have survived pruning")
	}
}

func TestDiffNewListing(t *testing.T) {
	s := &State{Version: 1, Listings: map[string]*StateRecord{}}
	now := time.Now()

	diff := s.Diff([]Listing{
		{Title: "Haus 1", URL: "https://example.com/haus-1", PriceRaw: "688.000 €", PriceCents: 68800000},
	}, now)

	if len(diff.New) != 1 || diff.New[0].Title != "Haus 1" {
		t.Fatalf("expected 1 new listing, got %+v", diff.New)
	}
	if len(diff.PriceDrops) != 0 {
		t.Errorf("expected no price drops, got %+v", diff.PriceDrops)
	}
	rec := s.Listings["https://example.com/haus-1"]
	if rec == nil || rec.LastPriceCents != 68800000 || !rec.FirstSeen.Equal(now) {
		t.Errorf("state record not created correctly: %+v", rec)
	}
}

func TestDiffKnownListingUnchanged(t *testing.T) {
	earlier := time.Now().Add(-24 * time.Hour)
	now := time.Now()
	s := &State{Version: 1, Listings: map[string]*StateRecord{
		"https://example.com/haus-1": {Title: "Haus 1", FirstSeen: earlier, LastSeen: earlier, LastPriceCents: 68800000, LastPriceRaw: "688.000 €"},
	}}

	diff := s.Diff([]Listing{
		{Title: "Haus 1", URL: "https://example.com/haus-1", PriceRaw: "688.000 €", PriceCents: 68800000},
	}, now)

	if !diff.Empty() {
		t.Errorf("expected empty diff, got %+v", diff)
	}
	if !s.Listings["https://example.com/haus-1"].LastSeen.Equal(now) {
		t.Error("last_seen should be updated on every run")
	}
}

func TestDiffPriceDrop(t *testing.T) {
	earlier := time.Now().Add(-24 * time.Hour)
	now := time.Now()
	s := &State{Version: 1, Listings: map[string]*StateRecord{
		"https://example.com/haus-1": {Title: "Haus 1", FirstSeen: earlier, LastSeen: earlier, LastPriceCents: 68800000, LastPriceRaw: "688.000 €"},
	}}

	diff := s.Diff([]Listing{
		{Title: "Haus 1", URL: "https://example.com/haus-1", PriceRaw: "650.000 €", PriceCents: 65000000},
	}, now)

	if len(diff.PriceDrops) != 1 {
		t.Fatalf("expected 1 price drop, got %+v", diff.PriceDrops)
	}
	drop := diff.PriceDrops[0]
	if drop.OldPriceRaw != "688.000 €" || drop.PriceRaw != "650.000 €" {
		t.Errorf("price drop has wrong prices: %+v", drop)
	}
	if len(diff.New) != 0 {
		t.Errorf("expected no new listings, got %+v", diff.New)
	}
	rec := s.Listings["https://example.com/haus-1"]
	if rec.LastPriceCents != 65000000 || rec.LastPriceRaw != "650.000 €" {
		t.Errorf("last_price should be updated after drop check: %+v", rec)
	}
}

func TestDiffPriceIncreaseNotReported(t *testing.T) {
	earlier := time.Now().Add(-24 * time.Hour)
	s := &State{Version: 1, Listings: map[string]*StateRecord{
		"https://example.com/haus-1": {LastSeen: earlier, LastPriceCents: 60000000, LastPriceRaw: "600.000 €"},
	}}

	diff := s.Diff([]Listing{
		{URL: "https://example.com/haus-1", PriceRaw: "650.000 €", PriceCents: 65000000},
	}, time.Now())

	if !diff.Empty() {
		t.Errorf("price increase must not be reported, got %+v", diff)
	}
	if s.Listings["https://example.com/haus-1"].LastPriceCents != 65000000 {
		t.Error("last_price should still be updated on increase")
	}
}

func TestDiffUnknownPriceNoDrop(t *testing.T) {
	earlier := time.Now().Add(-24 * time.Hour)
	s := &State{Version: 1, Listings: map[string]*StateRecord{
		"https://example.com/haus-1": {LastSeen: earlier, LastPriceCents: 60000000, LastPriceRaw: "600.000 €"},
	}}

	// Price became "auf Anfrage" (0) — must not count as a drop.
	diff := s.Diff([]Listing{
		{URL: "https://example.com/haus-1", PriceRaw: "auf Anfrage", PriceCents: 0},
	}, time.Now())

	if !diff.Empty() {
		t.Errorf("unknown price must not be reported as drop, got %+v", diff)
	}
}

func TestSaveIsAtomicOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte(`{"version":1,"listings":{}}`), 0644)

	s := &State{Version: 1, Listings: map[string]*StateRecord{
		"https://example.com/x": {Title: "X", LastSeen: time.Now()},
	}}
	if err := s.Save(path); err != nil {
		t.Fatalf("save over existing file failed: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

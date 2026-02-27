package aggregator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/janiskrasemann/burrow/internal/fetcher"
)

type stubFetcher struct {
	name  string
	data  any
	err   error
	delay time.Duration
}

func (s *stubFetcher) Name() string { return s.name }

func (s *stubFetcher) Fetch(ctx context.Context) (any, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.data, s.err
}

func TestFetchAllSuccess(t *testing.T) {
	agg := New(
		&stubFetcher{name: "A", data: "data-a"},
		&stubFetcher{name: "B", data: "data-b"},
		&stubFetcher{name: "C", data: "data-c"},
	)

	results := agg.FetchAll(context.Background())

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("unexpected error for %s: %v", r.Name, r.Error)
		}
	}
	if results[0].Data != "data-a" {
		t.Errorf("expected 'data-a', got %v", results[0].Data)
	}
}

func TestFetchAllPartialError(t *testing.T) {
	agg := New(
		&stubFetcher{name: "OK", data: "ok-data"},
		&stubFetcher{name: "Fail", err: fmt.Errorf("network down")},
		&stubFetcher{name: "OK2", data: "ok2-data"},
	)

	results := agg.FetchAll(context.Background())

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("expected no error for OK, got %v", results[0].Error)
	}
	if results[1].Error == nil {
		t.Error("expected error for Fail")
	}
	if results[2].Error != nil {
		t.Errorf("expected no error for OK2, got %v", results[2].Error)
	}
}

func TestFetchAllPreservesOrder(t *testing.T) {
	agg := New(
		&stubFetcher{name: "Slow", data: "slow-data", delay: 50 * time.Millisecond},
		&stubFetcher{name: "Fast1", data: "fast1-data"},
		&stubFetcher{name: "Fast2", data: "fast2-data"},
	)

	results := agg.FetchAll(context.Background())

	// Results should match input order, not completion order
	expected := []string{"Slow", "Fast1", "Fast2"}
	for i, name := range expected {
		if results[i].Name != name {
			t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, name)
		}
	}
}

func TestFetchAllEmpty(t *testing.T) {
	agg := New()

	results := agg.FetchAll(context.Background())

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFetchAllContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	agg := New(
		&stubFetcher{name: "Blocked", delay: 5 * time.Second},
	)

	results := agg.FetchAll(ctx)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected context error")
	}

	_ = results[0] // Verify we can safely access the result without panic
}

// Verify stubFetcher implements Fetcher interface
var _ fetcher.Fetcher = (*stubFetcher)(nil)

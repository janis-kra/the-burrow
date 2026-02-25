package fetcher

import "context"

// Result holds the output of a single fetcher.
type Result struct {
	Name  string
	Data  any
	Error error
}

// Fetcher is the interface all content modules implement.
type Fetcher interface {
	// Name returns a human-readable name for the module.
	Name() string
	// Fetch retrieves content from the source.
	Fetch(ctx context.Context) (any, error)
}

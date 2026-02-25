package aggregator

import (
	"context"
	"log"
	"sync"

	"github.com/janiskrasemann/burrow/internal/fetcher"
)

type Aggregator struct {
	fetchers []fetcher.Fetcher
}

func New(fetchers ...fetcher.Fetcher) *Aggregator {
	return &Aggregator{fetchers: fetchers}
}

func (a *Aggregator) FetchAll(ctx context.Context) []fetcher.Result {
	results := make([]fetcher.Result, len(a.fetchers))
	var wg sync.WaitGroup

	for i, f := range a.fetchers {
		wg.Add(1)
		go func(idx int, ft fetcher.Fetcher) {
			defer wg.Done()
			log.Printf("Fetching %s...", ft.Name())
			data, err := ft.Fetch(ctx)
			if err != nil {
				log.Printf("Error fetching %s: %v", ft.Name(), err)
			} else {
				log.Printf("Fetched %s successfully", ft.Name())
			}
			results[idx] = fetcher.Result{
				Name:  ft.Name(),
				Data:  data,
				Error: err,
			}
		}(i, f)
	}

	wg.Wait()
	return results
}

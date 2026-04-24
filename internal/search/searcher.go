package search

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/billmal071/audbookdl/internal/source"
)

type Searcher struct {
	sources []source.Source
}

func New(sources ...source.Source) *Searcher {
	return &Searcher{sources: sources}
}

// Search queries all sources concurrently. Returns partial results if some fail.
// Returns error only if ALL sources fail.
func (s *Searcher) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	if len(s.sources) == 0 {
		return nil, nil
	}

	type result struct {
		books []*source.Audiobook
		err   error
	}
	results := make([]result, len(s.sources))
	var wg sync.WaitGroup

	for i, src := range s.sources {
		wg.Add(1)
		go func(idx int, src source.Source) {
			defer wg.Done()
			books, err := src.Search(ctx, query, opts)
			results[idx] = result{books: books, err: err}
		}(i, src)
	}
	wg.Wait()

	var allBooks []*source.Audiobook
	var errs []string
	for i, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", s.sources[i].Name(), r.err))
			continue
		}
		allBooks = append(allBooks, r.books...)
	}
	if len(errs) == len(s.sources) {
		return nil, fmt.Errorf("all sources failed: %s", strings.Join(errs, "; "))
	}
	return allBooks, nil
}

func (s *Searcher) Sources() []source.Source { return s.sources }

// Package searchapi provides a public search API for MinusSync.
package searchapi

import (
	"github.com/MinusSync/internal/search"
)

// SearchAPI wraps the search index for public consumption.
type SearchAPI struct {
	idx *search.Index
}

// Open opens a search index.
func Open(path string) (*SearchAPI, error) {
	idx, err := search.Open(path)
	if err != nil {
		return nil, err
	}
	return &SearchAPI{idx: idx}, nil
}

// Search executes a query.
func (s *SearchAPI) Search(query string, maxResults int) ([]search.Result, error) {
	q := search.ParseQuery(query, false, maxResults)
	return s.idx.Search(q)
}

// Close closes the search index.
func (s *SearchAPI) Close() error {
	return s.idx.Close()
}

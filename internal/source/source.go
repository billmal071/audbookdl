package source

import "context"

// Source is the contract all audiobook source clients must implement.
type Source interface {
	// Search returns audiobooks matching the query and options.
	Search(ctx context.Context, query string, opts SearchOptions) ([]*Audiobook, error)

	// GetChapters returns the chapters for the given audiobook ID.
	GetChapters(ctx context.Context, bookID string) ([]*Chapter, error)

	// Name returns the human-readable name of the source.
	Name() string
}

// SearchOptions configures a Search call.
type SearchOptions struct {
	Limit    int
	Page     int
	Language string
	Author   string
	Format   string
}

package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/billmal071/audbookdl/internal/source"
)

type mockSource struct {
	name     string
	books    []*source.Audiobook
	chapters []*source.Chapter
	err      error
	delay    time.Duration
}

func (m *mockSource) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.books, m.err
}
func (m *mockSource) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	return m.chapters, m.err
}
func (m *mockSource) Name() string { return m.name }

func TestSearcher_MultipleSources(t *testing.T) {
	s := New(
		&mockSource{name: "source1", books: []*source.Audiobook{{ID: "1", Title: "Book A", Source: "source1"}}},
		&mockSource{name: "source2", books: []*source.Audiobook{{ID: "2", Title: "Book B", Source: "source2"}}},
	)
	books, err := s.Search(context.Background(), "test", source.SearchOptions{})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 2 {
		t.Errorf("got %d books, want 2", len(books))
	}
}

func TestSearcher_PartialFailure(t *testing.T) {
	s := New(
		&mockSource{name: "good", books: []*source.Audiobook{{ID: "1", Title: "Book A", Source: "good"}}},
		&mockSource{name: "bad", err: errors.New("connection refused")},
	)
	books, err := s.Search(context.Background(), "test", source.SearchOptions{})
	if err != nil {
		t.Fatalf("expected no error with partial results, got: %v", err)
	}
	if len(books) != 1 {
		t.Errorf("got %d books, want 1", len(books))
	}
}

func TestSearcher_AllFail(t *testing.T) {
	s := New(
		&mockSource{name: "bad1", err: errors.New("fail1")},
		&mockSource{name: "bad2", err: errors.New("fail2")},
	)
	_, err := s.Search(context.Background(), "test", source.SearchOptions{})
	if err == nil {
		t.Error("expected error when all sources fail")
	}
}

func TestSearcher_Empty(t *testing.T) {
	s := New()
	books, err := s.Search(context.Background(), "test", source.SearchOptions{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(books) != 0 {
		t.Errorf("got %d books, want 0", len(books))
	}
}

func TestSearcher_ContextCancellation(t *testing.T) {
	s := New(&mockSource{name: "slow", books: []*source.Audiobook{{ID: "1"}}, delay: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := s.Search(ctx, "test", source.SearchOptions{})
	if err == nil {
		t.Error("expected error on context cancellation")
	}
}

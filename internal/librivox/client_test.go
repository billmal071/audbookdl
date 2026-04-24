package librivox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

const fixtureJSON = `{
  "books": [{
    "id": "1234",
    "title": "The Adventures of Sherlock Holmes",
    "description": "A collection of stories",
    "url_librivox": "https://librivox.org/the-adventures-of-sherlock-holmes",
    "language": "English",
    "copyright_year": "1892",
    "totaltime": "11:32:00",
    "totaltimesecs": 41520,
    "num_sections": "12",
    "authors": [{"id": "42", "first_name": "Arthur Conan", "last_name": "Doyle"}],
    "sections": [
      {"id": "1", "section_number": "1", "title": "A Scandal in Bohemia", "listen_url": "https://archive.org/download/adventures_holmes/ch01.mp3", "language": "English", "playtime": "00:32:15", "readers": [{"display_name": "Mark Nelson"}]},
      {"id": "2", "section_number": "2", "title": "The Red-Headed League", "listen_url": "https://archive.org/download/adventures_holmes/ch02.mp3", "language": "English", "playtime": "00:28:40", "readers": [{"display_name": "Mark Nelson"}]}
    ]
  }]
}`

func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
}

func TestSearch(t *testing.T) {
	srv := newTestServer(t, fixtureJSON)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	books, err := client.Search(context.Background(), "sherlock", source.SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}

	b := books[0]
	if b.Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("title: got %q, want %q", b.Title, "The Adventures of Sherlock Holmes")
	}
	if b.Author != "Arthur Conan Doyle" {
		t.Errorf("author: got %q, want %q", b.Author, "Arthur Conan Doyle")
	}
	if b.Source != "librivox" {
		t.Errorf("source: got %q, want %q", b.Source, "librivox")
	}
	if b.ChapterCount != 12 {
		t.Errorf("chapterCount: got %d, want %d", b.ChapterCount, 12)
	}
}

func TestGetChapters(t *testing.T) {
	srv := newTestServer(t, fixtureJSON)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "1234")
	if err != nil {
		t.Fatalf("GetChapters returned error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}

	ch := chapters[0]
	if ch.Title != "A Scandal in Bohemia" {
		t.Errorf("chapter title: got %q, want %q", ch.Title, "A Scandal in Bohemia")
	}
	if ch.Index != 1 {
		t.Errorf("chapter index: got %d, want %d", ch.Index, 1)
	}
	if ch.Format != "mp3" {
		t.Errorf("chapter format: got %q, want %q", ch.Format, "mp3")
	}
}

func TestName(t *testing.T) {
	client := NewClient("", httpclient.New())
	if name := client.Name(); name != "librivox" {
		t.Errorf("Name(): got %q, want %q", name, "librivox")
	}
}

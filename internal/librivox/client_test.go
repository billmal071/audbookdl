package librivox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

// ── fixtures ──────────────────────────────────────────────────────────────────

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

const fixtureRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
<channel>
<title>The Adventures of Sherlock Holmes</title>
<item>
  <title>01 - A Scandal in Bohemia</title>
  <itunes:episode>1</itunes:episode>
  <itunes:duration>00:32:15</itunes:duration>
  <enclosure url="https://archive.org/download/adventures_holmes/ch01.mp3" length="0" type="audio/mpeg"/>
</item>
<item>
  <title>02 - The Red-Headed League</title>
  <itunes:episode>2</itunes:episode>
  <itunes:duration>00:28:40</itunes:duration>
  <enclosure url="https://archive.org/download/adventures_holmes/ch02.mp3" length="0" type="audio/mpeg"/>
</item>
</channel>
</rss>`

const fixtureAPIError = `{"error": "no audiobooks were found"}`

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
}

// newMuxServer creates a test server that dispatches by path prefix so we can
// serve the RSS feed and the API from the same host.
func newMuxServer(t *testing.T, mux *http.ServeMux) *httptest.Server {
	t.Helper()
	return httptest.NewServer(mux)
}

// ── tests ─────────────────────────────────────────────────────────────────────

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

func TestSearch_APIError(t *testing.T) {
	srv := newTestServer(t, fixtureAPIError)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	books, err := client.Search(context.Background(), "zzznomatch", source.SearchOptions{})
	if err != nil {
		t.Fatalf("Search with API error should not return a Go error, got: %v", err)
	}
	if books != nil {
		t.Errorf("expected nil books slice for API error, got %v", books)
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

func TestGetChapters_RSS(t *testing.T) {
	mux := http.NewServeMux()
	// Serve the RSS feed on /rss/1234
	mux.HandleFunc("/rss/1234", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fixtureRSS))
	})
	// Return an empty JSON if the API fallback is hit (should not be needed).
	mux.HandleFunc("/api/feed/audiobooks/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"books":[]}`))
	})

	srv := newMuxServer(t, mux)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "1234")
	if err != nil {
		t.Fatalf("GetChapters_RSS returned error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters from RSS, got %d", len(chapters))
	}

	ch := chapters[0]
	if ch.Title != "01 - A Scandal in Bohemia" {
		t.Errorf("RSS chapter title: got %q, want %q", ch.Title, "01 - A Scandal in Bohemia")
	}
	if ch.Index != 1 {
		t.Errorf("RSS chapter index: got %d, want %d", ch.Index, 1)
	}
	if ch.DownloadURL != "https://archive.org/download/adventures_holmes/ch01.mp3" {
		t.Errorf("RSS chapter URL: got %q", ch.DownloadURL)
	}
	if ch.Format != "mp3" {
		t.Errorf("RSS chapter format: got %q, want %q", ch.Format, "mp3")
	}

	ch2 := chapters[1]
	if ch2.Title != "02 - The Red-Headed League" {
		t.Errorf("RSS chapter 2 title: got %q", ch2.Title)
	}
	if ch2.Index != 2 {
		t.Errorf("RSS chapter 2 index: got %d, want %d", ch2.Index, 2)
	}
}

func TestName(t *testing.T) {
	client := NewClient("", httpclient.New())
	if name := client.Name(); name != "librivox" {
		t.Errorf("Name(): got %q, want %q", name, "librivox")
	}
}

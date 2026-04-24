package loyalbooks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

const searchHTML = `<html><body>
<table class="layout2-blue">
<tr>
  <td class="layout2"><a href="/book/adventures-of-sherlock-holmes"><img src="/cover.jpg"/></a></td>
  <td class="layout2"><a href="/book/adventures-of-sherlock-holmes">The Adventures of Sherlock Holmes</a><br/>By: <a href="/author/Arthur-Conan-Doyle">Arthur Conan Doyle</a></td>
</tr>
<tr>
  <td class="layout2"><a href="/book/study-in-scarlet"><img src="/cover2.jpg"/></a></td>
  <td class="layout2"><a href="/book/study-in-scarlet">A Study in Scarlet</a><br/>By: <a href="/author/Arthur-Conan-Doyle">Arthur Conan Doyle</a></td>
</tr>
</table>
</body></html>`

const bookHTML = `<html><body>
<table class="chapter-download">
<tr><td>1</td><td><a href="https://archive.org/download/adventures_holmes/ch01.mp3">A Scandal in Bohemia</a></td><td>32:15</td></tr>
<tr><td>2</td><td><a href="https://archive.org/download/adventures_holmes/ch02.mp3">The Red-Headed League</a></td><td>28:40</td></tr>
</table>
</body></html>`

func newTestServer(searchBody, bookBody string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(searchBody))
	})
	mux.HandleFunc("/book/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(bookBody))
	})
	return httptest.NewServer(mux)
}

func TestSearch(t *testing.T) {
	srv := newTestServer(searchHTML, bookHTML)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	books, err := client.Search(context.Background(), "sherlock", source.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books, got %d", len(books))
	}

	first := books[0]
	if first.Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("expected title %q, got %q", "The Adventures of Sherlock Holmes", first.Title)
	}
	if first.Author != "Arthur Conan Doyle" {
		t.Errorf("expected author %q, got %q", "Arthur Conan Doyle", first.Author)
	}
	if first.Source != "loyalbooks" {
		t.Errorf("expected source %q, got %q", "loyalbooks", first.Source)
	}
}

func TestGetChapters(t *testing.T) {
	srv := newTestServer(searchHTML, bookHTML)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "adventures-of-sherlock-holmes")
	if err != nil {
		t.Fatalf("GetChapters() error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}

	first := chapters[0]
	if first.Title != "A Scandal in Bohemia" {
		t.Errorf("expected title %q, got %q", "A Scandal in Bohemia", first.Title)
	}
	if first.Index != 1 {
		t.Errorf("expected index 1, got %d", first.Index)
	}
	if first.DownloadURL == "" {
		t.Error("expected non-empty DownloadURL")
	}
}

func TestName(t *testing.T) {
	client := NewClient("", httpclient.New())
	if name := client.Name(); name != "loyalbooks" {
		t.Errorf("expected name %q, got %q", "loyalbooks", name)
	}
}

func TestSearch_FallbackSelectors(t *testing.T) {
	// HTML without table.layout2-blue but with /book/ links in div-based layout
	divHTML := `<html><body>
<div class="results">
  <div class="book-item">
    <a href="/book/sherlock-holmes">Sherlock Holmes</a>
    <a href="/author/doyle">Arthur Conan Doyle</a>
  </div>
  <div class="book-item">
    <a href="/book/hound-of-baskervilles">The Hound of the Baskervilles</a>
    <a href="/author/doyle">Arthur Conan Doyle</a>
  </div>
</div>
</body></html>`

	srv := newTestServer(divHTML, bookHTML)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	books, err := client.Search(context.Background(), "sherlock", source.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books via strategy 2, got %d", len(books))
	}
	if books[0].Title != "Sherlock Holmes" {
		t.Errorf("expected title %q, got %q", "Sherlock Holmes", books[0].Title)
	}
	if books[0].Author != "Arthur Conan Doyle" {
		t.Errorf("expected author %q, got %q", "Arthur Conan Doyle", books[0].Author)
	}
}

func TestSearch_GenericFallback(t *testing.T) {
	// HTML without table.layout2-blue and without known div classes — generic /book/ links only
	genericHTML := `<html><body>
<ul>
  <li><a href="/book/moby-dick">Moby Dick</a></li>
  <li><a href="/book/war-and-peace">War and Peace</a></li>
</ul>
</body></html>`

	srv := newTestServer(genericHTML, bookHTML)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	books, err := client.Search(context.Background(), "moby", source.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books via generic strategy, got %d", len(books))
	}
	if books[0].Title != "Moby Dick" {
		t.Errorf("expected title %q, got %q", "Moby Dick", books[0].Title)
	}
}

func TestGetChapters_FallbackMP3Links(t *testing.T) {
	// HTML without table.chapter-download but with bare .mp3 links
	mp3HTML := `<html><body>
<div>
  <a href="https://archive.org/download/book/ch01.mp3">Chapter 1</a>
  <a href="https://archive.org/download/book/ch02.mp3">Chapter 2</a>
  <a href="https://archive.org/download/book/ch03.mp3">Chapter 3</a>
</div>
</body></html>`

	srv := newTestServer(searchHTML, mp3HTML)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "some-book")
	if err != nil {
		t.Fatalf("GetChapters() error: %v", err)
	}
	if len(chapters) != 3 {
		t.Fatalf("expected 3 chapters via MP3 link fallback, got %d", len(chapters))
	}
	if chapters[0].Title != "Chapter 1" {
		t.Errorf("expected title %q, got %q", "Chapter 1", chapters[0].Title)
	}
	if chapters[1].Index != 2 {
		t.Errorf("expected index 2, got %d", chapters[1].Index)
	}
	if chapters[2].DownloadURL != "https://archive.org/download/book/ch03.mp3" {
		t.Errorf("unexpected download URL: %q", chapters[2].DownloadURL)
	}
}

func TestBuildSearchURL_URLEncoding(t *testing.T) {
	u := buildSearchURL("https://www.loyalbooks.com", "sherlock holmes & watson")
	expected := "https://www.loyalbooks.com/search?q=sherlock+holmes+%26+watson"
	if u != expected {
		t.Errorf("expected URL %q, got %q", expected, u)
	}
}

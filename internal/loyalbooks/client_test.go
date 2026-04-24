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

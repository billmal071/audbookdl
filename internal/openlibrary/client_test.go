package openlibrary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

const searchFixture = `{"numFound":2,"start":0,"docs":[{"key":"/works/OL262758W","title":"The Adventures of Sherlock Holmes","author_name":["Arthur Conan Doyle"],"first_publish_year":1892,"ia":["adventures_sherlock_holmes_0711_librivox","adventuresofsherlo0000doyl"]},{"key":"/works/OL262421W","title":"A Study in Scarlet","author_name":["Arthur Conan Doyle"],"first_publish_year":1887,"ia":["study_in_scarlet_librivox"]}]}`

const iaFixture = `{"metadata":{"identifier":"adventures_sherlock_holmes_0711_librivox"},"files":[{"name":"chapter01.mp3","format":"VBR MP3","size":"18874368","length":"1935.5","title":"01 - A Scandal in Bohemia"}]}`

func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
}

func TestSearch(t *testing.T) {
	srv := newTestServer(t, searchFixture)
	defer srv.Close()

	client := NewClient(srv.URL, "", httpclient.New())
	books, err := client.Search(context.Background(), "sherlock", source.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books, got %d", len(books))
	}

	b := books[0]
	if b.Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("title: got %q, want %q", b.Title, "The Adventures of Sherlock Holmes")
	}
	if b.Author != "Arthur Conan Doyle" {
		t.Errorf("author: got %q, want %q", b.Author, "Arthur Conan Doyle")
	}
	if b.Year != "1892" {
		t.Errorf("year: got %q, want %q", b.Year, "1892")
	}
	if b.Source != "openlibrary" {
		t.Errorf("source: got %q, want %q", b.Source, "openlibrary")
	}
	if b.ID != "adventures_sherlock_holmes_0711_librivox" {
		t.Errorf("ID: got %q, want first IA identifier %q", b.ID, "adventures_sherlock_holmes_0711_librivox")
	}
}

func TestGetChapters_DelegatesToIA(t *testing.T) {
	iaSrv := newTestServer(t, iaFixture)
	defer iaSrv.Close()

	client := NewClient("", iaSrv.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "adventures_sherlock_holmes_0711_librivox")
	if err != nil {
		t.Fatalf("GetChapters returned error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters))
	}

	ch := chapters[0]
	if ch.Format != "mp3" {
		t.Errorf("format: got %q, want %q", ch.Format, "mp3")
	}
}

func TestName(t *testing.T) {
	client := NewClient("", "", httpclient.New())
	if name := client.Name(); name != "openlibrary" {
		t.Errorf("Name(): got %q, want %q", name, "openlibrary")
	}
}

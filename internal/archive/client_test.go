package archive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

const searchFixture = `{"response":{"numFound":1,"start":0,"docs":[{"identifier":"adventures_sherlock_holmes_0711_librivox","title":"The Adventures of Sherlock Holmes","creator":"Arthur Conan Doyle","description":"Twelve stories of mystery","date":"2006-08-01T00:00:00Z","downloads":50000}]}}`

const metadataFixture = `{"metadata":{"identifier":"adventures_sherlock_holmes_0711_librivox","title":"The Adventures of Sherlock Holmes","creator":"Arthur Conan Doyle"},"files":[{"name":"ch01.mp3","format":"VBR MP3","size":"18874368","length":"1935.5","title":"01 - A Scandal in Bohemia"},{"name":"ch02.mp3","format":"VBR MP3","size":"17301504","length":"1720.2","title":"02 - The Red-Headed League"},{"name":"ch01.ogg","format":"Ogg Vorbis","size":"12345678","length":"1935.5","title":"01 - A Scandal in Bohemia"},{"name":"__ia_thumb.jpg","format":"Thumbnail","size":"5000"}]}`

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
	if b.Source != "archive" {
		t.Errorf("source: got %q, want %q", b.Source, "archive")
	}
	if b.ID != "adventures_sherlock_holmes_0711_librivox" {
		t.Errorf("ID: got %q, want %q", b.ID, "adventures_sherlock_holmes_0711_librivox")
	}
}

func TestGetChapters(t *testing.T) {
	srv := newTestServer(t, metadataFixture)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "adventures_sherlock_holmes_0711_librivox")
	if err != nil {
		t.Fatalf("GetChapters returned error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters (MP3 only), got %d", len(chapters))
	}

	ch1 := chapters[0]
	if ch1.Title != "01 - A Scandal in Bohemia" {
		t.Errorf("chapter 1 title: got %q, want %q", ch1.Title, "01 - A Scandal in Bohemia")
	}
	if ch1.Index != 1 {
		t.Errorf("chapter 1 index: got %d, want %d", ch1.Index, 1)
	}
	if ch1.Format != "mp3" {
		t.Errorf("chapter 1 format: got %q, want %q", ch1.Format, "mp3")
	}
	if ch1.FileSize != 18874368 {
		t.Errorf("chapter 1 filesize: got %d, want %d", ch1.FileSize, 18874368)
	}

	ch2 := chapters[1]
	if ch2.Title != "02 - The Red-Headed League" {
		t.Errorf("chapter 2 title: got %q, want %q", ch2.Title, "02 - The Red-Headed League")
	}
	if ch2.Index != 2 {
		t.Errorf("chapter 2 index: got %d, want %d", ch2.Index, 2)
	}
}

func TestGetChapters_FiltersMp3Only(t *testing.T) {
	srv := newTestServer(t, metadataFixture)
	defer srv.Close()

	client := NewClient(srv.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "adventures_sherlock_holmes_0711_librivox")
	if err != nil {
		t.Fatalf("GetChapters returned error: %v", err)
	}

	for _, ch := range chapters {
		if ch.Format != "mp3" {
			t.Errorf("chapter %d has format %q, want %q", ch.Index, ch.Format, "mp3")
		}
	}
}

func TestName(t *testing.T) {
	client := NewClient("", httpclient.New())
	if client.Name() != "archive" {
		t.Errorf("Name() = %q, want %q", client.Name(), "archive")
	}
}

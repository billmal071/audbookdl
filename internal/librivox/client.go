package librivox

import (
	"context"
	"fmt"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

type Client struct {
	baseURL string
	http    *httpclient.Client
}

func NewClient(baseURL string, http *httpclient.Client) *Client {
	if baseURL == "" {
		baseURL = "https://librivox.org"
	}
	return &Client{baseURL: baseURL, http: http}
}

func (c *Client) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	url := buildSearchURL(c.baseURL, query, opts)
	var resp apiResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("librivox search: %w", err)
	}
	books := make([]*source.Audiobook, 0, len(resp.Books))
	for _, b := range resp.Books {
		books = append(books, b.toAudiobook())
	}
	return books, nil
}

func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	url := buildGetURL(c.baseURL, bookID)
	var resp apiResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("librivox get chapters: %w", err)
	}
	if len(resp.Books) == 0 {
		return nil, fmt.Errorf("librivox: book %s not found", bookID)
	}
	book := resp.Books[0]
	chapters := make([]*source.Chapter, 0, len(book.Sections))
	for _, s := range book.Sections {
		chapters = append(chapters, s.toChapter())
	}
	return chapters, nil
}

func (c *Client) Name() string { return "librivox" }

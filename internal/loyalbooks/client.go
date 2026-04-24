package loyalbooks

import (
	"bytes"
	"context"
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

// Client scrapes Loyal Books for free public-domain audiobooks.
type Client struct {
	baseURL string
	http    *httpclient.Client
}

// NewClient creates a new Loyal Books client. If baseURL is empty it defaults
// to https://www.loyalbooks.com.
func NewClient(baseURL string, http *httpclient.Client) *Client {
	if baseURL == "" {
		baseURL = "https://www.loyalbooks.com"
	}
	return &Client{baseURL: baseURL, http: http}
}

// Search returns audiobooks matching the query from Loyal Books.
func (c *Client) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	url := buildSearchURL(c.baseURL, query)
	body, err := c.http.GetBody(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("loyalbooks search: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("loyalbooks parse: %w", err)
	}
	listings := parseSearchPage(doc)
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	if len(listings) > limit {
		listings = listings[:limit]
	}
	books := make([]*source.Audiobook, 0, len(listings))
	for _, l := range listings {
		books = append(books, &source.Audiobook{
			ID:      l.Slug,
			Title:   l.Title,
			Author:  l.Author,
			PageURL: buildBookURL(c.baseURL, l.Slug),
			Format:  "mp3",
			Source:  "loyalbooks",
		})
	}
	return books, nil
}

// GetChapters returns chapters for the audiobook identified by bookID (the slug).
func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	url := buildBookURL(c.baseURL, bookID)
	body, err := c.http.GetBody(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("loyalbooks book page: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("loyalbooks parse: %w", err)
	}
	chapters := parseBookPage(doc)
	if len(chapters) == 0 {
		return nil, fmt.Errorf("loyalbooks: no chapters found for %s", bookID)
	}
	return chapters, nil
}

// Name returns the source identifier.
func (c *Client) Name() string { return "loyalbooks" }

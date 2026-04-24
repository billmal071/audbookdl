package librivox

import (
	"context"
	"encoding/json"
	"encoding/xml"
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
	searchURL := buildSearchURL(c.baseURL, query, opts)
	body, err := c.http.GetBody(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("librivox search: %w", err)
	}

	resp, err := decodeAPIResponse(body)
	if err != nil || resp.Error != "" {
		return nil, nil
	}

	books := make([]*source.Audiobook, 0, len(resp.Books))
	for _, b := range resp.Books {
		books = append(books, b.toAudiobook())
	}
	return books, nil
}

func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	// Try RSS feed first — it reliably carries <enclosure> MP3 URLs.
	rssURL := buildRSSURL(c.baseURL, bookID)
	if body, err := c.http.GetBody(ctx, rssURL); err == nil {
		if chapters := parseRSS(body); len(chapters) > 0 {
			return chapters, nil
		}
	}

	// Fall back to the extended API endpoint.
	apiURL := buildGetURL(c.baseURL, bookID)
	body, err := c.http.GetBody(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("librivox get chapters: %w", err)
	}

	resp, err := decodeAPIResponse(body)
	if err != nil || resp.Error != "" {
		return nil, fmt.Errorf("librivox: book %s not found", bookID)
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

// ── helpers ───────────────────────────────────────────────────────────────────

// decodeAPIResponse tries JSON first, then XML.
func decodeAPIResponse(body []byte) (*apiResponse, error) {
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err == nil {
		return &resp, nil
	}
	// JSON failed — try XML (the API sometimes returns XML without a format param).
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode API response: %w", err)
	}
	return &resp, nil
}


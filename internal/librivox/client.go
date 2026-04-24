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
	body, err := c.http.GetBody(ctx, rssURL)
	if err == nil {
		if chapters := parseRSS(body); len(chapters) > 0 {
			return chapters, nil
		}
	}

	// Fall back to the API endpoint with ?id=
	apiURL := buildGetURL(c.baseURL, bookID)
	body, err = c.http.GetBody(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("librivox get chapters: %w", err)
	}

	resp, decErr := decodeAPIResponse(body)
	if decErr != nil || resp.Error != "" || len(resp.Books) == 0 {
		return nil, fmt.Errorf("librivox: book %s not found", bookID)
	}

	book := resp.Books[0]
	chapters := make([]*source.Chapter, 0, len(book.Sections))
	for _, s := range book.Sections {
		chapters = append(chapters, s.toChapter())
	}
	return chapters, nil
}

// GetBook fetches a single audiobook by its LibriVox ID.
// Tries the API first, falls back to RSS metadata extraction.
func (c *Client) GetBook(ctx context.Context, bookID string) (*source.Audiobook, error) {
	// Try API
	apiURL := buildGetURL(c.baseURL, bookID)
	if body, err := c.http.GetBody(ctx, apiURL); err == nil {
		if resp, decErr := decodeAPIResponse(body); decErr == nil && resp.Error == "" && len(resp.Books) > 0 {
			return resp.Books[0].toAudiobook(), nil
		}
	}

	// Fall back to RSS for metadata
	rssURL := buildRSSURL(c.baseURL, bookID)
	if body, err := c.http.GetBody(ctx, rssURL); err == nil {
		if book := parseRSSMetadata(body, bookID); book != nil {
			return book, nil
		}
	}

	return nil, fmt.Errorf("librivox: book %s not found", bookID)
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


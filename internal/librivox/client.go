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
	url := buildSearchURL(c.baseURL, query, opts)
	body, err := c.http.GetBody(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("librivox search: %w", err)
	}

	// Check for an API-level error embedded in the response body.
	if apiErr := extractAPIError(body); apiErr != "" {
		// Return empty results rather than a hard error — the API returns HTTP 200 for errors.
		return nil, nil
	}

	resp, err := decodeAPIResponse(body)
	if err != nil {
		// Could not decode at all — return empty rather than crash.
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

	if apiErr := extractAPIError(body); apiErr != "" {
		return nil, fmt.Errorf("librivox: book %s not found (%s)", bookID, apiErr)
	}

	resp, err := decodeAPIResponse(body)
	if err != nil {
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

// extractAPIError returns a non-empty string if the body encodes an API error.
func extractAPIError(body []byte) string {
	var errJSON apiErrorResponse
	if err := json.Unmarshal(body, &errJSON); err == nil && errJSON.Error != "" {
		return errJSON.Error
	}
	var errXML apiErrorResponse
	if err := xml.Unmarshal(body, &errXML); err == nil && errXML.Error != "" {
		return errXML.Error
	}
	return ""
}

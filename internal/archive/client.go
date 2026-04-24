package archive

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

type Client struct {
	baseURL string
	http    *httpclient.Client
}

func NewClient(baseURL string, http *httpclient.Client) *Client {
	if baseURL == "" {
		baseURL = "https://archive.org"
	}
	return &Client{baseURL: baseURL, http: http}
}

func (c *Client) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	url := buildSearchURL(c.baseURL, query, opts)
	var resp searchResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("archive search: %w", err)
	}
	books := make([]*source.Audiobook, 0, len(resp.Response.Docs))
	for _, d := range resp.Response.Docs {
		books = append(books, d.toAudiobook())
	}
	return books, nil
}

func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	url := buildMetadataURL(c.baseURL, bookID)
	var resp metadataResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("archive metadata: %w", err)
	}
	var chapters []*source.Chapter
	idx := 1
	for _, f := range resp.Files {
		if f.isAudioMP3() {
			chapters = append(chapters, f.toChapter(resp.Metadata.Identifier, idx))
			idx++
		}
	}
	sort.Slice(chapters, func(i, j int) bool {
		return strings.ToLower(chapters[i].DownloadURL) < strings.ToLower(chapters[j].DownloadURL)
	})
	for i := range chapters {
		chapters[i].Index = i + 1
	}
	return chapters, nil
}

func (c *Client) Name() string { return "archive" }

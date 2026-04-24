package openlibrary

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

type Client struct {
	baseURL   string
	iaBaseURL string
	http      *httpclient.Client
}

func NewClient(baseURL, iaBaseURL string, http *httpclient.Client) *Client {
	if baseURL == "" {
		baseURL = "https://openlibrary.org"
	}
	if iaBaseURL == "" {
		iaBaseURL = "https://archive.org"
	}
	return &Client{baseURL: baseURL, iaBaseURL: iaBaseURL, http: http}
}

func (c *Client) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	url := buildSearchURL(c.baseURL, query, opts)
	var resp searchResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("openlibrary search: %w", err)
	}
	books := make([]*source.Audiobook, 0, len(resp.Docs))
	for _, d := range resp.Docs {
		if len(d.IA) > 0 {
			books = append(books, d.toAudiobook())
		}
	}
	return books, nil
}

func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	url := fmt.Sprintf("%s/metadata/%s", c.iaBaseURL, bookID)
	var resp iaMetadataResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("openlibrary get chapters (via IA): %w", err)
	}
	var chapters []*source.Chapter
	idx := 1
	for _, f := range resp.Files {
		if f.isAudioMP3() {
			chapters = append(chapters, f.toChapter(bookID, idx))
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

func (c *Client) Name() string { return "openlibrary" }

// IA metadata types (local to avoid circular dep with archive package)
type iaMetadataResponse struct {
	Metadata struct {
		Identifier string `json:"identifier"`
	} `json:"metadata"`
	Files []iaFileInfo `json:"files"`
}

type iaFileInfo struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Size   string `json:"size"`
	Length string `json:"length"`
	Title  string `json:"title"`
}

func (f *iaFileInfo) isAudioMP3() bool {
	return strings.Contains(strings.ToLower(f.Format), "mp3") &&
		strings.HasSuffix(strings.ToLower(f.Name), ".mp3")
}

func (f *iaFileInfo) toChapter(identifier string, index int) *source.Chapter {
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	lengthSec, _ := strconv.ParseFloat(f.Length, 64)
	title := f.Title
	if title == "" {
		title = strings.TrimSuffix(f.Name, ".mp3")
	}
	return &source.Chapter{
		Index: index, Title: title,
		Duration:    time.Duration(math.Round(lengthSec)) * time.Second,
		DownloadURL: fmt.Sprintf("https://archive.org/download/%s/%s", identifier, f.Name),
		Format:      "mp3", FileSize: size,
	}
}

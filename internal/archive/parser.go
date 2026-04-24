package archive

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/source"
)

type searchResponse struct {
	Response struct {
		NumFound int         `json:"numFound"`
		Start    int         `json:"start"`
		Docs     []searchDoc `json:"docs"`
	} `json:"response"`
}

type searchDoc struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Creator     string `json:"creator"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Downloads   int    `json:"downloads"`
}

type metadataResponse struct {
	Metadata metadataInfo `json:"metadata"`
	Files    []fileInfo   `json:"files"`
}

type metadataInfo struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Creator     string `json:"creator"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Runtime     string `json:"runtime"`
}

type fileInfo struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Size   string `json:"size"`
	Length string `json:"length"`
	Title  string `json:"title"`
}

func (d *searchDoc) toAudiobook() *source.Audiobook {
	year := ""
	if len(d.Date) >= 4 {
		year = d.Date[:4]
	}
	return &source.Audiobook{
		ID:          d.Identifier,
		Title:       d.Title,
		Author:      d.Creator,
		Description: d.Description,
		Year:        year,
		PageURL:     fmt.Sprintf("https://archive.org/details/%s", d.Identifier),
		Format:      "mp3",
		Source:      "archive",
	}
}

func (f *fileInfo) isAudioMP3() bool {
	return strings.Contains(strings.ToLower(f.Format), "mp3") &&
		strings.HasSuffix(strings.ToLower(f.Name), ".mp3")
}

func (f *fileInfo) toChapter(identifier string, index int) *source.Chapter {
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	lengthSec, _ := strconv.ParseFloat(f.Length, 64)
	title := f.Title
	if title == "" {
		title = strings.TrimSuffix(f.Name, ".mp3")
	}
	return &source.Chapter{
		Index:       index,
		Title:       title,
		Duration:    time.Duration(math.Round(lengthSec)) * time.Second,
		DownloadURL: fmt.Sprintf("https://archive.org/download/%s/%s", identifier, f.Name),
		Format:      "mp3",
		FileSize:    size,
	}
}

func buildSearchURL(baseURL, query string, opts source.SearchOptions) string {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	q := fmt.Sprintf("title:(%s) AND collection:(librivoxaudio OR audio_bookspoetry)", query)
	u := fmt.Sprintf("%s/advancedsearch.php?q=%s&output=json&rows=%d&fl[]=identifier,title,creator,description,date,downloads",
		baseURL, url.QueryEscape(q), limit)
	if opts.Page > 0 {
		u += fmt.Sprintf("&page=%d", opts.Page+1)
	}
	return u
}

func buildMetadataURL(baseURL, identifier string) string {
	return fmt.Sprintf("%s/metadata/%s", baseURL, identifier)
}

package openlibrary

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/source"
)

type searchResponse struct {
	NumFound int         `json:"numFound"`
	Start    int         `json:"start"`
	Docs     []searchDoc `json:"docs"`
}

type searchDoc struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	IA               []string `json:"ia"`
}

func (d *searchDoc) toAudiobook() *source.Audiobook {
	author := ""
	if len(d.AuthorName) > 0 {
		author = d.AuthorName[0]
	}
	year := ""
	if d.FirstPublishYear > 0 {
		year = strconv.Itoa(d.FirstPublishYear)
	}
	id := d.Key
	if len(d.IA) > 0 {
		id = d.IA[0]
	}
	return &source.Audiobook{
		ID: id, Title: d.Title, Author: author, Year: year,
		PageURL: fmt.Sprintf("https://openlibrary.org%s", d.Key),
		Format:  "mp3", Source: "openlibrary",
	}
}

func buildSearchURL(baseURL, query string, opts source.SearchOptions) string {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	url := fmt.Sprintf("%s/search.json?q=%s&fields=key,title,author_name,first_publish_year,ia&limit=%d", baseURL, query, limit)
	if opts.Page > 0 {
		url += fmt.Sprintf("&offset=%d", opts.Page*limit)
	}
	return url
}

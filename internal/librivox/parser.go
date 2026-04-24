package librivox

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/source"
)

type apiResponse struct {
	Books []apiBook `json:"books"`
}

type apiBook struct {
	ID            string       `json:"id"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	URLLibrivox   string       `json:"url_librivox"`
	Language      string       `json:"language"`
	CopyrightYear string       `json:"copyright_year"`
	TotalTime     string       `json:"totaltime"`
	TotalTimeSecs int          `json:"totaltimesecs"`
	NumSections   string       `json:"num_sections"`
	Authors       []apiAuthor  `json:"authors"`
	Sections      []apiSection `json:"sections"`
}

type apiAuthor struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type apiSection struct {
	ID            string      `json:"id"`
	SectionNumber string      `json:"section_number"`
	Title         string      `json:"title"`
	ListenURL     string      `json:"listen_url"`
	Language      string      `json:"language"`
	PlayTime      string      `json:"playtime"`
	Readers       []apiReader `json:"readers"`
}

type apiReader struct {
	DisplayName string `json:"display_name"`
}

func (b *apiBook) toAudiobook() *source.Audiobook {
	author := ""
	if len(b.Authors) > 0 {
		a := b.Authors[0]
		author = strings.TrimSpace(a.FirstName + " " + a.LastName)
	}
	narrator := ""
	if len(b.Sections) > 0 && len(b.Sections[0].Readers) > 0 {
		narrator = b.Sections[0].Readers[0].DisplayName
	}
	numSections, _ := strconv.Atoi(b.NumSections)
	return &source.Audiobook{
		ID: b.ID, Title: b.Title, Author: author, Narrator: narrator,
		Description: b.Description, Language: b.Language, Year: b.CopyrightYear,
		Duration: time.Duration(b.TotalTimeSecs) * time.Second,
		PageURL: b.URLLibrivox, Format: "mp3", ChapterCount: numSections, Source: "librivox",
	}
}

func (s *apiSection) toChapter() *source.Chapter {
	idx, _ := strconv.Atoi(s.SectionNumber)
	return &source.Chapter{
		Index: idx, Title: s.Title, Duration: parsePlaytime(s.PlayTime),
		DownloadURL: s.ListenURL, Format: "mp3",
	}
}

func parsePlaytime(s string) time.Duration {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sec, _ := strconv.Atoi(parts[2])
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}

func buildSearchURL(baseURL, query string, opts source.SearchOptions) string {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	url := fmt.Sprintf("%s/api/feed/audiobooks/?title=%s&format=json&extended=1&limit=%d", baseURL, query, limit)
	if opts.Author != "" {
		url += "&author=" + opts.Author
	}
	if opts.Page > 0 {
		url += fmt.Sprintf("&offset=%d", opts.Page*limit)
	}
	return url
}

func buildGetURL(baseURL, bookID string) string {
	return fmt.Sprintf("%s/api/feed/audiobooks/?id=%s&format=json&extended=1", baseURL, bookID)
}

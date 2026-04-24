package librivox

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/source"
)

// ── JSON / API types ────────────────────────────────────────────────────────

type apiResponse struct {
	XMLName xml.Name  `json:"-" xml:"xml"`
	Error   string    `json:"error" xml:"error"`
	Books   []apiBook `json:"books" xml:"books>book"`
}

type apiBook struct {
	ID            string       `json:"id"            xml:"id"`
	Title         string       `json:"title"         xml:"title"`
	Description   string       `json:"description"   xml:"description"`
	URLLibrivox   string       `json:"url_librivox"  xml:"url_librivox"`
	Language      string       `json:"language"      xml:"language"`
	CopyrightYear string       `json:"copyright_year" xml:"copyright_year"`
	TotalTime     string       `json:"totaltime"     xml:"totaltime"`
	TotalTimeSecs int          `json:"totaltimesecs" xml:"totaltimesecs"`
	NumSections   string       `json:"num_sections"  xml:"num_sections"`
	Authors       []apiAuthor  `json:"authors"       xml:"authors>author"`
	Sections      []apiSection `json:"sections"      xml:"sections>section"`
}

type apiAuthor struct {
	ID        string `json:"id"         xml:"id"`
	FirstName string `json:"first_name" xml:"first_name"`
	LastName  string `json:"last_name"  xml:"last_name"`
}

type apiSection struct {
	ID            string      `json:"id"             xml:"id"`
	SectionNumber string      `json:"section_number" xml:"section_number"`
	Title         string      `json:"title"          xml:"title"`
	ListenURL     string      `json:"listen_url"     xml:"listen_url"`
	Language      string      `json:"language"       xml:"language"`
	PlayTime      string      `json:"playtime"       xml:"playtime"`
	Readers       []apiReader `json:"readers"        xml:"readers>reader"`
}

type apiReader struct {
	DisplayName string `json:"display_name" xml:"display_name"`
}

// ── RSS types ───────────────────────────────────────────────────────────────

type rssResponse struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title    string    `xml:"title"`
	Link     string    `xml:"link"`
	ImageURL string    `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd image"`
	Items    []rssItem `xml:"item"`
}

type rssItem struct {
	Title     string       `xml:"title"`
	Episode   string       `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd episode"`
	Duration  string       `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd duration"`
	Enclosure rssEnclosure `xml:"enclosure"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

// ── Conversions ─────────────────────────────────────────────────────────────

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

// ── RSS parsing ─────────────────────────────────────────────────────────────

// parseRSS parses a LibriVox RSS feed body and returns chapters.
func parseRSS(body []byte) []*source.Chapter {
	var feed rssResponse
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil
	}
	chapters := make([]*source.Chapter, 0, len(feed.Channel.Items))
	for i, item := range feed.Channel.Items {
		if item.Enclosure.URL == "" {
			continue
		}
		idx := i + 1
		if ep, err := strconv.Atoi(item.Episode); err == nil && ep > 0 {
			idx = ep
		}
		chapters = append(chapters, &source.Chapter{
			Index:       idx,
			Title:       item.Title,
			Duration:    parsePlaytime(item.Duration),
			DownloadURL: item.Enclosure.URL,
			Format:      "mp3",
		})
	}
	return chapters
}

// parseRSSMetadata extracts audiobook metadata from a LibriVox RSS feed.
// RSS title format: "Title by Author Name (year - year)"
func parseRSSMetadata(body []byte, bookID string) *source.Audiobook {
	var feed rssResponse
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil
	}

	raw := feed.Channel.Title
	if raw == "" {
		return nil
	}

	// Parse "Christmas Carol, A by Charles Dickens (1812 - 1870)"
	title := raw
	author := ""
	if idx := strings.Index(raw, " by "); idx >= 0 {
		title = strings.TrimSpace(raw[:idx])
		rest := raw[idx+4:]
		// Strip "(year - year)" suffix
		if paren := strings.LastIndex(rest, "("); paren >= 0 {
			author = strings.TrimSpace(rest[:paren])
		} else {
			author = strings.TrimSpace(rest)
		}
	}

	return &source.Audiobook{
		ID:      bookID,
		Title:   title,
		Author:  author,
		PageURL: feed.Channel.Link,
		Format:  "mp3",
		Source:  "librivox",
	}
}

// ── URL builders ─────────────────────────────────────────────────────────────

func buildSearchURL(baseURL, query string, opts source.SearchOptions) string {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	// LibriVox API only supports XML. The `author=` param works reliably,
	// while `title=` often returns errors. We use `author=` as the primary
	// search and add `title=` as a secondary filter when an author filter is set.
	u := fmt.Sprintf("%s/api/feed/audiobooks?author=%s&limit=%d",
		baseURL, url.QueryEscape(query), limit)
	if opts.Author != "" {
		// If user explicitly set author filter, search by that and use query as title
		u = fmt.Sprintf("%s/api/feed/audiobooks?author=%s&title=%s&limit=%d",
			baseURL, url.QueryEscape(opts.Author), url.QueryEscape(query), limit)
	}
	if opts.Page > 0 {
		u += fmt.Sprintf("&offset=%d", opts.Page*limit)
	}
	return u
}

func buildGetURL(baseURL, bookID string) string {
	return fmt.Sprintf("%s/api/feed/audiobooks?id=%s", baseURL, url.QueryEscape(bookID))
}

func buildRSSURL(baseURL, bookID string) string {
	return fmt.Sprintf("%s/rss/%s", baseURL, url.QueryEscape(bookID))
}

// ── Helpers ──────────────────────────────────────────────────────────────────

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

package loyalbooks

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/billmal071/audbookdl/internal/source"
)

type bookListing struct {
	Slug   string
	Title  string
	Author string
}

func parseSearchPage(doc *goquery.Document) []bookListing {
	// Strategy 1: Original selector (table.layout2-blue)
	books := parseSearchPageStrategy1(doc)
	if len(books) > 0 {
		return books
	}

	// Strategy 2: div-based layout with /book/ and /author/ links
	books = parseSearchPageStrategy2(doc)
	if len(books) > 0 {
		return books
	}

	// Strategy 3: Generic — find all /book/ links
	return parseSearchPageGeneric(doc)
}

func parseSearchPageStrategy1(doc *goquery.Document) []bookListing {
	var books []bookListing
	doc.Find("table.layout2-blue tr").Each(func(i int, s *goquery.Selection) {
		// Find the first book-link anchor that has non-empty text (skip image-only anchors).
		var titleLink *goquery.Selection
		var href string
		s.Find("td.layout2 a[href^='/book/']").Each(func(_ int, a *goquery.Selection) {
			if titleLink != nil {
				return
			}
			t := strings.TrimSpace(a.Text())
			if t == "" {
				return
			}
			h, exists := a.Attr("href")
			if !exists || h == "" {
				return
			}
			titleLink = a
			href = h
		})
		if titleLink == nil || href == "" {
			return
		}
		title := strings.TrimSpace(titleLink.Text())
		if title == "" {
			return
		}
		author := ""
		s.Find("td.layout2 a[href^='/author/']").Each(func(_ int, a *goquery.Selection) {
			author = strings.TrimSpace(a.Text())
		})
		slug := extractSlug(href)
		if slug != "" {
			books = append(books, bookListing{Slug: slug, Title: title, Author: author})
		}
	})
	return books
}

func parseSearchPageStrategy2(doc *goquery.Document) []bookListing {
	var books []bookListing
	// Try results in div-based layout
	doc.Find("div.result, div.book-item, .book-list-item, .s-result-item").Each(func(i int, s *goquery.Selection) {
		bookLink := s.Find("a[href*='/book/']").First()
		href, exists := bookLink.Attr("href")
		if !exists {
			return
		}
		title := strings.TrimSpace(bookLink.Text())
		if title == "" {
			return
		}

		author := ""
		s.Find("a[href*='/author/']").Each(func(_ int, a *goquery.Selection) {
			author = strings.TrimSpace(a.Text())
		})

		slug := extractSlug(href)
		if slug != "" {
			books = append(books, bookListing{Slug: slug, Title: title, Author: author})
		}
	})
	return books
}

func parseSearchPageGeneric(doc *goquery.Document) []bookListing {
	var books []bookListing
	seen := make(map[string]bool)
	doc.Find("a[href*='/book/']").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		title := strings.TrimSpace(s.Text())
		if title == "" || len(title) < 3 {
			return // Skip image-only links
		}
		slug := extractSlug(href)
		if slug == "" || seen[slug] {
			return
		}
		seen[slug] = true
		books = append(books, bookListing{Slug: slug, Title: title})
	})
	return books
}

func extractSlug(href string) string {
	// Handle both /book/slug and full URLs
	idx := strings.Index(href, "/book/")
	if idx < 0 {
		return ""
	}
	slug := href[idx+6:]
	// Remove trailing slash or query params
	if i := strings.IndexAny(slug, "?#/"); i >= 0 {
		slug = slug[:i]
	}
	return slug
}

func parseBookPage(doc *goquery.Document) []*source.Chapter {
	// Strategy 1: Original selector (table.chapter-download)
	chapters := parseBookPageTable(doc)
	if len(chapters) > 0 {
		return chapters
	}

	// Strategy 2: Find any MP3 links on the page
	return parseBookPageLinks(doc)
}

func parseBookPageTable(doc *goquery.Document) []*source.Chapter {
	var chapters []*source.Chapter
	doc.Find("table.chapter-download tr").Each(func(i int, s *goquery.Selection) {
		cells := s.Find("td")
		if cells.Length() < 2 {
			return
		}
		link := cells.Eq(1).Find("a")
		href, exists := link.Attr("href")
		if !exists {
			return
		}
		title := strings.TrimSpace(link.Text())
		idx := i + 1
		idxText := strings.TrimSpace(cells.Eq(0).Text())
		if n, err := strconv.Atoi(idxText); err == nil {
			idx = n
		}
		var duration time.Duration
		if cells.Length() >= 3 {
			duration = parseDuration(strings.TrimSpace(cells.Eq(2).Text()))
		}
		chapters = append(chapters, &source.Chapter{
			Index: idx, Title: title, Duration: duration,
			DownloadURL: href, Format: "mp3",
		})
	})
	return chapters
}

func parseBookPageLinks(doc *goquery.Document) []*source.Chapter {
	var chapters []*source.Chapter
	idx := 1
	doc.Find("a[href$='.mp3']").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		title := strings.TrimSpace(s.Text())
		if title == "" {
			title = fmt.Sprintf("Chapter %d", idx)
		}
		chapters = append(chapters, &source.Chapter{
			Index: idx, Title: title, DownloadURL: href, Format: "mp3",
		})
		idx++
	})
	return chapters
}

func parseDuration(s string) time.Duration {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		m, _ := strconv.Atoi(parts[0])
		sec, _ := strconv.Atoi(parts[1])
		return time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
	case 3:
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
	default:
		return 0
	}
}

func buildSearchURL(baseURL, query string) string {
	return fmt.Sprintf("%s/search?q=%s", baseURL, url.QueryEscape(query))
}

func buildBookURL(baseURL, slug string) string {
	return fmt.Sprintf("%s/book/%s", baseURL, slug)
}

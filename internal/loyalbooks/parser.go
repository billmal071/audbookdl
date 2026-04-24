package loyalbooks

import (
	"fmt"
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
		slug := strings.TrimPrefix(href, "/book/")
		books = append(books, bookListing{Slug: slug, Title: title, Author: author})
	})
	return books
}

func parseBookPage(doc *goquery.Document) []*source.Chapter {
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
	return fmt.Sprintf("%s/search?q=%s", baseURL, query)
}

func buildBookURL(baseURL, slug string) string {
	return fmt.Sprintf("%s/book/%s", baseURL, slug)
}

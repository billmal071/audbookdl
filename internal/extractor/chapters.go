package extractor

import (
	"fmt"
	"regexp"
	"strings"
)

var chapterPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^(Chapter\s+\d+[^\n]*)$`),
	regexp.MustCompile(`(?m)^(CHAPTER\s+[IVXLCDM]+[^\n]*)$`),
	regexp.MustCompile(`(?m)^(PART\s+[A-Z]+[^\n]*)$`),
	regexp.MustCompile(`(?m)^(\d+\.\s+[^\n]+)$`),
}

const fallbackWordLimit = 5000

func DetectChapters(text string) []Chapter {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	for _, pat := range chapterPatterns {
		chapters := splitByPattern(text, pat)
		if len(chapters) >= 2 {
			return chapters
		}
	}
	return splitByWordCount(text, fallbackWordLimit)
}

func splitByPattern(text string, pat *regexp.Regexp) []Chapter {
	locs := pat.FindAllStringIndex(text, -1)
	if len(locs) < 2 {
		return nil
	}
	var chapters []Chapter
	for i, loc := range locs {
		title := strings.TrimSpace(text[loc[0]:loc[1]])
		var body string
		if i+1 < len(locs) {
			body = text[loc[1]:locs[i+1][0]]
		} else {
			body = text[loc[1]:]
		}
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		chapters = append(chapters, Chapter{
			Index: len(chapters) + 1,
			Title: title,
			Text:  body,
		})
	}
	return chapters
}

func splitByWordCount(text string, wordLimit int) []Chapter {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var chapters []Chapter
	for i := 0; i < len(words); i += wordLimit {
		end := i + wordLimit
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ")
		idx := len(chapters) + 1
		chapters = append(chapters, Chapter{
			Index: idx,
			Title: fmt.Sprintf("Part %d", idx),
			Text:  chunk,
		})
	}
	return chapters
}

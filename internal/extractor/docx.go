// internal/extractor/docx.go
package extractor

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// docxBody represents the body of a DOCX document.xml.
type docxDocument struct {
	Body docxBody `xml:"body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text string `xml:"t"`
}

func extractDOCX(filePath string) (*Book, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	defer r.Close()

	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return nil, fmt.Errorf("word/document.xml not found in docx")
	}

	rc, err := docFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open document.xml: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read document.xml: %w", err)
	}

	var doc docxDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse document.xml: %w", err)
	}

	var paragraphs []string
	for _, p := range doc.Body.Paragraphs {
		var parts []string
		for _, r := range p.Runs {
			if r.Text != "" {
				parts = append(parts, r.Text)
			}
		}
		line := strings.Join(parts, "")
		if line != "" {
			paragraphs = append(paragraphs, line)
		}
	}

	text := strings.Join(paragraphs, "\n\n")
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	chapters := DetectChapters(text)
	if len(chapters) == 0 {
		chapters = []Chapter{{Index: 1, Title: title, Text: strings.TrimSpace(text)}}
	}

	return &Book{
		Title:    title,
		Author:   "Unknown",
		Chapters: chapters,
	}, nil
}

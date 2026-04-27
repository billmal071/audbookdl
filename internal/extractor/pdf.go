// internal/extractor/pdf.go
package extractor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gopdf "github.com/ledongthuc/pdf"
)

func extractPDF(filePath string) (*Book, error) {
	// Try pdftotext first (better quality).
	text, err := extractPDFWithPdftotext(filePath)
	if err != nil {
		// Fall back to pure Go.
		text, err = extractPDFWithGoLib(filePath)
		if err != nil {
			return nil, fmt.Errorf("extract pdf: %w", err)
		}
	}

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

func extractPDFWithPdftotext(filePath string) (string, error) {
	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("pdftotext not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, pdftotext, "-layout", filePath, "-")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return out.String(), nil
}

func extractPDFWithGoLib(filePath string) (string, error) {
	f, r, err := gopdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}

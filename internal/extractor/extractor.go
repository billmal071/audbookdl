// internal/extractor/extractor.go
package extractor

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Extract reads an ebook file and returns a Book with chapters.
// Supported formats: .txt, .pdf, .epub, .docx
func Extract(filePath string) (*Book, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".txt":
		return extractTXT(filePath)
	case ".pdf":
		return extractPDF(filePath)
	case ".epub":
		return extractEPUB(filePath)
	case ".docx":
		return extractDOCX(filePath)
	default:
		return nil, fmt.Errorf("unsupported format: %s (supported: .txt, .pdf, .epub, .docx)", ext)
	}
}

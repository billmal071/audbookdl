// internal/extractor/epub.go
package extractor

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// opfPackage represents the OPF package document.
type opfPackage struct {
	Metadata opfMetadata `xml:"metadata"`
	Spine    opfSpine    `xml:"spine"`
	Manifest opfManifest `xml:"manifest"`
}

type opfMetadata struct {
	Title   string `xml:"title"`
	Creator string `xml:"creator"`
}

type opfSpine struct {
	ItemRefs []opfItemRef `xml:"itemref"`
}

type opfItemRef struct {
	IDRef string `xml:"idref,attr"`
}

type opfManifest struct {
	Items []opfItem `xml:"item"`
}

type opfItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

// containerXML locates the OPF file inside the EPUB.
type containerXML struct {
	Rootfiles []rootfile `xml:"rootfiles>rootfile"`
}

type rootfile struct {
	FullPath string `xml:"full-path,attr"`
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func extractEPUB(filePath string) (*Book, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer r.Close()

	// Find OPF path from META-INF/container.xml.
	opfPath, err := findOPFPath(r)
	if err != nil {
		return nil, err
	}

	// Parse OPF.
	opfDir := filepath.Dir(opfPath)
	pkg, err := parseOPF(r, opfPath)
	if err != nil {
		return nil, err
	}

	// Build href map from manifest.
	hrefByID := make(map[string]string)
	for _, item := range pkg.Manifest.Items {
		hrefByID[item.ID] = item.Href
	}

	// Read spine items in order.
	var chapters []Chapter
	for _, itemRef := range pkg.Spine.ItemRefs {
		href, ok := hrefByID[itemRef.IDRef]
		if !ok {
			continue
		}
		fullHref := href
		if opfDir != "." {
			fullHref = opfDir + "/" + href
		}
		text, err := readZipFileText(r, fullHref)
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		title := filepath.Base(href)
		title = strings.TrimSuffix(title, filepath.Ext(title))
		chapters = append(chapters, Chapter{
			Index: len(chapters) + 1,
			Title: title,
			Text:  text,
		})
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("no text content found in epub")
	}

	return &Book{
		Title:    pkg.Metadata.Title,
		Author:   pkg.Metadata.Creator,
		Chapters: chapters,
	}, nil
}

func findOPFPath(r *zip.ReadCloser) (string, error) {
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			var c containerXML
			if err := xml.NewDecoder(rc).Decode(&c); err != nil {
				return "", fmt.Errorf("parse container.xml: %w", err)
			}
			if len(c.Rootfiles) > 0 {
				return c.Rootfiles[0].FullPath, nil
			}
			return "", fmt.Errorf("no rootfile in container.xml")
		}
	}
	return "", fmt.Errorf("META-INF/container.xml not found")
}

func parseOPF(r *zip.ReadCloser, path string) (*opfPackage, error) {
	for _, f := range r.File {
		if f.Name == path {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			var pkg opfPackage
			if err := xml.NewDecoder(rc).Decode(&pkg); err != nil {
				return nil, fmt.Errorf("parse opf: %w", err)
			}
			return &pkg, nil
		}
	}
	return nil, fmt.Errorf("opf file not found: %s", path)
}

func readZipFileText(r *zip.ReadCloser, name string) (string, error) {
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			return stripHTML(string(data)), nil
		}
	}
	return "", fmt.Errorf("file not found in epub: %s", name)
}

func stripHTML(s string) string {
	text := htmlTagRegex.ReplaceAllString(s, " ")
	// Collapse whitespace.
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

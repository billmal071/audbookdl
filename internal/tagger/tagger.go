package tagger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
	id3 "github.com/bogem/id3v2/v2"
)

// TagResult contains the result of tagging an audiobook.
type TagResult struct {
	TaggedFiles int
	CoverSaved  bool
	Errors      []error
}

// TagAudiobook writes ID3v2 metadata tags to all chapter files and downloads cover art.
func TagAudiobook(ctx context.Context, book *source.Audiobook, chapters []*source.Chapter, baseDir string) *TagResult {
	result := &TagResult{}

	// Download cover art if available
	var coverData []byte
	if book.CoverURL != "" {
		coverPath := filepath.Join(baseDir, book.Author, book.Title, "cover.jpg")
		if err := downloadCover(ctx, book.CoverURL, coverPath); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("cover art: %w", err))
		} else {
			result.CoverSaved = true
			coverData, _ = os.ReadFile(coverPath)
		}
	}

	// Write ID3 tags to each chapter file
	for i, ch := range chapters {
		filePath := filepath.Join(baseDir, book.Author, book.Title,
			fmt.Sprintf("%02d - %s.%s", ch.Index, ch.Title, ch.Format))
		if _, err := os.Stat(filePath); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("chapter %d missing: %w", ch.Index, err))
			continue
		}

		if err := writeID3Tags(filePath, book, ch, i+1, len(chapters), coverData); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("chapter %d tagging: %w", ch.Index, err))
			continue
		}

		result.TaggedFiles++
	}

	return result
}

// writeID3Tags writes ID3v2 tags to an MP3 file using github.com/bogem/id3v2/v2.
func writeID3Tags(filePath string, book *source.Audiobook, chapter *source.Chapter, trackNum, totalTracks int, coverData []byte) error {
	tag, err := id3.Open(filePath, id3.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("open id3 tag: %w", err)
	}
	defer tag.Close()

	tag.SetDefaultEncoding(id3.EncodingUTF8)

	// Core metadata frames
	tag.SetTitle(chapter.Title)
	tag.SetAlbum(book.Title)
	tag.SetArtist(book.Author)
	tag.SetGenre("Audiobook")

	if book.Year != "" {
		tag.SetYear(book.Year)
	}

	// Album artist (TPE2)
	tag.AddTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"), id3.EncodingUTF8, book.Author)

	// Narrator stored in Composer (TCOM) — widely supported in audiobook players
	if book.Narrator != "" {
		tag.AddTextFrame(tag.CommonID("Composer"), id3.EncodingUTF8, book.Narrator)
	}

	// Track number as "N/Total"
	if totalTracks > 0 {
		track := fmt.Sprintf("%d/%d", trackNum, totalTracks)
		tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3.EncodingUTF8, track)
	}

	// Embed cover art as front cover (APIC frame)
	if len(coverData) > 0 {
		pic := id3.PictureFrame{
			Encoding:    id3.EncodingUTF8,
			MimeType:    "image/jpeg",
			PictureType: id3.PTFrontCover,
			Description: "Cover",
			Picture:     coverData,
		}
		tag.AddAttachedPicture(pic)
	}

	return tag.Save()
}

func downloadCover(ctx context.Context, url, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	http := httpclient.New()
	body, err := http.GetBody(ctx, url)
	if err != nil {
		return fmt.Errorf("download cover: %w", err)
	}

	return os.WriteFile(destPath, body, 0644)
}

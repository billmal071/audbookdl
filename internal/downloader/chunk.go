package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type ProgressFunc func(downloaded int64)

var httpClient = &http.Client{
	Timeout: 0,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
		MaxIdleConnsPerHost: 5,
	},
}

func DownloadFile(ctx context.Context, url, destPath string, progressFn ProgressFunc) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Check for existing partial file to support resume.
	var existingSize int64
	if info, err := os.Stat(destPath); err == nil {
		existingSize = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "audbookdl/1.0")

	// Request range from where we left off.
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	var file *os.File
	var downloaded int64

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Server supports range — append to existing file.
		file, err = os.OpenFile(destPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("open file for append: %w", err)
		}
		downloaded = existingSize
	case http.StatusOK:
		// Server doesn't support range or this is a fresh download — start from scratch.
		file, err = os.Create(destPath)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
		downloaded = 0
	default:
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	defer file.Close()

	if progressFn != nil {
		progressFn(downloaded)
	}

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write file: %w", writeErr)
			}
			downloaded += int64(n)
			if progressFn != nil {
				progressFn(downloaded)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read body: %w", readErr)
		}
	}
	return nil
}

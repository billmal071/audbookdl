package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	userAgent  string
}

type Option func(*Client)

func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

func New(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) GetJSON(ctx context.Context, url string, dst interface{}) error {
	body, err := c.GetBody(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode JSON from %s: %w", url, err)
	}
	return nil
}

func (c *Client) GetBody(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body from %s: %w", url, err)
	}
	return body, nil
}

package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.langsearch.com"
	defaultTimeout = 30 * time.Second
	maxRetries     = 3
)

// Client talks to LangSearch APIs with shared auth and 429 retry.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	cache      *ttlCache
	sleep      func(time.Duration)
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the underlying HTTP client (for tests).
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		cl.httpClient = c
	}
}

// WithBaseURL overrides the LangSearch API base URL (for tests).
func WithBaseURL(base string) Option {
	return func(cl *Client) {
		cl.baseURL = strings.TrimRight(base, "/")
	}
}

// WithSleep overrides sleep used for backoff (for tests).
func WithSleep(fn func(time.Duration)) Option {
	return func(cl *Client) {
		cl.sleep = fn
	}
}

// WithCacheTTL overrides cache TTL (for tests).
func WithCacheTTL(ttl time.Duration) Option {
	return func(cl *Client) {
		cl.cache = newTTLCache(ttl, defaultCacheMax)
	}
}

// NewClient creates a LangSearch client. apiKey may be empty; callers must check before use.
func NewClient(apiKey string, opts ...Option) *Client {
	cl := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		cache: newTTLCache(defaultCacheTTL, defaultCacheMax),
		sleep: time.Sleep,
	}
	for _, opt := range opts {
		opt(cl)
	}
	return cl
}

// Configured reports whether an API key is present.
func (c *Client) Configured() bool {
	return c != nil && strings.TrimSpace(c.apiKey) != ""
}

func (c *Client) postJSON(ctx context.Context, path string, body []byte) ([]byte, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("LANGSEARCH_API_KEY is required")
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 200 * time.Millisecond
			jitter := time.Duration(rand.Intn(100)) * time.Millisecond
			c.sleep(backoff + jitter)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("langsearch request: %w", err)
		}

		raw, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf(
				"langsearch rate limited (HTTP 429): free tier is ~1 req/s, 60/min, 1000/day; wait and retry, or rely on cache. %s",
				trimBody(raw),
			)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("langsearch HTTP %d: %s", resp.StatusCode, trimBody(raw))
		}
		return raw, nil
	}
	return nil, lastErr
}

func trimBody(b []byte) string {
	const max = 200
	s := string(bytes.TrimSpace(b))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

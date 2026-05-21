package omnitun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultBaseURL    = "https://api.omnitun.dev"
	defaultUserAgent  = "omnitun-go"
	defaultRetries    = 3
	defaultRetryDelay = 500 * time.Millisecond
)

type ClientOption func(*Client)

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) {
		c.UserAgent = userAgent
	}
}

func WithRateLimit(ratePerSecond float64) ClientOption {
	return func(c *Client) {
		c.rateMu.Lock()
		defer c.rateMu.Unlock()
		c.ratePerSecond = ratePerSecond
		c.rateLastCall = time.Time{}
	}
}

type Client struct {
	Token      string
	BaseURL    string
	UserAgent  string
	httpClient *http.Client

	rateMu        sync.Mutex
	ratePerSecond float64
	rateLastCall  time.Time

	Tunnels  *TunnelsService
	Domains  *DomainsService
	Networks *NetworksService
}

func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		Token:     token,
		BaseURL:   defaultBaseURL,
		UserAgent: defaultUserAgent,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	c.Tunnels = &TunnelsService{client: c}
	c.Domains = &DomainsService{client: c}
	c.Networks = &NetworksService{client: c}
	return c
}

func (c *Client) waitRateLimit() {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	if c.ratePerSecond <= 0 {
		return
	}
	if !c.rateLastCall.IsZero() {
		elapsed := time.Since(c.rateLastCall)
		interval := time.Duration(float64(time.Second) / c.ratePerSecond)
		if elapsed < interval {
			time.Sleep(interval - elapsed)
		}
	}
	c.rateLastCall = time.Now()
}

func (c *Client) request(ctx context.Context, method, path string, body, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	requestURL, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return fmt.Errorf("build request url: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < defaultRetries; attempt++ {
		c.waitRateLimit()

		req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < defaultRetries-1 {
				time.Sleep(defaultRetryDelay * time.Duration(attempt+1))
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited")
			if attempt < defaultRetries-1 {
				retryAfter := resp.Header.Get("Retry-After")
				if d, err := time.ParseDuration(retryAfter + "s"); err == nil {
					time.Sleep(d)
				} else {
					time.Sleep(defaultRetryDelay * time.Duration(attempt+1))
				}
			}
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			if attempt < defaultRetries-1 {
				time.Sleep(defaultRetryDelay * time.Duration(attempt+1))
			}
			continue
		}

		if resp.StatusCode >= 400 {
			var apiErr APIError
			if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
				apiErr = APIError{
					Code:    fmt.Sprintf("HTTP_%d", resp.StatusCode),
					Message: resp.Status,
				}
			}
			return fmt.Errorf("api error [%s]: %s", apiErr.Code, apiErr.Message)
		}

		if result != nil && resp.StatusCode != http.StatusNoContent {
			if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("request failed after %d retries: %w", defaultRetries, lastErr)
}

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	return c.request(ctx, http.MethodGet, path, nil, result)
}

func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
	return c.request(ctx, http.MethodPost, path, body, result)
}

func (c *Client) put(ctx context.Context, path string, body, result interface{}) error {
	return c.request(ctx, http.MethodPut, path, body, result)
}

func (c *Client) patch(ctx context.Context, path string, body, result interface{}) error {
	return c.request(ctx, http.MethodPatch, path, body, result)
}

func (c *Client) delete(ctx context.Context, path string, result interface{}) error {
	return c.request(ctx, http.MethodDelete, path, nil, result)
}

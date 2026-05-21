package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	endpoint   string
	httpClient *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint:   strings.TrimSuffix(endpoint, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Exec(ctx context.Context, query string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, strings.NewReader(query))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clickhouse error (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) Query(ctx context.Context, query string) ([]map[string]interface{}, error) {
	fullQuery := query
	if !strings.Contains(strings.ToUpper(query), "FORMAT") {
		fullQuery = query + " FORMAT JSONEachRow"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, strings.NewReader(fullQuery))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clickhouse error (status %d): %s", resp.StatusCode, string(body))
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	results := make([]map[string]interface{}, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var row map[string]interface{}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parse row: %w", err)
		}
		results = append(results, row)
	}
	return results, nil
}

func EscapeString(s string) string {
	return strings.ReplaceAll(s, "\\", "\\\\")
}

package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) PostJSON(ctx context.Context, path string, payload any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return fmt.Errorf("encode request payload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+normalizePath(path), &body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		if len(responseBody) == 0 {
			return fmt.Errorf("unexpected response status: %s", response.Status)
		}

		return fmt.Errorf("unexpected response status: %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 512))

	return nil
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}

	if strings.HasPrefix(path, "/") {
		return path
	}

	return "/" + path
}

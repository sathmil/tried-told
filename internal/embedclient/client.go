// Package embedclient is the Go side of the query-time embedding split:
// python/embed_service.py runs the PyTorch model (the one piece Go can't
// do itself), and this package calls it over HTTP to embed a live user
// query before searching internal/semantic.Index. See
// docs/design/27-query-embedding-service.md.
package embedclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client calls a running embed_service.py instance.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client that calls the embedding service at baseURL
// (e.g. "http://127.0.0.1:8091").
func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{}}
}

type embedRequest struct {
	Text string `json:"text"`
}

type embedResponse struct {
	Vector []float32 `json:"vector"`
	Error  string    `json:"error"`
}

// Embed sends query text to the embedding service and returns the
// resulting vector, already prefixed and normalized on the service side
// - the caller doesn't need to know BGE's query-instruction convention.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(embedRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("embedclient: marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedclient: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedclient: calling embed service: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embedclient: reading response: %w", err)
	}

	var parsed embedResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("embedclient: decoding response (status %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedclient: embed service returned %d: %s", resp.StatusCode, parsed.Error)
	}
	return parsed.Vector, nil
}

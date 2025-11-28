package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/santaclaude2025/confab/pkg/config"
)

const (
	// compressionThreshold is the minimum payload size to compress.
	// Below this, compression overhead isn't worth it.
	compressionThreshold = 1024 // 1KB
)

// ErrUnauthorized is returned when the server returns 401 or 403.
// This typically means the API key is invalid or expired.
var ErrUnauthorized = errors.New("unauthorized")

// Client is a configured HTTP client for making authenticated requests to the backend
type Client struct {
	cfg        *config.UploadConfig
	httpClient *http.Client
	encoder    *zstd.Encoder
}

// NewClient creates a new authenticated HTTP client
func NewClient(cfg *config.UploadConfig, timeout time.Duration) *Client {
	// Create zstd encoder with default compression level (good balance of speed/ratio)
	encoder, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		encoder: encoder,
	}
}

// DoJSON performs an HTTP request with JSON body and parses JSON response
// Automatically sets Content-Type, Authorization, and handles error responses.
// Payloads larger than 1KB are compressed with zstd.
func (c *Client) DoJSON(method, path string, reqBody, respBody interface{}) error {
	// Marshal request body if provided
	var bodyReader io.Reader
	var contentEncoding string

	if reqBody != nil {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		// Compress if payload is large enough
		if len(payload) >= compressionThreshold {
			compressed := c.encoder.EncodeAll(payload, make([]byte, 0, len(payload)/2))
			bodyReader = bytes.NewReader(compressed)
			contentEncoding = "zstd"
		} else {
			bodyReader = bytes.NewReader(payload)
		}
	}

	// Create request
	url := c.cfg.BackendURL + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
		if contentEncoding != "" {
			req.Header.Set("Content-Encoding", contentEncoding)
		}
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: status %d: %s", ErrUnauthorized, resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response if requested
	if respBody != nil {
		if err := json.Unmarshal(body, respBody); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// Get performs a GET request with JSON response parsing
func (c *Client) Get(path string, respBody interface{}) error {
	return c.DoJSON("GET", path, nil, respBody)
}

// Post performs a POST request with JSON body and response
func (c *Client) Post(path string, reqBody, respBody interface{}) error {
	return c.DoJSON("POST", path, reqBody, respBody)
}

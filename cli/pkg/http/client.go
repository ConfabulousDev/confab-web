package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/santaclaude2025/confab/pkg/config"
)

const (
	// compressionThreshold is the minimum payload size to compress.
	// Below this, compression overhead isn't worth it.
	compressionThreshold = 1024 // 1KB

	// Retry settings for rate limiting
	maxRetries       = 5
	initialBackoff   = 1 * time.Second
	maxBackoff       = 60 * time.Second
	backoffMultipler = 2.0
)

// ErrUnauthorized is returned when the server returns 401 or 403.
// This typically means the API key is invalid or expired.
var ErrUnauthorized = errors.New("unauthorized")

// ErrRateLimited is returned when retries are exhausted on 429 responses.
var ErrRateLimited = errors.New("rate limited")

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
// Retries with exponential backoff on 429 (rate limited) responses.
func (c *Client) DoJSON(method, path string, reqBody, respBody interface{}) error {
	// Marshal and compress request body once (for retries)
	var payload []byte
	var contentEncoding string

	if reqBody != nil {
		var err error
		payload, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		// Compress if payload is large enough
		if len(payload) >= compressionThreshold {
			payload = c.encoder.EncodeAll(payload, make([]byte, 0, len(payload)/2))
			contentEncoding = "zstd"
		}
	}

	url := c.cfg.BackendURL + path
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create fresh reader for each attempt
		var bodyReader io.Reader
		if payload != nil {
			bodyReader = bytes.NewReader(payload)
		}

		// Create request
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

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		// Handle rate limiting with retry
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == maxRetries {
				return fmt.Errorf("%w: exceeded %d retries", ErrRateLimited, maxRetries)
			}

			// Use Retry-After header if provided, otherwise use exponential backoff
			waitTime := backoff
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					waitTime = time.Duration(seconds) * time.Second
				}
			}

			time.Sleep(waitTime)

			// Exponential backoff for next attempt
			backoff = time.Duration(float64(backoff) * backoffMultipler)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Check other status codes
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

	return fmt.Errorf("%w: exceeded %d retries", ErrRateLimited, maxRetries)
}

// Get performs a GET request with JSON response parsing
func (c *Client) Get(path string, respBody interface{}) error {
	return c.DoJSON("GET", path, nil, respBody)
}

// Post performs a POST request with JSON body and response
func (c *Client) Post(path string, reqBody, respBody interface{}) error {
	return c.DoJSON("POST", path, reqBody, respBody)
}

package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
)

// Client is a configured HTTP client for making authenticated requests to the backend
type Client struct {
	cfg        *config.UploadConfig
	httpClient *http.Client
}

// NewClient creates a new authenticated HTTP client
func NewClient(cfg *config.UploadConfig, timeout time.Duration) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// DoJSON performs an HTTP request with JSON body and parses JSON response
// Automatically sets Content-Type, Authorization, and handles error responses
func (c *Client) DoJSON(method, path string, reqBody, respBody interface{}) error {
	// Marshal request body if provided
	var bodyReader io.Reader
	if reqBody != nil {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
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

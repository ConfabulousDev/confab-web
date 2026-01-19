package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
)

// TestClient provides HTTP client utilities for making authenticated requests
// to the test server.
type TestClient struct {
	*http.Client
	t         *testing.T
	ts        *TestServer
	apiKey    string         // For API key auth
	cookies   []*http.Cookie // For session auth
	csrfToken string         // CSRF token for state-changing requests
}

// NewTestClient creates a new test client for the given server.
func NewTestClient(t *testing.T, ts *TestServer) *TestClient {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}

	return &TestClient{
		Client: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
			// Don't follow redirects automatically - we want to check them
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		t:  t,
		ts: ts,
	}
}

// WithAPIKey returns a new client configured with API key authentication.
// The API key will be included in the Authorization header for all requests.
func (c *TestClient) WithAPIKey(apiKey string) *TestClient {
	return &TestClient{
		Client: c.Client,
		t:      c.t,
		ts:     c.ts,
		apiKey: apiKey,
	}
}

// WithSession returns a new client configured with session cookie authentication.
// The session token will be included as a cookie for all requests.
// This also fetches a CSRF token for state-changing requests.
func (c *TestClient) WithSession(sessionToken string) *TestClient {
	newClient := &TestClient{
		Client: c.Client,
		t:      c.t,
		ts:     c.ts,
		cookies: []*http.Cookie{{
			Name:  auth.SessionCookieName,
			Value: sessionToken,
		}},
	}

	// Fetch CSRF token by making a GET request (CSRF middleware sets cookie)
	csrfToken := newClient.fetchCSRFToken()
	newClient.csrfToken = csrfToken

	return newClient
}

// fetchCSRFToken makes a GET request to the CSRF token endpoint and stores cookies.
func (c *TestClient) fetchCSRFToken() string {
	// Use the dedicated CSRF token endpoint
	req, err := http.NewRequest("GET", c.ts.URL+"/api/v1/csrf-token", nil)
	if err != nil {
		return ""
	}

	// Add session cookies
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Store any cookies set by the CSRF middleware (needed for subsequent requests)
	// Only add cookies that aren't already in the list
	for _, newCookie := range resp.Cookies() {
		exists := false
		for _, existingCookie := range c.cookies {
			if existingCookie.Name == newCookie.Name {
				exists = true
				break
			}
		}
		if !exists {
			c.cookies = append(c.cookies, newCookie)
		}
	}

	// Get token from response header (X-CSRF-Token)
	token := resp.Header.Get("X-CSRF-Token")
	if token != "" {
		return token
	}

	// Fallback: try to get from response body
	var result struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		return result.CSRFToken
	}

	return ""
}

// Request makes an HTTP request to the test server.
// Body can be nil, a struct (will be JSON encoded), or an io.Reader.
func (c *TestClient) Request(method, path string, body interface{}) (*http.Response, error) {
	return c.RequestWithHeaders(method, path, body, nil)
}

// RequestWithHeaders makes an HTTP request with custom headers.
func (c *TestClient) RequestWithHeaders(method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	url := c.ts.URL + path

	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case io.Reader:
			bodyReader = v
		case []byte:
			bodyReader = bytes.NewReader(v)
		case string:
			bodyReader = bytes.NewReader([]byte(v))
		default:
			// Assume it's a struct, JSON encode it
			jsonBytes, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBytes)
		}
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Origin header for CSRF validation (must match trusted origins)
	req.Header.Set("Origin", "http://localhost:3000")
	// Set Sec-Fetch-Site header for the new filippo.io/csrf library
	// This header is required for browser-like CSRF validation
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	// Set Content-Type for requests with body
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add API key if configured
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Add session cookies if configured
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}

	// Add CSRF token header for state-changing requests (POST, PATCH, DELETE, PUT)
	// The CSRF cookie is already in c.cookies from fetchCSRFToken
	if c.csrfToken != "" && isStateChangingMethod(method) {
		req.Header.Set("X-CSRF-Token", c.csrfToken)
	}

	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.Client.Do(req)
}

// isStateChangingMethod returns true for HTTP methods that modify state
func isStateChangingMethod(method string) bool {
	return method == http.MethodPost ||
		method == http.MethodPatch ||
		method == http.MethodDelete ||
		method == http.MethodPut
}

// Get makes a GET request to the test server.
func (c *TestClient) Get(path string) (*http.Response, error) {
	return c.Request(http.MethodGet, path, nil)
}

// Post makes a POST request to the test server with a JSON body.
func (c *TestClient) Post(path string, body interface{}) (*http.Response, error) {
	return c.Request(http.MethodPost, path, body)
}

// Patch makes a PATCH request to the test server with a JSON body.
func (c *TestClient) Patch(path string, body interface{}) (*http.Response, error) {
	return c.Request(http.MethodPatch, path, body)
}

// Delete makes a DELETE request to the test server.
func (c *TestClient) Delete(path string) (*http.Response, error) {
	return c.Request(http.MethodDelete, path, nil)
}

// PostForm makes a POST request with form-encoded body (application/x-www-form-urlencoded).
// The formData should be a URL-encoded string like "code=ABCD-1234&field=value".
func (c *TestClient) PostForm(path string, formData string) (*http.Response, error) {
	return c.RequestWithHeaders(http.MethodPost, path, formData, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
}

// ParseJSON decodes the response body as JSON into v and closes the body.
func ParseJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("failed to decode response JSON: %v. Body: %s", err, string(body))
	}
}

// RequireStatus checks that the response has the expected status code.
// If not, it fails the test with the response body for debugging.
func RequireStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
	}
}

// APIKeyWithRawToken holds both the database key ID and the raw token.
// The raw token is needed for Authorization headers; the ID for database queries.
type APIKeyWithRawToken struct {
	ID       int64
	RawToken string
	Name     string
}

// CreateTestAPIKeyWithToken creates an API key and returns both the ID and raw token.
// This is useful for tests that need to make authenticated requests.
func CreateTestAPIKeyWithToken(t *testing.T, env *TestEnvironment, userID int64, name string) *APIKeyWithRawToken {
	t.Helper()

	// Generate a real API key
	rawKey, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("failed to generate API key: %v", err)
	}

	// Store in database
	keyID := CreateTestAPIKey(t, env, userID, keyHash, name)

	return &APIKeyWithRawToken{
		ID:       keyID,
		RawToken: rawKey,
		Name:     name,
	}
}

// CreateTestWebSessionWithToken creates a web session and returns the session token.
// This is useful for tests that need to make session-authenticated requests.
func CreateTestWebSessionWithToken(t *testing.T, env *TestEnvironment, userID int64) string {
	t.Helper()

	// Generate a random session ID (32 chars like production)
	sessionID := fmt.Sprintf("test-session-%d-%d", userID, time.Now().UnixNano())

	// Create session that expires in the future
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	CreateTestWebSession(t, env, sessionID, userID, expiresAt)

	return sessionID
}

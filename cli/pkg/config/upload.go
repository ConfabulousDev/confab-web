package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// UploadConfig holds cloud upload configuration
type UploadConfig struct {
	BackendURL string `json:"backend_url"`
	APIKey     string `json:"api_key"`
}

// GetUploadConfig reads upload configuration from ~/.confab/config.json
func GetUploadConfig() (*UploadConfig, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// Return default config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &UploadConfig{
			BackendURL: "",
			APIKey:     "",
		}, nil
	}

	// Read and parse config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config UploadConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// SaveUploadConfig writes upload configuration to ~/.confab/config.json
func SaveUploadConfig(config *UploadConfig) error {
	// Validate before saving
	if err := config.Validate(); err != nil {
		return err
	}

	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	confabDir := filepath.Dir(configPath)
	if err := os.MkdirAll(confabDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func getConfigPath() (string, error) {
	// Allow overriding config path for testing
	if testConfigPath := os.Getenv("CONFAB_CONFIG_PATH"); testConfigPath != "" {
		return testConfigPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".confab", "config.json"), nil
}

// ValidateBackendURL checks if the backend URL is valid
func ValidateBackendURL(backendURL string) error {
	if backendURL == "" {
		return nil // Empty is allowed (not configured)
	}

	parsed, err := url.Parse(backendURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Must have a scheme
	if parsed.Scheme == "" {
		return fmt.Errorf("url must include scheme (http:// or https://)")
	}

	// Only allow http and https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https, got %q", parsed.Scheme)
	}

	// Must have a host
	if parsed.Host == "" {
		return fmt.Errorf("url must include a host")
	}

	return nil
}

// ValidateAPIKey checks if the API key format is valid
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return nil // Empty is allowed (not configured)
	}

	// Minimum length check to catch truncated/corrupted keys
	const minKeyLength = 16
	if len(apiKey) < minKeyLength {
		return fmt.Errorf("api key too short (minimum %d characters)", minKeyLength)
	}

	// Check for obviously invalid characters (whitespace, control chars)
	if strings.ContainsAny(apiKey, " \t\n\r") {
		return fmt.Errorf("api key contains invalid whitespace characters")
	}

	return nil
}

// Validate checks if the upload config is valid
func (c *UploadConfig) Validate() error {
	if err := ValidateBackendURL(c.BackendURL); err != nil {
		return fmt.Errorf("invalid backend URL: %w", err)
	}

	if err := ValidateAPIKey(c.APIKey); err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}

	return nil
}

// EnsureAuthenticated reads the config and verifies it has valid credentials
// Returns the config if authenticated, or an error if not configured
func EnsureAuthenticated() (*UploadConfig, error) {
	cfg, err := GetUploadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	if cfg.BackendURL == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("not authenticated. Run 'confab login' first")
	}

	return cfg, nil
}

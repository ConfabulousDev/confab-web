package redactor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configFileName         = "redaction.json"
	disabledConfigFileName = "redaction.json.disabled"
)

// GetDefaultPatterns returns the default high-precision redaction patterns
func GetDefaultPatterns() []Pattern {
	return []Pattern{
		{
			Name:    "Anthropic API Key",
			Pattern: `sk-ant-api\d{2}-[A-Za-z0-9_-]{95}`,
			Type:    "api_key",
		},
		{
			Name:    "OpenAI API Key",
			Pattern: `sk-[A-Za-z0-9]{48}`,
			Type:    "api_key",
		},
		{
			Name:    "AWS Access Key",
			Pattern: `AKIA[0-9A-Z]{16}`,
			Type:    "aws_key",
		},
		{
			Name:    "AWS Secret Key",
			Pattern: `aws_secret_access_key\s*=\s*([A-Za-z0-9/+=]{40})`,
			Type:    "aws_secret",
			CaptureGroup: 1,
		},
		{
			Name:    "GitHub Personal Access Token",
			Pattern: `ghp_[A-Za-z0-9]{36}`,
			Type:    "github_token",
		},
		{
			Name:    "GitHub OAuth Token",
			Pattern: `gho_[A-Za-z0-9]{36}`,
			Type:    "github_token",
		},
		{
			Name:    "GitHub App Token",
			Pattern: `(ghu|ghs)_[A-Za-z0-9]{36}`,
			Type:    "github_token",
		},
		{
			Name:    "GitHub Refresh Token",
			Pattern: `ghr_[A-Za-z0-9]{36}`,
			Type:    "github_token",
		},
		{
			Name:    "JWT Token",
			Pattern: `eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`,
			Type:    "jwt",
		},
		{
			Name:    "RSA Private Key",
			Pattern: `-----BEGIN RSA PRIVATE KEY-----`,
			Type:    "private_key",
		},
		{
			Name:    "EC Private Key",
			Pattern: `-----BEGIN EC PRIVATE KEY-----`,
			Type:    "private_key",
		},
		{
			Name:    "OpenSSH Private Key",
			Pattern: `-----BEGIN OPENSSH PRIVATE KEY-----`,
			Type:    "private_key",
		},
		{
			Name:         "PostgreSQL Connection String Password",
			Pattern:      `(postgres(?:ql)?://[^:]+:)([^@\s]+)(@[^\s]+)`,
			Type:         "password",
			CaptureGroup: 2,
		},
		{
			Name:         "MySQL Connection String Password",
			Pattern:      `(mysql://[^:]+:)([^@\s]+)(@[^\s]+)`,
			Type:         "password",
			CaptureGroup: 2,
		},
		{
			Name:         "MongoDB Connection String Password",
			Pattern:      `(mongodb(?:\+srv)?://[^:]+:)([^@\s]+)(@[^\s]+)`,
			Type:         "password",
			CaptureGroup: 2,
		},
		{
			Name:         "Redis Connection String Password",
			Pattern:      `(redis://[^:/@\s]*:)([^@\s]+)(@[^\s]+)`,
			Type:         "password",
			CaptureGroup: 2,
		},
		{
			Name:         "Generic URL Password",
			Pattern:      `(://[^:/@\s]+:)([^@\s]+)(@)`,
			Type:         "password",
			CaptureGroup: 2,
		},
		{
			Name:    "Slack Token",
			Pattern: `xox[baprs]-[0-9a-zA-Z-]{10,72}`,
			Type:    "slack_token",
		},
		{
			Name:    "Stripe Live API Key",
			Pattern: `sk_live_[0-9a-zA-Z]{24,}`,
			Type:    "stripe_key",
		},
		{
			Name:    "Google API Key",
			Pattern: `AIza[0-9A-Za-z_-]{35}`,
			Type:    "google_api_key",
		},
		{
			Name:    "Twilio API Key",
			Pattern: `SK[0-9a-fA-F]{32}`,
			Type:    "twilio_key",
		},
		{
			Name:    "SendGrid API Key",
			Pattern: `SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}`,
			Type:    "sendgrid_key",
		},
		{
			Name:    "MailChimp API Key",
			Pattern: `[0-9a-f]{32}-us[0-9]{1,2}`,
			Type:    "mailchimp_key",
		},
		{
			Name:    "npm Access Token",
			Pattern: `npm_[A-Za-z0-9]{36}`,
			Type:    "npm_token",
		},
		{
			Name:    "PyPI Token",
			Pattern: `pypi-AgEIcHlwaS5vcmc[A-Za-z0-9_-]{70,}`,
			Type:    "pypi_token",
		},
	}
}

// GetConfigPath returns the path to the enabled config file
func GetConfigPath() string {
	return getConfigPathInDir(getConfabDir())
}

// GetDisabledConfigPath returns the path to the disabled config file
func GetDisabledConfigPath() string {
	return getDisabledConfigPathInDir(getConfabDir())
}

// LoadConfig loads the redaction config from the standard location
func LoadConfig() (Config, error) {
	return loadConfigFromPath(GetConfigPath())
}

// SaveConfig saves the redaction config to the standard location
func SaveConfig(config Config) error {
	return saveConfigToPath(GetConfigPath(), config)
}

// IsEnabled returns true if redaction is currently enabled
func IsEnabled() bool {
	return isEnabledInDir(getConfabDir())
}

// Enable enables redaction by renaming the config file
func Enable() error {
	return enableInDir(getConfabDir())
}

// Disable disables redaction by renaming the config file
func Disable() error {
	return disableInDir(getConfabDir())
}

// InitializeDefaultConfig creates a new config file with default patterns
// The file is created as disabled by default
func InitializeDefaultConfig() error {
	return initializeDefaultConfigToPath(GetDisabledConfigPath())
}

// --- Internal functions for testability ---

func getConfabDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".confab")
}

func getConfigPathInDir(dir string) string {
	return filepath.Join(dir, configFileName)
}

func getDisabledConfigPathInDir(dir string) string {
	return filepath.Join(dir, disabledConfigFileName)
}

func loadConfigFromPath(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

func saveConfigToPath(path string, config Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func isEnabledInDir(dir string) bool {
	enabledPath := getConfigPathInDir(dir)
	_, err := os.Stat(enabledPath)
	return err == nil
}

func enableInDir(dir string) error {
	disabledPath := getDisabledConfigPathInDir(dir)
	enabledPath := getConfigPathInDir(dir)

	// Check if disabled file exists
	if _, err := os.Stat(disabledPath); os.IsNotExist(err) {
		return fmt.Errorf("no disabled config file found at %s", disabledPath)
	}

	// Rename disabled to enabled
	if err := os.Rename(disabledPath, enabledPath); err != nil {
		return fmt.Errorf("failed to enable redaction: %w", err)
	}

	return nil
}

func disableInDir(dir string) error {
	enabledPath := getConfigPathInDir(dir)
	disabledPath := getDisabledConfigPathInDir(dir)

	// Check if enabled file exists
	if _, err := os.Stat(enabledPath); os.IsNotExist(err) {
		return fmt.Errorf("no enabled config file found at %s", enabledPath)
	}

	// Rename enabled to disabled
	if err := os.Rename(enabledPath, disabledPath); err != nil {
		return fmt.Errorf("failed to disable redaction: %w", err)
	}

	return nil
}

func initializeDefaultConfigToPath(path string) error {
	config := Config{
		Patterns: GetDefaultPatterns(),
	}

	return saveConfigToPath(path, config)
}

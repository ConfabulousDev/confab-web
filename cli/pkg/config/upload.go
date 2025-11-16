package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// UploadConfig holds cloud upload configuration
type UploadConfig struct {
	Enabled    bool   `json:"enabled"`
	BackendURL string `json:"backend_url"`
	APIKey     string `json:"api_key"`
}

// GetUploadConfig reads upload configuration from ~/.confab/config.json
func GetUploadConfig() (*UploadConfig, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// Return default disabled config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &UploadConfig{
			Enabled:    false,
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
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".confab", "config.json"), nil
}

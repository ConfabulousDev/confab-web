package redactor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig tests loading configuration from file
func TestLoadConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "redaction.json")

	// Test data
	testConfig := Config{
		Patterns: []Pattern{
			{
				Name:    "Test API Key",
				Pattern: `test-[0-9]+`,
				Type:    "api_key",
			},
		},
	}

	// Write test config
	data, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	loaded, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify
	if len(loaded.Patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(loaded.Patterns))
	}
	if loaded.Patterns[0].Name != "Test API Key" {
		t.Errorf("Expected pattern name 'Test API Key', got '%s'", loaded.Patterns[0].Name)
	}
}

// TestLoadConfigFileNotFound tests loading config when file doesn't exist
func TestLoadConfigFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.json")

	_, err := loadConfigFromPath(configPath)
	if err == nil {
		t.Error("Expected error when loading nonexistent config")
	}
}

// TestLoadConfigInvalidJSON tests loading config with invalid JSON
func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	if err := os.WriteFile(configPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := loadConfigFromPath(configPath)
	if err == nil {
		t.Error("Expected error when loading invalid JSON")
	}
}

// TestSaveConfig tests saving configuration to file
func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "redaction.json")

	testConfig := Config{
		Patterns: []Pattern{
			{
				Name:    "Test Pattern",
				Pattern: `test-\d+`,
				Type:    "api_key",
			},
		},
	}

	// Save config
	if err := saveConfigToPath(configPath, testConfig); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists and can be read
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal saved config: %v", err)
	}

	if len(loaded.Patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(loaded.Patterns))
	}
}

// TestIsEnabled tests checking if redaction is enabled
func TestIsEnabled(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		setupFile     string
		expectEnabled bool
	}{
		{
			name:          "Enabled - redaction.json exists",
			setupFile:     "redaction.json",
			expectEnabled: true,
		},
		{
			name:          "Disabled - redaction.json.disabled exists",
			setupFile:     "redaction.json.disabled",
			expectEnabled: false,
		},
		{
			name:          "Disabled - no file exists",
			setupFile:     "",
			expectEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.setupFile != "" {
				path := filepath.Join(tmpDir, tt.setupFile)
				if err := os.WriteFile(path, []byte(`{"patterns":[]}`), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				defer os.Remove(path)
			}

			enabled := isEnabledInDir(tmpDir)
			if enabled != tt.expectEnabled {
				t.Errorf("Expected enabled=%v, got %v", tt.expectEnabled, enabled)
			}
		})
	}
}

// TestEnableDisable tests enabling and disabling redaction via file renaming
func TestEnableDisable(t *testing.T) {
	tmpDir := t.TempDir()
	enabledPath := filepath.Join(tmpDir, "redaction.json")
	disabledPath := filepath.Join(tmpDir, "redaction.json.disabled")

	// Create disabled config
	testConfig := Config{
		Patterns: []Pattern{
			{Name: "Test", Pattern: "test", Type: "api_key"},
		},
	}
	data, _ := json.MarshalIndent(testConfig, "", "  ")
	if err := os.WriteFile(disabledPath, data, 0644); err != nil {
		t.Fatalf("Failed to create disabled config: %v", err)
	}

	// Test Enable
	t.Run("Enable", func(t *testing.T) {
		if err := enableInDir(tmpDir); err != nil {
			t.Fatalf("Failed to enable: %v", err)
		}

		// Verify enabled file exists
		if _, err := os.Stat(enabledPath); os.IsNotExist(err) {
			t.Error("Expected redaction.json to exist after enabling")
		}

		// Verify disabled file doesn't exist
		if _, err := os.Stat(disabledPath); !os.IsNotExist(err) {
			t.Error("Expected redaction.json.disabled to not exist after enabling")
		}

		// Verify enabled state
		if !isEnabledInDir(tmpDir) {
			t.Error("Expected redaction to be enabled")
		}
	})

	// Test Disable
	t.Run("Disable", func(t *testing.T) {
		if err := disableInDir(tmpDir); err != nil {
			t.Fatalf("Failed to disable: %v", err)
		}

		// Verify disabled file exists
		if _, err := os.Stat(disabledPath); os.IsNotExist(err) {
			t.Error("Expected redaction.json.disabled to exist after disabling")
		}

		// Verify enabled file doesn't exist
		if _, err := os.Stat(enabledPath); !os.IsNotExist(err) {
			t.Error("Expected redaction.json to not exist after disabling")
		}

		// Verify disabled state
		if isEnabledInDir(tmpDir) {
			t.Error("Expected redaction to be disabled")
		}
	})
}

// TestEnableWhenNoFile tests enabling when no config file exists
func TestEnableWhenNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	err := enableInDir(tmpDir)
	if err == nil {
		t.Error("Expected error when enabling with no config file")
	}
}

// TestDisableWhenNoFile tests disabling when no config file exists
func TestDisableWhenNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	err := disableInDir(tmpDir)
	if err == nil {
		t.Error("Expected error when disabling with no config file")
	}
}

// TestInitializeDefaultConfig tests creating a new config with default patterns
func TestInitializeDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "redaction.json.disabled")

	if err := initializeDefaultConfigToPath(configPath); err != nil {
		t.Fatalf("Failed to initialize default config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to exist after initialization")
	}

	// Load and verify default patterns
	config, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load initialized config: %v", err)
	}

	// Should have multiple default patterns
	if len(config.Patterns) < 5 {
		t.Errorf("Expected at least 5 default patterns, got %d", len(config.Patterns))
	}

	// Verify pattern structure
	for i, pattern := range config.Patterns {
		if pattern.Name == "" {
			t.Errorf("Pattern %d has empty name", i)
		}
		// Must have at least one of Pattern or FieldPattern
		if pattern.Pattern == "" && pattern.FieldPattern == "" {
			t.Errorf("Pattern %d has neither pattern nor field_pattern", i)
		}
		if pattern.Type == "" {
			t.Errorf("Pattern %d has empty type", i)
		}
	}
}

// TestConfigWithCaptureGroups tests patterns with capture groups
func TestConfigWithCaptureGroups(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "redaction.json")

	testConfig := Config{
		Patterns: []Pattern{
			{
				Name:         "PostgreSQL Password",
				Pattern:      `postgres://[^:]+:([^@\s]+)@`,
				Type:         "password",
				CaptureGroup: 1,
			},
		},
	}

	// Save config
	if err := saveConfigToPath(configPath, testConfig); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load and verify
	loaded, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(loaded.Patterns) != 1 {
		t.Fatalf("Expected 1 pattern, got %d", len(loaded.Patterns))
	}

	pattern := loaded.Patterns[0]
	if pattern.CaptureGroup != 1 {
		t.Errorf("Expected capture group 1, got %d", pattern.CaptureGroup)
	}
}

// TestGetConfigPath tests getting the config file path
func TestGetConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	enabledPath := getConfigPathInDir(tmpDir)
	expected := filepath.Join(tmpDir, "redaction.json")

	if enabledPath != expected {
		t.Errorf("Expected config path %s, got %s", expected, enabledPath)
	}

	disabledPath := getDisabledConfigPathInDir(tmpDir)
	expected = filepath.Join(tmpDir, "redaction.json.disabled")

	if disabledPath != expected {
		t.Errorf("Expected disabled config path %s, got %s", expected, disabledPath)
	}
}

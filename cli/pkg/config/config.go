package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const settingsFile = ".claude/settings.json"

// ClaudeSettings represents the structure of ~/.claude/settings.json
type ClaudeSettings struct {
	Hooks map[string][]HookMatcher `json:"hooks,omitempty"`
	// Other fields we don't care about are ignored
}

// HookMatcher represents a hook matcher configuration
type HookMatcher struct {
	Matcher string `json:"matcher"`
	Hooks   []Hook `json:"hooks"`
}

// Hook represents a single hook command
type Hook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// GetSettingsPath returns the path to the Claude settings file
func GetSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, settingsFile), nil
}

// ReadSettings reads the Claude settings file
func ReadSettings() (*ClaudeSettings, error) {
	settingsPath, err := GetSettingsPath()
	if err != nil {
		return nil, err
	}

	// If file doesn't exist, return empty settings
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return &ClaudeSettings{
			Hooks: make(map[string][]HookMatcher),
		}, nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}

	var settings ClaudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]HookMatcher)
	}

	return &settings, nil
}

// WriteSettings writes the Claude settings file
func WriteSettings(settings *ClaudeSettings) error {
	settingsPath, err := GetSettingsPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	settingsDir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	return nil
}

// GetBinaryPath returns the absolute path to the confab binary
func GetBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	return realPath, nil
}

// InstallHook installs the confab SessionEnd hook
func InstallHook() error {
	settings, err := ReadSettings()
	if err != nil {
		return err
	}

	// Get binary path
	binaryPath, err := GetBinaryPath()
	if err != nil {
		return err
	}

	// Create hook configuration
	confabHook := Hook{
		Type:    "command",
		Command: fmt.Sprintf("%s save", binaryPath),
	}

	// Check if SessionEnd hook already exists
	sessionEndHooks := settings.Hooks["SessionEnd"]

	// Check if confab hook is already installed
	for i, matcher := range sessionEndHooks {
		if matcher.Matcher == "*" {
			// Check if confab is already in the hooks
			for _, hook := range matcher.Hooks {
				if hook.Type == "command" && (hook.Command == confabHook.Command ||
					filepath.Base(hook.Command) == filepath.Base(confabHook.Command)) {
					// Already installed, update the path
					settings.Hooks["SessionEnd"][i].Hooks = []Hook{confabHook}
					return WriteSettings(settings)
				}
			}
			// Add to existing matcher
			settings.Hooks["SessionEnd"][i].Hooks = append(matcher.Hooks, confabHook)
			return WriteSettings(settings)
		}
	}

	// No matching matcher found, create new one
	newMatcher := HookMatcher{
		Matcher: "*",
		Hooks:   []Hook{confabHook},
	}

	settings.Hooks["SessionEnd"] = append(sessionEndHooks, newMatcher)

	return WriteSettings(settings)
}

// UninstallHook removes the confab SessionEnd hook
func UninstallHook() error {
	settings, err := ReadSettings()
	if err != nil {
		return err
	}

	sessionEndHooks := settings.Hooks["SessionEnd"]
	if len(sessionEndHooks) == 0 {
		return nil // Nothing to uninstall
	}

	// Remove confab hooks
	var updated []HookMatcher
	for _, matcher := range sessionEndHooks {
		var remainingHooks []Hook
		for _, hook := range matcher.Hooks {
			// Keep hooks that don't contain "confab"
			if hook.Type != "command" || !containsConfab(hook.Command) {
				remainingHooks = append(remainingHooks, hook)
			}
		}

		// Only keep matcher if it has remaining hooks
		if len(remainingHooks) > 0 {
			matcher.Hooks = remainingHooks
			updated = append(updated, matcher)
		}
	}

	settings.Hooks["SessionEnd"] = updated

	return WriteSettings(settings)
}

// IsHookInstalled checks if the confab hook is installed
func IsHookInstalled() (bool, error) {
	settings, err := ReadSettings()
	if err != nil {
		return false, err
	}

	sessionEndHooks := settings.Hooks["SessionEnd"]
	for _, matcher := range sessionEndHooks {
		for _, hook := range matcher.Hooks {
			if hook.Type == "command" && containsConfab(hook.Command) {
				return true, nil
			}
		}
	}

	return false, nil
}

// containsConfab checks if a command string references confab
func containsConfab(command string) bool {
	// Check if command contains "confab save" or just "confab"
	return strings.Contains(command, "confab save") ||
		   strings.Contains(command, "confab")
}

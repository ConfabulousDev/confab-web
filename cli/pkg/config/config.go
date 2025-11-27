package config

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
// (defaults to ~/.claude/settings.json, can be overridden with CONFAB_CLAUDE_DIR)
func GetSettingsPath() (string, error) {
	return GetClaudeSettingsPath()
}

// ReadSettings reads the Claude settings file
func ReadSettings() (*ClaudeSettings, error) {
	settingsPath, err := GetSettingsPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get settings path: %w", err)
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

// writeSettingsInternal writes settings with optional mtime-based optimistic locking
// If expectedMtime is zero, mtime checking is skipped
// If expectedMtime is non-zero, it checks mtime and returns error on mismatch
func writeSettingsInternal(settings *ClaudeSettings, expectedMtime time.Time) error {
	settingsPath, err := GetSettingsPath()
	if err != nil {
		return fmt.Errorf("failed to get settings path: %w", err)
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

	// Use temp file + atomic rename to prevent corruption
	// Create a unique temp file in the same directory to avoid conflicts
	tempFile, err := os.CreateTemp(settingsDir, ".settings-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Write data and close
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write temp settings: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set proper permissions
	if err := os.Chmod(tempPath, 0644); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// If mtime checking is enabled, verify file hasn't changed RIGHT BEFORE rename
	// This minimizes the race window to just the rename operation
	if !expectedMtime.IsZero() {
		info, err := os.Stat(settingsPath)
		if err != nil && !os.IsNotExist(err) {
			os.Remove(tempPath)
			return fmt.Errorf("failed to stat settings for mtime check: %w", err)
		}

		// Check mtime mismatch (file was modified by another process)
		if info != nil && !info.ModTime().Equal(expectedMtime) {
			os.Remove(tempPath)
			return fmt.Errorf("settings file was modified by another process (expected mtime: %v, actual: %v)",
				expectedMtime, info.ModTime())
		}
	}

	// Atomic rename (this is where mtime gets updated by OS)
	if err := os.Rename(tempPath, settingsPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename temp settings: %w", err)
	}

	return nil
}

// AtomicUpdateSettings performs a read-modify-write with optimistic locking
// It retries up to maxRetries times if the file is modified by another process
// The updateFn receives the current settings and should modify them in-place
//
// Race condition window: <1ms (only during the mtime check + rename operation)
// This is significantly smaller than the ~9ms window without optimistic locking
func AtomicUpdateSettings(updateFn func(*ClaudeSettings) error) error {
	const maxRetries = 10
	const baseRetryDelay = 5 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Read current settings and capture mtime
		settingsPath, err := GetSettingsPath()
		if err != nil {
			return fmt.Errorf("failed to get settings path: %w", err)
		}

		var mtime time.Time
		if info, err := os.Stat(settingsPath); err == nil {
			mtime = info.ModTime()
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat settings: %w", err)
		}
		// If file doesn't exist, mtime stays zero (no conflict possible)

		settings, err := ReadSettings()
		if err != nil {
			return fmt.Errorf("failed to read settings: %w", err)
		}

		// Apply user's modifications
		if err := updateFn(settings); err != nil {
			return fmt.Errorf("update function failed: %w", err)
		}

		// Try to write with mtime check
		err = writeSettingsInternal(settings, mtime)
		if err == nil {
			return nil // Success!
		}

		// Check if error is due to concurrent modification
		if strings.Contains(err.Error(), "modified by another process") {
			// Retry with exponential backoff + jitter
			if attempt < maxRetries-1 {
				// Exponential backoff: 5ms, 10ms, 20ms, 40ms, ...
				backoff := baseRetryDelay * time.Duration(1<<uint(attempt))
				// Add jitter (0-50% of backoff) to avoid thundering herd
				jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
				time.Sleep(backoff + jitter)
				continue
			}
			return fmt.Errorf("failed to update settings after %d attempts: %w", maxRetries, err)
		}

		// Other error, don't retry
		return err
	}

	return fmt.Errorf("failed to update settings after %d attempts", maxRetries)
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
// Uses optimistic locking to prevent race conditions with concurrent updates
func InstallHook() error {
	// Get binary path outside the update function (doesn't change)
	binaryPath, err := GetBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get binary path: %w", err)
	}

	// Create hook configuration
	confabHook := Hook{
		Type:    "command",
		Command: fmt.Sprintf("%s save", binaryPath),
	}

	// Use atomic update with retry
	return AtomicUpdateSettings(func(settings *ClaudeSettings) error {
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
						return nil
					}
				}
				// Add to existing matcher
				settings.Hooks["SessionEnd"][i].Hooks = append(matcher.Hooks, confabHook)
				return nil
			}
		}

		// No matching matcher found, create new one
		newMatcher := HookMatcher{
			Matcher: "*",
			Hooks:   []Hook{confabHook},
		}

		settings.Hooks["SessionEnd"] = append(sessionEndHooks, newMatcher)
		return nil
	})
}

// UninstallHook removes the confab SessionEnd hook
// Uses optimistic locking to prevent race conditions with concurrent updates
func UninstallHook() error {
	return AtomicUpdateSettings(func(settings *ClaudeSettings) error {
		sessionEndHooks := settings.Hooks["SessionEnd"]
		if len(sessionEndHooks) == 0 {
			return nil // Nothing to uninstall
		}

		// Remove confab hooks
		var updated []HookMatcher
		for _, matcher := range sessionEndHooks {
			var remainingHooks []Hook
			for _, hook := range matcher.Hooks {
				// Keep hooks that aren't confab commands
				if hook.Type != "command" || !isConfabCommand(hook.Command) {
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
		return nil
	})
}

// IsHookInstalled checks if the confab hook is installed
func IsHookInstalled() (bool, error) {
	settings, err := ReadSettings()
	if err != nil {
		return false, fmt.Errorf("failed to read settings: %w", err)
	}

	sessionEndHooks := settings.Hooks["SessionEnd"]
	for _, matcher := range sessionEndHooks {
		for _, hook := range matcher.Hooks {
			if hook.Type == "command" && isConfabCommand(hook.Command) {
				return true, nil
			}
		}
	}

	return false, nil
}

// isConfabCommand checks if a command string is a confab command
// More precise than simple string contains to avoid false positives
func isConfabCommand(command string) bool {
	// Extract the executable name from the command
	// Command format is typically: "/path/to/confab save" or "confab save"
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}

	executable := parts[0]
	baseName := filepath.Base(executable)

	// Check if the executable is exactly "confab"
	return baseName == "confab"
}

// InstallSyncHooks installs hooks for incremental sync daemon
// This installs both SessionStart (to start daemon) and SessionEnd (to stop daemon)
func InstallSyncHooks() error {
	binaryPath, err := GetBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get binary path: %w", err)
	}

	syncStartHook := Hook{
		Type:    "command",
		Command: fmt.Sprintf("%s sync start", binaryPath),
	}

	syncStopHook := Hook{
		Type:    "command",
		Command: fmt.Sprintf("%s sync stop", binaryPath),
	}

	return AtomicUpdateSettings(func(settings *ClaudeSettings) error {
		// Install SessionStart hook
		installHookForEvent(settings, "SessionStart", syncStartHook)

		// Install SessionEnd hook
		installHookForEvent(settings, "SessionEnd", syncStopHook)

		return nil
	})
}

// installHookForEvent installs a hook for a specific event type
func installHookForEvent(settings *ClaudeSettings, eventName string, hook Hook) {
	eventHooks := settings.Hooks[eventName]

	// Check if hook is already installed
	for i, matcher := range eventHooks {
		if matcher.Matcher == "*" {
			// Check if confab is already in the hooks
			for j, existingHook := range matcher.Hooks {
				if existingHook.Type == "command" && isConfabCommand(existingHook.Command) {
					// Already installed, update it
					settings.Hooks[eventName][i].Hooks[j] = hook
					return
				}
			}
			// Add to existing matcher
			settings.Hooks[eventName][i].Hooks = append(matcher.Hooks, hook)
			return
		}
	}

	// No matching matcher found, create new one
	newMatcher := HookMatcher{
		Matcher: "*",
		Hooks:   []Hook{hook},
	}
	settings.Hooks[eventName] = append(eventHooks, newMatcher)
}

// UninstallSyncHooks removes the sync daemon hooks
func UninstallSyncHooks() error {
	return AtomicUpdateSettings(func(settings *ClaudeSettings) error {
		uninstallHookForEvent(settings, "SessionStart")
		uninstallHookForEvent(settings, "SessionEnd")
		return nil
	})
}

// uninstallHookForEvent removes confab hooks from a specific event type
func uninstallHookForEvent(settings *ClaudeSettings, eventName string) {
	eventHooks := settings.Hooks[eventName]
	if len(eventHooks) == 0 {
		return
	}

	var updated []HookMatcher
	for _, matcher := range eventHooks {
		var remainingHooks []Hook
		for _, hook := range matcher.Hooks {
			if hook.Type != "command" || !isConfabCommand(hook.Command) {
				remainingHooks = append(remainingHooks, hook)
			}
		}
		if len(remainingHooks) > 0 {
			matcher.Hooks = remainingHooks
			updated = append(updated, matcher)
		}
	}
	settings.Hooks[eventName] = updated
}

// IsSyncHooksInstalled checks if sync daemon hooks are installed
func IsSyncHooksInstalled() (bool, error) {
	settings, err := ReadSettings()
	if err != nil {
		return false, fmt.Errorf("failed to read settings: %w", err)
	}

	// Check for SessionStart hook
	hasStart := false
	for _, matcher := range settings.Hooks["SessionStart"] {
		for _, hook := range matcher.Hooks {
			if hook.Type == "command" && isConfabCommand(hook.Command) &&
				strings.Contains(hook.Command, "sync start") {
				hasStart = true
				break
			}
		}
	}

	// Check for SessionEnd hook
	hasEnd := false
	for _, matcher := range settings.Hooks["SessionEnd"] {
		for _, hook := range matcher.Hooks {
			if hook.Type == "command" && isConfabCommand(hook.Command) &&
				strings.Contains(hook.Command, "sync stop") {
				hasEnd = true
				break
			}
		}
	}

	return hasStart && hasEnd, nil
}

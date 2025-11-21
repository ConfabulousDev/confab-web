package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/discovery"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/santaclaude2025/confab/pkg/upload"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save [session-id...]",
	Short: "Save session data to cloud",
	Long: `Without arguments: reads session metadata from stdin (for SessionEnd hook).
With arguments: uploads specified session(s) by ID.

Examples:
  confab save                    # Hook mode: read from stdin
  confab save abc123de           # Upload specific session
  confab save abc123de f9e8d7c6  # Upload multiple sessions`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If session IDs provided, use manual upload mode
		if len(args) > 0 {
			return saveSessionsByID(args)
		}

		// Otherwise, use hook mode (read from stdin)
		return saveFromHook()
	},
}

// saveSessionsByID uploads specific sessions by their IDs
func saveSessionsByID(sessionIDs []string) error {
	// Check authentication
	cfg, err := config.GetUploadConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("not authenticated. Run 'confab login' first")
	}

	// Find session files
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectsDir := filepath.Join(home, ".claude", "projects")

	for _, sessionID := range sessionIDs {
		// Handle partial session IDs (first 8 chars)
		fullSessionID, transcriptPath, err := findSessionByID(projectsDir, sessionID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Uploading session %s...\n", fullSessionID[:8])

		// Create hook input for discovery
		hookInput := &types.HookInput{
			SessionID:      fullSessionID,
			TranscriptPath: transcriptPath,
			CWD:            filepath.Dir(transcriptPath),
			Reason:         "manual",
		}

		// Discover and upload
		files, err := discovery.DiscoverSessionFiles(hookInput)
		if err != nil {
			fmt.Printf("  Error discovering files: %v\n", err)
			continue
		}

		if err := upload.UploadToCloud(hookInput, files); err != nil {
			fmt.Printf("  Error uploading: %v\n", err)
			continue
		}

		fmt.Printf("  ✓ Uploaded (%d files)\n", len(files))
	}

	return nil
}

// findSessionByID finds a session transcript by full or partial ID
func findSessionByID(projectsDir, partialID string) (fullID string, transcriptPath string, err error) {
	var matches []struct {
		id   string
		path string
	}

	filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".jsonl") || strings.HasPrefix(d.Name(), "agent-") {
			return nil
		}

		sessionID := strings.TrimSuffix(d.Name(), ".jsonl")

		// Match full ID or prefix
		if sessionID == partialID || strings.HasPrefix(sessionID, partialID) {
			matches = append(matches, struct {
				id   string
				path string
			}{sessionID, path})
		}

		return nil
	})

	if len(matches) == 0 {
		return "", "", fmt.Errorf("session not found: %s", partialID)
	}

	if len(matches) > 1 {
		return "", "", fmt.Errorf("ambiguous session ID '%s' matches %d sessions", partialID, len(matches))
	}

	return matches[0].id, matches[0].path, nil
}

// saveFromHook handles the hook mode (reading from stdin)
func saveFromHook() error {
	// Initialize logger
	logger.Init()
	defer logger.Close()

	logger.Info("Starting session capture")

	// Always output valid hook response, even on error
	defer func() {
		response := types.HookResponse{
			Continue:       true,
			StopReason:     "",
			SuppressOutput: false,
		}
		json.NewEncoder(os.Stdout).Encode(response)
	}()

	fmt.Fprintln(os.Stderr, "=== Confab: Capture Session ===")
	fmt.Fprintln(os.Stderr)

	// Read hook input from stdin
	hookInput, err := discovery.ReadHookInput()
	if err != nil {
		logger.Error("Failed to read hook input: %v", err)
		fmt.Fprintf(os.Stderr, "Error reading hook input: %v\n", err)
		return nil // Don't return error - let defer send success response
	}

	logger.Info("Session ID: %s", hookInput.SessionID)
	logger.Info("Transcript: %s", hookInput.TranscriptPath)
	logger.Info("Working Directory: %s", hookInput.CWD)
	logger.Info("End Reason: %s", hookInput.Reason)

	fmt.Fprintf(os.Stderr, "Session ID: %s\n", hookInput.SessionID)
	fmt.Fprintf(os.Stderr, "Transcript: %s\n", hookInput.TranscriptPath)
	fmt.Fprintf(os.Stderr, "Working Directory: %s\n", hookInput.CWD)
	fmt.Fprintf(os.Stderr, "End Reason: %s\n", hookInput.Reason)
	fmt.Fprintln(os.Stderr)

	// Discover session files
	files, err := discovery.DiscoverSessionFiles(hookInput)
	if err != nil {
		logger.Error("Failed to discover files: %v", err)
		fmt.Fprintf(os.Stderr, "Error discovering files: %v\n", err)
		return nil
	}

	// Calculate total size
	var totalSize int64
	for _, f := range files {
		totalSize += f.SizeBytes
	}

	logger.Info("Discovered %d file(s) (%.2f MB)", len(files), float64(totalSize)/(1024*1024))
	for _, f := range files {
		sizeKB := float64(f.SizeBytes) / 1024
		logger.Debug("File: %s (%s, %.1f KB)", f.Path, f.Type, sizeKB)
	}

	fmt.Fprintf(os.Stderr, "Discovered %d file(s) (%.2f MB)\n", len(files), float64(totalSize)/(1024*1024))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Files:")
	for _, f := range files {
		sizeKB := float64(f.SizeBytes) / 1024
		fmt.Fprintf(os.Stderr, "  - %s (%s, %.1f KB)\n", filepath.Base(f.Path), f.Type, sizeKB)
	}
	fmt.Fprintln(os.Stderr)

	// Cloud upload (required)
	logger.Info("Uploading to cloud...")
	if err := upload.UploadToCloud(hookInput, files); err != nil {
		logger.Error("Failed to upload to cloud: %v", err)
		fmt.Fprintf(os.Stderr, "Error: Cloud upload failed: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Cloud upload is required. Please run 'confab login' to authenticate.")
		return nil
	}

	logger.Info("Cloud upload completed")
	fmt.Fprintln(os.Stderr, "✓ Uploaded to cloud")

	logger.Info("Session capture complete")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "=== Session Captured ===")

	return nil
}

func init() {
	rootCmd.AddCommand(saveCmd)
}

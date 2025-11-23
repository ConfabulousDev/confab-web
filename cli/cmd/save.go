package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/discovery"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/santaclaude2025/confab/pkg/upload"
	"github.com/santaclaude2025/confab/pkg/utils"
	"github.com/spf13/cobra"
)

// Helper functions for dual logging (file + stderr)

// logInfo logs to file and prints to stderr
func logInfo(format string, args ...interface{}) {
	logger.Info(format, args...)
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// logError logs error to file and prints to stderr
func logError(format string, args ...interface{}) {
	logger.Error(format, args...)
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// logDebugPrint logs at debug level and prints to stderr
// (stderr output is not conditional on log level)
func logDebugPrint(format string, args ...interface{}) {
	logger.Debug(format, args...)
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

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
	cfg, err := config.EnsureAuthenticated()
	if err != nil {
		return err
	}

	for _, sessionID := range sessionIDs {
		// Handle partial session IDs (first 8 chars)
		fullSessionID, transcriptPath, err := discovery.FindSessionByID(sessionID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Uploading session %s...\n", utils.TruncateSecret(fullSessionID, 8, 0))

		// Create hook input for discovery
		hookInput := types.NewHookInput(fullSessionID, transcriptPath, filepath.Dir(transcriptPath), "manual")

		// Discover and upload
		files, err := discovery.DiscoverSessionFiles(hookInput)
		if err != nil {
			fmt.Printf("  Error discovering files: %v\n", err)
			continue
		}

		if err := upload.UploadToCloudWithConfig(cfg, hookInput, files); err != nil {
			fmt.Printf("  Error uploading: %v\n", err)
			continue
		}

		fmt.Printf("  ✓ Uploaded (%d files)\n", len(files))
	}

	return nil
}

// saveFromHook handles the hook mode (reading from stdin)
func saveFromHook() error {
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
		logError("Error reading hook input: %v", err)
		return nil // Don't return error - let defer send success response
	}

	// Display session info
	printSessionInfo(hookInput)

	// Discover session files
	files, err := discovery.DiscoverSessionFiles(hookInput)
	if err != nil {
		logError("Error discovering files: %v", err)
		return nil
	}

	// Display discovered files
	printDiscoveredFiles(files)

	// Upload to cloud
	if err := uploadSessionFiles(hookInput, files); err != nil {
		return nil // Error already logged and displayed
	}

	logger.Info("Session capture complete")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "=== Session Captured ===")

	return nil
}

// printSessionInfo logs and displays session metadata
func printSessionInfo(hookInput *types.HookInput) {
	logInfo("Session ID: %s", hookInput.SessionID)
	logInfo("Transcript: %s", hookInput.TranscriptPath)
	logInfo("Working Directory: %s", hookInput.CWD)
	logInfo("End Reason: %s", hookInput.Reason)
	fmt.Fprintln(os.Stderr)
}

// printDiscoveredFiles logs and displays discovered files with sizes
func printDiscoveredFiles(files []types.SessionFile) {
	totalSize := utils.CalculateTotalSize(files)

	logInfo("Discovered %d file(s) (%s)", len(files), utils.FormatBytesMB(totalSize))
	for _, f := range files {
		logger.Debug("File: %s (%s, %s)", f.Path, f.Type, utils.FormatBytesKB(f.SizeBytes))
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Files:")
	for _, f := range files {
		fmt.Fprintf(os.Stderr, "  - %s (%s, %s)\n", filepath.Base(f.Path), f.Type, utils.FormatBytesKB(f.SizeBytes))
	}
	fmt.Fprintln(os.Stderr)
}

// uploadSessionFiles uploads files to cloud and handles errors
func uploadSessionFiles(hookInput *types.HookInput, files []types.SessionFile) error {
	logger.Info("Uploading to cloud...")
	if err := upload.UploadToCloud(hookInput, files); err != nil {
		logError("Error: Cloud upload failed: %v", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Cloud upload is required. Please run 'confab login' to authenticate.")
		return err
	}

	logger.Info("Cloud upload completed")
	fmt.Fprintln(os.Stderr, "✓ Uploaded to cloud")
	return nil
}

func init() {
	rootCmd.AddCommand(saveCmd)
}

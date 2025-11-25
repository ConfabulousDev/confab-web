package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/discovery"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/picker"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/santaclaude2025/confab/pkg/upload"
	"github.com/santaclaude2025/confab/pkg/utils"
	"github.com/spf13/cobra"
)

var interactiveDuration string

var saveCmd = &cobra.Command{
	Use:   "save [session-id...]",
	Short: "Save session data to cloud",
	Long: `Without arguments: reads session metadata from stdin (for SessionEnd hook).
With arguments: uploads specified session(s) by ID.
With -i flag: interactive mode to select sessions.

Examples:
  confab save                    # Hook mode: read from stdin
  confab save abc123de           # Upload specific session
  confab save abc123de f9e8d7c6  # Upload multiple sessions
  confab save -i all             # Interactive: pick from all sessions
  confab save -i 5d              # Interactive: sessions from last 5 days
  confab save -i 12h             # Interactive: sessions from last 12 hours`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Interactive mode
		if interactiveDuration != "" {
			// Treat "all" as no filter
			if interactiveDuration == "all" {
				return saveInteractive("")
			}
			return saveInteractive(interactiveDuration)
		}

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

		sessionURL, err := upload.UploadToCloudWithConfig(cfg, hookInput, files)
		if err != nil {
			fmt.Printf("  Error uploading: %v\n", err)
			continue
		}

		fmt.Printf("  ✓ Uploaded (%d files)\n", len(files))
		fmt.Printf("  %s\n", sessionURL)
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
		logger.ErrorPrint("Error reading hook input: %v", err)
		return nil // Don't return error - let defer send success response
	}

	// Display session info
	printSessionInfo(hookInput)

	// Discover session files
	files, err := discovery.DiscoverSessionFiles(hookInput)
	if err != nil {
		logger.ErrorPrint("Error discovering files: %v", err)
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
	logger.InfoPrint("Session ID: %s", hookInput.SessionID)
	logger.InfoPrint("Transcript: %s", hookInput.TranscriptPath)
	logger.InfoPrint("Working Directory: %s", hookInput.CWD)
	logger.InfoPrint("End Reason: %s", hookInput.Reason)
	fmt.Fprintln(os.Stderr)
}

// printDiscoveredFiles logs and displays discovered files with sizes
func printDiscoveredFiles(files []types.SessionFile) {
	totalSize := utils.CalculateTotalSize(files)

	logger.InfoPrint("Discovered %d file(s) (%s)", len(files), utils.FormatBytesMB(totalSize))
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
	sessionURL, err := upload.UploadToCloud(hookInput, files)
	if err != nil {
		logger.ErrorPrint("Error: Cloud upload failed: %v", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Cloud upload is required. Please run 'confab login' to authenticate.")
		return err
	}

	logger.InfoPrint("Uploaded to cloud")
	if sessionURL != "" {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "View session: %s\n", sessionURL)
	}
	return nil
}

// parseDuration parses a duration string like "5d", "12h", "30m"
// Returns the duration. If empty string, returns 0 (meaning no filter).
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	// Match pattern like "5d", "12h", "30m"
	re := regexp.MustCompile(`^(\d+)([dhm])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s (use e.g., 5d, 12h, 30m)", s)
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	default:
		return 0, fmt.Errorf("invalid duration unit: %s", unit)
	}
}

// saveInteractive launches the interactive session picker
// durationStr filters sessions to those within the given duration (e.g., "5d", "12h")
func saveInteractive(durationStr string) error {
	// Parse duration filter
	duration, err := parseDuration(durationStr)
	if err != nil {
		return err
	}

	// Check authentication
	cfg, err := config.EnsureAuthenticated()
	if err != nil {
		return err
	}

	// Scan for sessions
	fmt.Println("Scanning for sessions...")
	sessions, err := discovery.ScanAllSessions()
	if err != nil {
		return fmt.Errorf("failed to scan sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found in ~/.claude/projects/")
		return nil
	}

	// Filter by duration if specified
	if duration > 0 {
		cutoff := time.Now().Add(-duration)
		var filtered []discovery.SessionInfo
		for _, s := range sessions {
			if s.ModTime.After(cutoff) {
				filtered = append(filtered, s)
			}
		}
		sessions = filtered

		if len(sessions) == 0 {
			fmt.Printf("No sessions found within the last %s\n", durationStr)
			return nil
		}
	}

	// Sort by mod time (most recent first for picker)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})

	// Launch picker
	selected, err := picker.PickSessions(sessions)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		fmt.Println("No sessions selected.")
		return nil
	}

	// Upload selected sessions
	fmt.Printf("\nUploading %d session(s)...\n", len(selected))
	for _, session := range selected {
		fmt.Printf("  %s ... ", utils.TruncateSecret(session.SessionID, 8, 0))

		hookInput := types.NewHookInput(session.SessionID, session.TranscriptPath, filepath.Dir(session.TranscriptPath), "interactive")

		files, err := discovery.DiscoverSessionFiles(hookInput)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}

		sessionURL, err := upload.UploadToCloudWithConfig(cfg, hookInput, files)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}

		fmt.Printf("✓ (%d files)\n", len(files))
		fmt.Printf("    %s\n", sessionURL)
	}

	fmt.Println("\nDone.")
	return nil
}

func init() {
	saveCmd.Flags().StringVarP(&interactiveDuration, "interactive", "i", "", "Interactive mode: select sessions to upload (optionally filter by duration, e.g., 5d, 12h)")
	rootCmd.AddCommand(saveCmd)
}

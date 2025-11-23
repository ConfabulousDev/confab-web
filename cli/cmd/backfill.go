package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/discovery"
	confabhttp "github.com/santaclaude2025/confab/pkg/http"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/santaclaude2025/confab/pkg/upload"
	"github.com/santaclaude2025/confab/pkg/utils"
	"github.com/spf13/cobra"
)

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Upload historical sessions from ~/.claude to cloud",
	Long: `Scans ~/.claude/projects/ for existing session transcripts and uploads
them to the cloud backend. Sessions modified within the last 20 minutes are skipped
(likely still in progress). Use 'confab save <session-id>' to force upload
a specific session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Confab: Backfill Historical Sessions ===")
		fmt.Println()

		// Check authentication
		cfg, err := config.EnsureAuthenticated()
		if err != nil {
			return err
		}

		// Scan for sessions
		fmt.Println("Scanning ~/.claude/projects...")
		sessions, err := discovery.ScanAllSessions()
		if err != nil {
			return fmt.Errorf("failed to scan for sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found in ~/.claude/projects/")
			return nil
		}

		fmt.Printf("Found %d session(s)\n", len(sessions))
		fmt.Println()

		// Filter sessions by age (20 minute threshold)
		threshold := time.Now().Add(-20 * time.Minute)
		oldSessions, recentSessions := filterSessionsByAge(sessions, threshold)

		// Determine which sessions need uploading
		toUpload, alreadySynced, err := determineSessionsToUpload(cfg, oldSessions)
		if err != nil {
			return fmt.Errorf("failed to check existing sessions: %w", err)
		}

		// Print summary
		printBackfillSummary(toUpload, alreadySynced, recentSessions)

		if len(toUpload) == 0 {
			fmt.Println()
			fmt.Println("Nothing to upload.")
			return nil
		}

		// Confirm upload
		if !confirmUpload(len(toUpload)) {
			fmt.Println("Cancelled.")
			return nil
		}

		// Upload with progress
		succeeded, failed := uploadSessionsWithProgress(cfg, toUpload)

		// Print final summary
		if failed > 0 {
			fmt.Printf("Uploaded %d session(s), %d failed.\n", succeeded, failed)
		} else {
			fmt.Printf("Uploaded %d session(s).\n", succeeded)
		}

		return nil
	},
}

// filterSessionsByAge separates sessions into old and recent based on threshold
func filterSessionsByAge(sessions []discovery.SessionInfo, threshold time.Time) (old, recent []discovery.SessionInfo) {
	for _, s := range sessions {
		if s.ModTime.Before(threshold) {
			old = append(old, s)
		} else {
			recent = append(recent, s)
		}
	}
	return old, recent
}

// determineSessionsToUpload checks server for existing sessions and returns what needs uploading
func determineSessionsToUpload(cfg *config.UploadConfig, oldSessions []discovery.SessionInfo) (toUpload []discovery.SessionInfo, alreadySynced []string, err error) {
	if len(oldSessions) == 0 {
		return nil, nil, nil
	}

	// Extract session IDs
	sessionIDs := make([]string, len(oldSessions))
	for i, s := range oldSessions {
		sessionIDs[i] = s.SessionID
	}

	// Check which exist on server
	existing, err := checkSessionsExist(cfg, sessionIDs)
	if err != nil {
		return nil, nil, err
	}

	// Build set for fast lookup
	existingSet := make(map[string]bool)
	for _, id := range existing {
		existingSet[id] = true
	}

	// Separate sessions into already synced vs to upload
	for _, s := range oldSessions {
		if existingSet[s.SessionID] {
			alreadySynced = append(alreadySynced, s.SessionID)
		} else {
			toUpload = append(toUpload, s)
		}
	}

	return toUpload, alreadySynced, nil
}

// printBackfillSummary displays what will be uploaded and what's skipped
func printBackfillSummary(toUpload []discovery.SessionInfo, alreadySynced []string, recentSessions []discovery.SessionInfo) {
	// Print sync summary
	if len(alreadySynced) > 0 {
		fmt.Printf("Already synced: %d\n", len(alreadySynced))
	}
	fmt.Printf("To upload: %d\n", len(toUpload))

	// Show skipped recent sessions
	if len(recentSessions) > 0 {
		fmt.Println()
		fmt.Printf("Skipping %d recent session(s) (modified < 20 minutes ago):\n", len(recentSessions))
		for _, s := range recentSessions {
			ago := time.Since(s.ModTime).Round(time.Minute)
			fmt.Printf("  %s  %-20s  modified %s ago\n", utils.TruncateSecret(s.SessionID, 8, 0), utils.TruncateWithEllipsis(s.ProjectPath, 20), ago)
		}
		fmt.Println()
		fmt.Println("To upload a skipped session later, run: confab save <session-id>")
	}
}

// confirmUpload prompts user to confirm uploading sessions
func confirmUpload(count int) bool {
	fmt.Println()
	fmt.Printf("Proceed with uploading %d session(s)? [Y/n]: ", count)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes"
}

// uploadSessionsWithProgress uploads sessions and displays progress
func uploadSessionsWithProgress(cfg *config.UploadConfig, sessions []discovery.SessionInfo) (succeeded, failed int) {
	fmt.Println()
	for i, session := range sessions {
		fmt.Printf("\rUploading... [%d/%d] %s", i+1, len(sessions), utils.TruncateSecret(session.SessionID, 8, 0))

		err := uploadSession(cfg, session)
		if err != nil {
			fmt.Printf("\n  Error uploading %s: %v\n", utils.TruncateSecret(session.SessionID, 8, 0), err)
			failed++
		} else {
			succeeded++
		}
	}

	fmt.Printf("\rUploading... [%d/%d] Done.                    \n", len(sessions), len(sessions))
	fmt.Println()

	return succeeded, failed
}

func init() {
	rootCmd.AddCommand(backfillCmd)
}

// checkSessionsExist calls the backend to check which sessions already exist
func checkSessionsExist(cfg *config.UploadConfig, sessionIDs []string) ([]string, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	// Build request
	reqBody := struct {
		SessionIDs []string `json:"session_ids"`
	}{
		SessionIDs: sessionIDs,
	}

	// Call backend API
	client := confabhttp.NewClient(cfg, utils.DefaultHTTPTimeout)
	var result struct {
		Existing []string `json:"existing"`
		Missing  []string `json:"missing"`
	}

	if err := client.Post("/api/v1/sessions/check", reqBody, &result); err != nil {
		return nil, err
	}

	return result.Existing, nil
}

// uploadSession uploads a single session to the backend using shared upload package
func uploadSession(cfg *config.UploadConfig, session discovery.SessionInfo) error {
	// Create a hook input for discovery
	hookInput := types.NewHookInput(session.SessionID, session.TranscriptPath, filepath.Dir(session.TranscriptPath), "backfill")

	// Discover session files
	files, err := discovery.DiscoverSessionFiles(hookInput)
	if err != nil {
		return fmt.Errorf("failed to discover session files for %s: %w", session.SessionID, err)
	}

	// Use UploadToCloudWithConfig to ensure timestamp extraction happens
	return upload.UploadToCloudWithConfig(cfg, hookInput, files)
}

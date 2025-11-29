package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santaclaude2025/confab/pkg/config"
	"github.com/santaclaude2025/confab/pkg/discovery"
	confabhttp "github.com/santaclaude2025/confab/pkg/http"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/sync"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/santaclaude2025/confab/pkg/utils"
	"github.com/spf13/cobra"
)

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Upload historical sessions from ~/.claude to cloud",
	Long: `Scans ~/.claude/projects/ for existing session transcripts and uploads
them to the cloud backend.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Starting backfill")
		fmt.Println("=== Confab: Backfill Historical Sessions ===")
		fmt.Println()

		// Check authentication
		cfg, err := config.EnsureAuthenticated()
		if err != nil {
			logger.Error("Authentication check failed: %v", err)
			return err
		}

		// Scan for sessions
		logger.Info("Scanning for sessions in ~/.claude/projects")
		fmt.Println("Scanning ~/.claude/projects...")
		sessions, err := discovery.ScanAllSessions()
		if err != nil {
			logger.Error("Failed to scan for sessions: %v", err)
			return fmt.Errorf("failed to scan for sessions: %w", err)
		}

		if len(sessions) == 0 {
			logger.Info("No sessions found")
			fmt.Println("No sessions found in ~/.claude/projects/")
			return nil
		}

		logger.Info("Found %d session(s)", len(sessions))
		fmt.Printf("Found %d session(s)\n", len(sessions))
		fmt.Println()

		// Determine which sessions need uploading
		toUpload, alreadySynced, err := determineSessionsToUpload(cfg, sessions)
		if err != nil {
			logger.Error("Failed to check existing sessions: %v", err)
			return fmt.Errorf("failed to check existing sessions: %w", err)
		}

		logger.Debug("Sessions to upload: %d, already synced: %d",
			len(toUpload), len(alreadySynced))

		// Print summary
		printBackfillSummary(toUpload, alreadySynced)

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
		succeeded, failed, skippedActive := uploadSessionsWithProgress(cfg, toUpload)

		// Print final summary
		logger.Info("Backfill complete: %d succeeded, %d failed, %d skipped (active)", succeeded, failed, skippedActive)
		if failed > 0 || skippedActive > 0 {
			fmt.Printf("Uploaded %d session(s)", succeeded)
			if failed > 0 {
				fmt.Printf(", %d failed", failed)
			}
			if skippedActive > 0 {
				fmt.Printf(", %d skipped (already being synced)", skippedActive)
			}
			fmt.Println(".")
		} else {
			fmt.Printf("Uploaded %d session(s).\n", succeeded)
		}

		if skippedActive > 0 {
			fmt.Println("\nNote: Some sessions were skipped because they're being actively synced")
			fmt.Println("by the daemon. This is normal for recent/in-progress sessions.")
		}

		return nil
	},
}

// determineSessionsToUpload checks server for existing sessions and returns what needs uploading
func determineSessionsToUpload(cfg *config.UploadConfig, sessions []discovery.SessionInfo) (toUpload []discovery.SessionInfo, alreadySynced []string, err error) {
	if len(sessions) == 0 {
		return nil, nil, nil
	}

	// Extract session IDs
	sessionIDs := make([]string, len(sessions))
	for i, s := range sessions {
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
	for _, s := range sessions {
		if existingSet[s.SessionID] {
			alreadySynced = append(alreadySynced, s.SessionID)
		} else {
			toUpload = append(toUpload, s)
		}
	}

	return toUpload, alreadySynced, nil
}

// printBackfillSummary displays what will be uploaded
func printBackfillSummary(toUpload []discovery.SessionInfo, alreadySynced []string) {
	if len(alreadySynced) > 0 {
		fmt.Printf("Already synced: %d\n", len(alreadySynced))
	}
	fmt.Printf("To upload: %d\n", len(toUpload))
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
func uploadSessionsWithProgress(cfg *config.UploadConfig, sessions []discovery.SessionInfo) (succeeded, failed, skippedActive int) {
	// Create uploader once for all sessions
	uploader, err := sync.NewUploader(cfg)
	if err != nil {
		logger.Error("Failed to create uploader: %v", err)
		fmt.Printf("Error creating uploader: %v\n", err)
		return 0, len(sessions), 0
	}

	fmt.Println()
	for i, session := range sessions {
		// Show title if available, otherwise session ID
		displayName := utils.TruncateSecret(session.SessionID, 8, 0)
		if session.Title != "" {
			displayName = utils.TruncateEnd(session.Title, 40)
		}
		fmt.Printf("\rUploading... [%d/%d] %s", i+1, len(sessions), displayName)

		err := uploadSession(uploader, session)
		if err != nil {
			logger.Error("Failed to upload session %s: %v", session.SessionID, err)
			// Check if this is a sync conflict (session being actively synced)
			if strings.Contains(err.Error(), "first_line must be") {
				skippedActive++
				logger.Debug("Session %s appears to be actively syncing, skipped", session.SessionID)
			} else {
				fmt.Printf("\n  Error uploading %s: %v\n", utils.TruncateSecret(session.SessionID, 8, 0), err)
				failed++
			}
		} else {
			logger.Debug("Uploaded session %s", session.SessionID)
			succeeded++
		}
	}

	fmt.Printf("\rUploading... [%d/%d] Done.                                        \n", len(sessions), len(sessions))
	fmt.Println()

	return succeeded, failed, skippedActive
}

func init() {
	rootCmd.AddCommand(backfillCmd)
}

// checkSessionsExist calls the backend to check which sessions already exist
func checkSessionsExist(cfg *config.UploadConfig, sessionIDs []string) ([]string, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	// Build request - backend expects external_ids (Claude Code's session IDs)
	reqBody := struct {
		ExternalIDs []string `json:"external_ids"`
	}{
		ExternalIDs: sessionIDs,
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

// uploadSession uploads a single session using the sync uploader
func uploadSession(uploader *sync.Uploader, session discovery.SessionInfo) error {
	// Create a hook input for discovery
	hookInput := types.NewHookInput(session.SessionID, session.TranscriptPath, filepath.Dir(session.TranscriptPath), "backfill")

	// Discover session files
	files, err := discovery.DiscoverSessionFiles(hookInput)
	if err != nil {
		return fmt.Errorf("failed to discover session files for %s: %w", session.SessionID, err)
	}

	// Upload using sync API
	_, err = uploader.UploadSession(session.SessionID, session.TranscriptPath, hookInput.CWD, files)
	return err
}

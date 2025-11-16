package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santaclaude2025/confab/pkg/db"
	"github.com/santaclaude2025/confab/pkg/discovery"
	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/santaclaude2025/confab/pkg/types"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save session data (called by SessionEnd hook)",
	Long: `Reads session metadata from stdin, discovers associated files,
and stores them in the local database. This command is automatically
called by the Claude Code SessionEnd hook.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Store in database
		database, err := db.Open()
		if err != nil {
			logger.Error("Failed to open database: %v", err)
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			return nil
		}
		defer database.Close()

		if err := database.InsertSession(hookInput, files); err != nil {
			logger.Error("Failed to save to database: %v", err)
			fmt.Fprintf(os.Stderr, "Error saving to database: %v\n", err)
			return nil
		}

		logger.Info("Saved to database: %s", database.Path())
		fmt.Fprintln(os.Stderr, "✓ Saved to database:", database.Path())

		// TODO: Cloud upload (currently stubbed out)
		// upload.UploadToCloud(hookInput, files)

		logger.Info("Session capture complete")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "=== Session Captured ===")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}

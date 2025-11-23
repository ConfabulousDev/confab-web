package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Manage confab logs",
	Long:  "View or manage confab CLI logs",
}

var logsPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print log directory path",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory: %v", err)
			os.Exit(1)
		}
		logDir := filepath.Join(home, ".confab/logs")
		fmt.Println(logDir)
	},
}

var logsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all log files",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory: %v", err)
			os.Exit(1)
		}
		logDir := filepath.Join(home, ".confab/logs")

		files, err := filepath.Glob(filepath.Join(logDir, "confab.log*"))
		if err != nil {
			logger.Error("Failed to list logs: %v", err)
			os.Exit(1)
		}

		if len(files) == 0 {
			fmt.Println("No log files found")
			return
		}

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				logger.Warn("Failed to stat %s: %v", file, err)
				continue
			}
			fmt.Printf("%s (%d bytes)\n", filepath.Base(file), info.Size())
		}
	},
}

var logsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all old log files (keeps current)",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory: %v", err)
			os.Exit(1)
		}
		logDir := filepath.Join(home, ".confab/logs")

		// Match rotated logs (confab.log.* but not confab.log)
		files, err := filepath.Glob(filepath.Join(logDir, "confab.log.*"))
		if err != nil {
			logger.Error("Failed to list logs: %v", err)
			os.Exit(1)
		}

		if len(files) == 0 {
			fmt.Println("No old log files to delete")
			return
		}

		deletedCount := 0
		for _, file := range files {
			if err := os.Remove(file); err != nil {
				logger.Warn("Failed to delete %s: %v", filepath.Base(file), err)
			} else {
				fmt.Printf("Deleted %s\n", filepath.Base(file))
				deletedCount++
			}
		}

		fmt.Printf("\nDeleted %d old log file(s)\n", deletedCount)
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(logsPathCmd)
	logsCmd.AddCommand(logsListCmd)
	logsCmd.AddCommand(logsClearCmd)
}

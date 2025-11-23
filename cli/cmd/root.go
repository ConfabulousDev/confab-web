package cmd

import (
	"fmt"
	"os"

	"github.com/santaclaude2025/confab/pkg/logger"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "confab",
	Short: "Archive and query your Claude Code sessions",
	Long: `Confab automatically captures Claude Code session transcripts and agent sidechains
and uploads them to cloud storage for retrieval, search, and analytics.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logger for all commands (except --help which doesn't run this)
		logger.Init()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Close logger after all commands
		logger.Close()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

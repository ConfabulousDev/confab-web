package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "confab",
	Short: "Archive and query your Claude Code sessions",
	Long: `Confab automatically captures Claude Code session transcripts and agent sidechains
to local SQLite storage for retrieval, search, and analytics.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

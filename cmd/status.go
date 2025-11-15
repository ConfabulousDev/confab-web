package cmd

import (
	"fmt"
	"time"

	"github.com/santaclaude/confab/pkg/config"
	"github.com/santaclaude/confab/pkg/db"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show confab status and recent sessions",
	Long:  `Displays hook installation status, database location, and recently captured sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Confab: Status ===")
		fmt.Println()

		// Open database
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Show database info
		fmt.Printf("Database: %s\n", database.Path())

		count, err := database.GetSessionCount()
		if err != nil {
			return fmt.Errorf("failed to get session count: %w", err)
		}
		fmt.Printf("Total Sessions: %d\n", count)
		fmt.Println()

		// Show recent sessions
		if count > 0 {
			fmt.Println("Recent Sessions:")
			sessions, err := database.GetRecentSessions(10)
			if err != nil {
				return fmt.Errorf("failed to get recent sessions: %w", err)
			}

			for _, s := range sessions {
				age := time.Since(s.Timestamp)
				sizeMB := float64(s.TotalSizeBytes) / (1024 * 1024)
				fmt.Printf("  %s - %s ago (%.2f MB, %d files)\n",
					s.SessionID[:8],
					formatDuration(age),
					sizeMB,
					s.FileCount,
				)
			}
		} else {
			fmt.Println("No sessions captured yet.")
		}

		fmt.Println()

		// Check hook installation
		hookInstalled, err := config.IsHookInstalled()
		if err != nil {
			return fmt.Errorf("failed to check hook status: %w", err)
		}

		if hookInstalled {
			fmt.Println("Hook Status: ✓ Installed")
		} else {
			fmt.Println("Hook Status: ✗ Not installed")
			fmt.Println("Run 'confab init' to install the SessionEnd hook.")
		}

		return nil
	},
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	} else {
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

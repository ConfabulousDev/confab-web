package picker

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/santaclaude2025/confab/pkg/discovery"
	"github.com/santaclaude2025/confab/pkg/utils"
)

// formatAge formats a time as a human-readable age string
func formatAge(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if d < 7*24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}

	return t.Format("Jan 2")
}

// PickSessions displays a numbered list and prompts user to select sessions
func PickSessions(sessions []discovery.SessionInfo) ([]discovery.SessionInfo, error) {
	if len(sessions) == 0 {
		return nil, nil
	}

	// Display numbered list
	fmt.Println("\nAvailable sessions:")
	for i, session := range sessions {
		sessionID := utils.TruncateSecret(session.SessionID, 8, 0)
		project := utils.TruncateWithEllipsis(session.ProjectPath, 35)
		age := formatAge(session.ModTime)
		fmt.Printf("  %2d)  %s  %-35s  %s\n", i+1, sessionID, project, age)
	}

	// Prompt for selection
	fmt.Println()
	fmt.Println("Enter number(s) to upload (e.g., 1 or 1,3,5 or 1-5 or 'all'):")
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	input = strings.TrimSpace(input)
	if input == "" || input == "q" || input == "quit" {
		return nil, nil
	}

	// Parse selection
	selected, err := parseSelection(input, len(sessions))
	if err != nil {
		return nil, err
	}

	// Map indices to sessions
	var result []discovery.SessionInfo
	for _, idx := range selected {
		result = append(result, sessions[idx])
	}

	return result, nil
}

// parseSelection parses user input like "1", "1,3,5", "1-5", "all"
func parseSelection(input string, max int) ([]int, error) {
	input = strings.ToLower(strings.TrimSpace(input))

	if input == "all" || input == "a" {
		result := make([]int, max)
		for i := range result {
			result[i] = i
		}
		return result, nil
	}

	seen := make(map[int]bool)
	var result []int

	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for range (e.g., "1-5")
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}

			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid number: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid number: %s", rangeParts[1])
			}

			if start < 1 || end > max || start > end {
				return nil, fmt.Errorf("invalid range: %d-%d (valid: 1-%d)", start, end, max)
			}

			for i := start; i <= end; i++ {
				idx := i - 1 // Convert to 0-indexed
				if !seen[idx] {
					seen[idx] = true
					result = append(result, idx)
				}
			}
		} else {
			// Single number
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid number: %s", part)
			}

			if num < 1 || num > max {
				return nil, fmt.Errorf("invalid selection: %d (valid: 1-%d)", num, max)
			}

			idx := num - 1 // Convert to 0-indexed
			if !seen[idx] {
				seen[idx] = true
				result = append(result, idx)
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid selections")
	}

	return result, nil
}

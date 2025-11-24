package utils

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/types"
)

// FormatBytesKB formats bytes as KB with 1 decimal place
func FormatBytesKB(bytes int64) string {
	return formatFloat(float64(bytes)/1024) + " KB"
}

// FormatBytesMB formats bytes as MB with 2 decimal places
func FormatBytesMB(bytes int64) string {
	return formatFloat(float64(bytes)/(1024*1024)) + " MB"
}

// CalculateTotalSize sums the size of all files
func CalculateTotalSize(files []types.SessionFile) int64 {
	var total int64
	for _, f := range files {
		total += f.SizeBytes
	}
	return total
}

// formatFloat formats a float with appropriate precision
func formatFloat(value float64) string {
	// Use 1 decimal place for values >= 10, otherwise 2
	if value >= 10 {
		return fmt.Sprintf("%.1f", value)
	}
	return fmt.Sprintf("%.2f", value)
}

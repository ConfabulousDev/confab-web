package utils

import (
	"fmt"

	"github.com/santaclaude2025/confab/pkg/types"
)

// FormatBytes formats a byte count as a human-readable string with units
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/GB) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/MB) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/KB) + " KB"
	default:
		return formatInt(bytes) + " B"
	}
}

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

// formatInt converts int64 to string
func formatInt(value int64) string {
	return fmt.Sprintf("%d", value)
}

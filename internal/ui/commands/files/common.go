package files

import (
	"fmt"
)

// Common utility functions for file operations

// FormatBytes formats bytes into human-readable string
func FormatBytes(bytes int64) string {
	if bytes < byteUnit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(byteUnit), 0
	for n := bytes / byteUnit; n >= byteUnit; n /= byteUnit {
		div *= byteUnit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

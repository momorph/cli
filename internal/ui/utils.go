package ui

import (
	"path/filepath"
	"strings"
)

// ShortenPath shortens a path by abbreviating parent directories
// e.g., /Users/john/workspaces/project -> /U/j/w/project
func ShortenPath(path string) string {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	if len(parts) <= 2 {
		return path
	}

	// Keep the last part (filename/directory) and abbreviate the rest
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] != "" && len(parts[i]) > 0 {
			parts[i] = string(parts[i][0])
		}
	}

	return strings.Join(parts, string(filepath.Separator))
}

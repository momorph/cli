package ui

import (
	"fmt"
	"strings"
)

// ProgressBar represents a simple progress bar
type ProgressBar struct {
	total   int64
	current int64
	width   int
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int64) *ProgressBar {
	return &ProgressBar{
		total: total,
		width: 40,
	}
}

// Update updates the progress bar
func (pb *ProgressBar) Update(current int64) {
	pb.current = current
	pb.Render()
}

// Render renders the progress bar
func (pb *ProgressBar) Render() {
	if pb.total <= 0 {
		return
	}

	percent := float64(pb.current) / float64(pb.total) * 100
	filled := int(float64(pb.width) * float64(pb.current) / float64(pb.total))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.width-filled)
	
	fmt.Printf("\r[%s] %.1f%% (%s / %s)", 
		bar, 
		percent,
		formatBytes(pb.current),
		formatBytes(pb.total))
}

// Finish completes the progress bar
func (pb *ProgressBar) Finish() {
	pb.current = pb.total
	pb.Render()
	fmt.Println() // New line
}

// formatBytes formats bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

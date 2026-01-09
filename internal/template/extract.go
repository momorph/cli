package template

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/momorph/cli/internal/logger"
)

// Extract extracts a ZIP file to the target directory
func Extract(zipPath, targetDir string) error {
	// Open ZIP file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer reader.Close()

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Clean target directory path for security checks
	cleanTarget := filepath.Clean(targetDir)

	// Extract files
	for _, file := range reader.File {
		if err := extractFile(file, cleanTarget); err != nil {
			return fmt.Errorf("failed to extract %s: %w", file.Name, err)
		}
	}

	logger.Info("Extracted %d files to: %s", len(reader.File), targetDir)
	return nil
}

// extractFile extracts a single file from the ZIP
func extractFile(file *zip.File, targetDir string) error {
	// Build target path
	targetPath := filepath.Join(targetDir, file.Name)

	// Validate path doesn't escape target directory (path traversal protection)
	cleanPath := filepath.Clean(targetPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(targetDir)) {
		return fmt.Errorf("invalid file path: %s (path traversal attempt)", file.Name)
	}

	// Check if it's a directory
	if file.FileInfo().IsDir() {
		return os.MkdirAll(targetPath, file.Mode())
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open file in ZIP
	srcFile, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in ZIP: %w", err)
	}
	defer srcFile.Close()

	// Create target file
	dstFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// CleanupPartial removes partially extracted files on error
func CleanupPartial(targetDir string) error {
	// Check if directory exists and is not empty
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to clean up
		}
		return err
	}

	// Only clean up if there are files (partial extraction)
	if len(entries) > 0 {
		logger.Debug("Cleaning up partial extraction: %s", targetDir)
		return os.RemoveAll(targetDir)
	}

	return nil
}

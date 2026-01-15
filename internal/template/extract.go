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

// ExtractWithMerge extracts a ZIP file to the target directory, merging config files instead of overwriting
func ExtractWithMerge(zipPath, targetDir string) error {
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
	mergeQueue := make(map[string]*zip.File) // Files to merge after extraction

	// First pass: extract non-mergeable files, queue mergeable ones
	for _, file := range reader.File {
		relativePath := file.Name
		targetPath := filepath.Join(cleanTarget, relativePath)

		// Validate path doesn't escape target directory (path traversal protection)
		cleanPath := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanPath, cleanTarget) {
			return fmt.Errorf("invalid file path: %s (path traversal attempt)", file.Name)
		}

		mergeType, shouldMerge := ShouldMerge(relativePath)
		_ = mergeType // Used in second pass

		if shouldMerge && fileExists(targetPath) {
			// Queue for merging - file exists and should be merged
			mergeQueue[relativePath] = file
			logger.Debug("Queued for merge: %s", relativePath)
			continue
		}

		// Extract normally
		if err := extractFile(file, cleanTarget); err != nil {
			return fmt.Errorf("failed to extract %s: %w", file.Name, err)
		}
	}

	// Second pass: merge queued files
	for relativePath, zipFile := range mergeQueue {
		targetPath := filepath.Join(cleanTarget, relativePath)
		mergeType, _ := ShouldMerge(relativePath)

		if err := mergeFileFromZip(zipFile, targetPath, mergeType); err != nil {
			logger.Warn("Failed to merge %s, overwriting instead: %v", relativePath, err)
			// Fallback to overwrite on merge failure
			if err := extractFile(zipFile, cleanTarget); err != nil {
				return fmt.Errorf("failed to extract %s: %w", zipFile.Name, err)
			}
		} else {
			logger.Info("Merged: %s", relativePath)
		}
	}

	logger.Info("Extracted %d files to: %s (merged %d config files)", len(reader.File), targetDir, len(mergeQueue))
	return nil
}

// mergeFileFromZip extracts a file from ZIP to temp location and merges it with existing file
func mergeFileFromZip(zipFile *zip.File, existingPath string, mergeType MergeType) error {
	// Extract to temp file
	tempFile, err := os.CreateTemp("", "momorph-merge-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	srcFile, err := zipFile.Open()
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to open zip file: %w", err)
	}

	if _, err := io.Copy(tempFile, srcFile); err != nil {
		srcFile.Close()
		tempFile.Close()
		return fmt.Errorf("failed to copy to temp file: %w", err)
	}
	srcFile.Close()
	tempFile.Close()

	// Perform merge based on type
	switch mergeType {
	case MergeTypeJSON:
		return MergeJSONFiles(existingPath, tempPath)
	case MergeTypeGitignore:
		return MergeGitignoreFiles(existingPath, tempPath)
	default:
		return fmt.Errorf("unknown merge type: %d", mergeType)
	}
}

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

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

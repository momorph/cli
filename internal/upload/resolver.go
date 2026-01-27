package upload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveFiles resolves file paths from arguments, directory, and recursive options
// Returns a list of CSV file paths that match the expected pattern
func ResolveFiles(args []string, dir string, recursive bool, uploadType string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	addFile := func(path string) error {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve path %s: %w", path, err)
		}

		// Skip if already seen
		if seen[absPath] {
			return nil
		}

		// Check if file exists
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("file not found: %s", path)
		}

		// If it's a directory, skip (will be handled separately)
		if info.IsDir() {
			return nil
		}

		// Only include CSV files
		if !strings.HasSuffix(strings.ToLower(absPath), ".csv") {
			return nil
		}

		// Validate file path matches expected pattern
		_, err = ParseFilePath(absPath)
		if err != nil {
			// File doesn't match pattern, skip with warning
			return nil
		}

		seen[absPath] = true
		files = append(files, absPath)
		return nil
	}

	// Process explicit file arguments
	for _, arg := range args {
		// Check if it's a glob pattern
		if strings.ContainsAny(arg, "*?[") {
			matches, err := filepath.Glob(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern %s: %w", arg, err)
			}
			for _, match := range matches {
				if err := addFile(match); err != nil {
					// Log warning but continue
					continue
				}
			}
		} else {
			// Check if it's a directory
			info, err := os.Stat(arg)
			if err == nil && info.IsDir() {
				// Scan directory
				dirFiles, err := scanDirectory(arg, recursive, uploadType)
				if err != nil {
					return nil, err
				}
				for _, f := range dirFiles {
					if err := addFile(f); err != nil {
						continue
					}
				}
			} else {
				// Single file
				if err := addFile(arg); err != nil {
					return nil, err
				}
			}
		}
	}

	// Process directory option
	if dir != "" {
		dirFiles, err := scanDirectory(dir, recursive, uploadType)
		if err != nil {
			return nil, err
		}
		for _, f := range dirFiles {
			if err := addFile(f); err != nil {
				continue
			}
		}
	}

	// If no args and no dir specified, try to find .momorph directory
	if len(args) == 0 && dir == "" {
		// Look for .momorph/{uploadType} in current directory
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}

		momorphDir := filepath.Join(cwd, ".momorph", uploadType)
		if info, err := os.Stat(momorphDir); err == nil && info.IsDir() {
			dirFiles, err := scanDirectory(momorphDir, true, uploadType)
			if err != nil {
				return nil, err
			}
			for _, f := range dirFiles {
				if err := addFile(f); err != nil {
					continue
				}
			}
		}
	}

	return files, nil
}

// scanDirectory scans a directory for CSV files
func scanDirectory(dir string, recursive bool, uploadType string) ([]string, error) {
	var files []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip directories (but continue walking into them if recursive)
		if info.IsDir() {
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include CSV files
		if !strings.HasSuffix(strings.ToLower(path), ".csv") {
			return nil
		}

		// Validate file path matches expected pattern
		parsed, err := ParseFilePath(path)
		if err != nil {
			return nil // Skip files that don't match pattern
		}

		// If uploadType is specified, only include matching files
		if uploadType != "" && parsed.Type != uploadType {
			return nil
		}

		files = append(files, path)
		return nil
	}

	if err := filepath.Walk(dir, walkFn); err != nil {
		return nil, fmt.Errorf("failed to scan directory %s: %w", dir, err)
	}

	return files, nil
}

// ValidateFiles validates that all files exist and match expected pattern
func ValidateFiles(files []string, uploadType string) ([]string, []UploadResult) {
	var validFiles []string
	var skipped []UploadResult

	for _, file := range files {
		// Check file exists
		info, err := os.Stat(file)
		if err != nil {
			skipped = append(skipped, UploadResult{
				FilePath: file,
				FileName: filepath.Base(file),
				Status:   StatusSkipped,
				Error:    err,
				Message:  "File not found",
			})
			continue
		}

		// Check it's a file, not directory
		if info.IsDir() {
			skipped = append(skipped, UploadResult{
				FilePath: file,
				FileName: filepath.Base(file),
				Status:   StatusSkipped,
				Message:  "Path is a directory, not a file",
			})
			continue
		}

		// Check it's a CSV file
		if !strings.HasSuffix(strings.ToLower(file), ".csv") {
			skipped = append(skipped, UploadResult{
				FilePath: file,
				FileName: filepath.Base(file),
				Status:   StatusSkipped,
				Message:  "Not a CSV file",
			})
			continue
		}

		// Validate path pattern
		parsed, err := ParseFilePath(file)
		if err != nil {
			skipped = append(skipped, UploadResult{
				FilePath: file,
				FileName: filepath.Base(file),
				Status:   StatusSkipped,
				Error:    err,
				Message:  "Invalid file path format",
			})
			continue
		}

		// Check upload type matches if specified
		if uploadType != "" && parsed.Type != uploadType {
			skipped = append(skipped, UploadResult{
				FilePath: file,
				FileName: filepath.Base(file),
				Status:   StatusSkipped,
				Message:  fmt.Sprintf("File type mismatch: expected %s, got %s", uploadType, parsed.Type),
			})
			continue
		}

		// Check file is not empty
		if info.Size() == 0 {
			skipped = append(skipped, UploadResult{
				FilePath: file,
				FileName: filepath.Base(file),
				Status:   StatusSkipped,
				Message:  "File is empty",
			})
			continue
		}

		validFiles = append(validFiles, file)
	}

	return validFiles, skipped
}

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/utils"
)

// ProgressCallback is called to report download progress
type ProgressCallback func(downloaded, total int64)

// DownloadAndReplace downloads a new binary and replaces the current one
func DownloadAndReplace(ctx context.Context, asset *Asset, progress ProgressCallback) error {
	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	logger.Debug("Current executable: %s", execPath)

	// Create temporary file for download
	tempFile, err := os.CreateTemp(filepath.Dir(execPath), "mm-update-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up on any error
	}()

	// Download new binary
	if err := downloadFile(ctx, asset.BrowserDownloadURL, tempFile, asset.Size, progress); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	tempFile.Close()

	// Make the new binary executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempPath, 0755); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	// Backup current binary
	backupPath := execPath + ".backup"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary to place
	if err := os.Rename(tempPath, execPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	logger.Info("Binary updated successfully")
	return nil
}

// downloadFile downloads a file with progress reporting
func downloadFile(ctx context.Context, url string, dest *os.File, expectedSize int64, progress ProgressCallback) error {
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// Send request
	client := utils.NewHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Get content length
	totalSize := resp.ContentLength
	if totalSize <= 0 && expectedSize > 0 {
		totalSize = expectedSize
	}

	// Create progress reader
	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{
			reader:   resp.Body,
			total:    totalSize,
			callback: progress,
		}
	}

	// Copy to destination
	_, err = io.Copy(dest, reader)
	return err
}

// progressReader wraps an io.Reader to report progress
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	callback   ProgressCallback
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)
	if pr.callback != nil {
		pr.callback(pr.downloaded, pr.total)
	}
	return n, err
}

// VerifyChecksum verifies the SHA256 checksum of a file
func VerifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

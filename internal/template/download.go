package template

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/momorph/cli/internal/config"
	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/utils"
)

// ProgressCallback is a function called to report download progress
type ProgressCallback func(downloaded, total int64)

// Download downloads a template from the given URL
func Download(url, checksum string, progress ProgressCallback) (string, error) {
	// Validate URL
	if !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("invalid URL: must use HTTPS")
	}

	// Ensure cache directory exists
	if err := config.EnsureTemplatesDir(); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create temporary file for download
	tempFile, err := os.CreateTemp(config.GetTemplatesDir(), "template-*.zip.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()

	// Cleanup function for error cases
	cleanup := func() {
		tempFile.Close()
		os.Remove(tempPath)
	}

	// Create HTTP client and request
	client := utils.NewHTTPClient()
	resp, err := client.Get(url)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		cleanup()
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Get content length
	totalSize := resp.ContentLength

	// Create progress reader
	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{
			reader:   resp.Body,
			total:    totalSize,
			callback: progress,
		}
	}

	// Create hash writer for checksum verification
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	// Download file
	_, err = io.Copy(multiWriter, reader)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	// Verify checksum if provided
	if checksum != "" {
		computedChecksum := hex.EncodeToString(hasher.Sum(nil))
		if computedChecksum != checksum {
			cleanup()
			return "", fmt.Errorf("checksum mismatch: expected %s, got %s", checksum, computedChecksum)
		}
		logger.Debug("Checksum verified: %s", checksum)
	}

	// Close temp file BEFORE renaming (required on Windows)
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Move temp file to final location
	finalPath := strings.TrimSuffix(tempPath, ".tmp")
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	logger.Info("Downloaded template to: %s", finalPath)
	return finalPath, nil
}

// progressReader wraps an io.Reader to report progress
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	callback   ProgressCallback
	lastReport time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)

	// Report progress (throttle to every 100ms)
	if time.Since(pr.lastReport) > 100*time.Millisecond || err == io.EOF {
		if pr.callback != nil {
			pr.callback(pr.downloaded, pr.total)
		}
		pr.lastReport = time.Now()
	}

	return n, err
}

package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/utils"
)

// ProgressCallback is called to report download progress
type ProgressCallback func(downloaded, total int64)

// DownloadAndReplace downloads a new binary and replaces the current one
// Returns the path of the installed binary on success
func DownloadAndReplace(ctx context.Context, asset *Asset, progress ProgressCallback) (string, error) {
	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	logger.Debug("Current executable: %s", execPath)

	// Create temporary directory for extraction
	tempDir, err := os.MkdirTemp(filepath.Dir(execPath), "mm-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create temporary file for download
	archivePath := filepath.Join(tempDir, asset.Name)
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to create archive file: %w", err)
	}

	// Download archive
	if err := downloadFile(ctx, asset.BrowserDownloadURL, archiveFile, asset.Size, progress); err != nil {
		archiveFile.Close()
		return "", fmt.Errorf("failed to download: %w", err)
	}
	archiveFile.Close()

	// Extract binary from archive
	var binaryPath string
	if strings.HasSuffix(asset.Name, ".tar.gz") || strings.HasSuffix(asset.Name, ".tgz") {
		binaryPath, err = extractTarGz(archivePath, tempDir)
	} else if strings.HasSuffix(asset.Name, ".zip") {
		binaryPath, err = extractZip(archivePath, tempDir)
	} else {
		// Assume it's a raw binary
		binaryPath = archivePath
	}
	if err != nil {
		return "", fmt.Errorf("failed to extract archive: %w", err)
	}

	logger.Debug("Extracted binary: %s", binaryPath)

	// Make the new binary executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return "", fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	// Backup current binary
	backupPath := execPath + ".backup"
	if err := os.Rename(execPath, backupPath); err != nil {
		return "", fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Copy new binary to place (use copy instead of rename for cross-device moves)
	if err := copyFile(binaryPath, execPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, execPath)
		return "", fmt.Errorf("failed to replace binary: %w", err)
	}

	// Set permissions on the final binary
	if runtime.GOOS != "windows" {
		if err := os.Chmod(execPath, 0755); err != nil {
			// Try to restore backup
			os.Remove(execPath)
			os.Rename(backupPath, execPath)
			return "", fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	// Remove backup
	os.Remove(backupPath)

	logger.Info("Binary updated successfully")
	return execPath, nil
}

// extractTarGz extracts a .tar.gz archive and returns the path to the momorph binary
func extractTarGz(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var binaryPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Look for the momorph binary
		name := filepath.Base(header.Name)
		if name == "momorph" || name == "momorph.exe" {
			binaryPath = filepath.Join(destDir, name)
			outFile, err := os.Create(binaryPath)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("momorph binary not found in archive")
	}

	return binaryPath, nil
}

// extractZip extracts a .zip archive and returns the path to the momorph binary
func extractZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var binaryPath string

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name == "momorph" || name == "momorph.exe" {
			binaryPath = filepath.Join(destDir, name)

			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			outFile, err := os.Create(binaryPath)
			if err != nil {
				rc.Close()
				return "", err
			}

			if _, err := io.Copy(outFile, rc); err != nil {
				outFile.Close()
				rc.Close()
				return "", err
			}

			outFile.Close()
			rc.Close()
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("momorph binary not found in archive")
	}

	return binaryPath, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
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

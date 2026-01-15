package vscode

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/momorph/cli/internal/logger"
)

const (
	// LatestVersionURL is the URL to fetch the latest VSIX filename
	LatestVersionURL = "https://vscode.momorph.ai/releases/latest.txt"
	// DownloadBaseURL is the base URL for downloading VSIX files
	DownloadBaseURL = "https://vscode.momorph.ai/releases/"
	// ExtensionName is the installed extension name to check
	ExtensionName = "momorph.vscode-morpheus"
	// HTTPTimeout is the timeout for HTTP requests
	HTTPTimeout = 30 * time.Second
)

// InstallResult represents the result of a VS Code extension installation
type InstallResult struct {
	Installed bool
	Message   string
	Error     error
}

// InstallExtension attempts to install the MoMorph VS Code extension
func InstallExtension() InstallResult {
	// Check if VS Code CLI is available
	codePath, err := findVSCodeCLI()
	if err != nil {
		return InstallResult{
			Installed: false,
			Message:   "VS Code not found, skipping extension installation",
			Error:     nil, // Not an error, just not installed
		}
	}

	// Check if extension is already installed
	if isExtensionInstalled(codePath) {
		return InstallResult{
			Installed: true,
			Message:   "MoMorph extension already installed",
			Error:     nil,
		}
	}

	// Get latest version filename
	vsixFilename, err := getLatestVersion()
	if err != nil {
		logger.Debug("Failed to get latest version: %v", err)
		return InstallResult{
			Installed: false,
			Message:   fmt.Sprintf("Failed to get latest extension version: %v", err),
			Error:     err,
		}
	}

	// Download VSIX file
	vsixPath, err := downloadVSIX(vsixFilename)
	if err != nil {
		logger.Debug("Failed to download VSIX: %v", err)
		return InstallResult{
			Installed: false,
			Message:   fmt.Sprintf("Failed to download extension: %v", err),
			Error:     err,
		}
	}
	defer os.Remove(vsixPath) // Clean up temp file

	// Install the extension
	cmd := exec.Command(codePath, "--install-extension", vsixPath, "--force")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Debug("Extension install stderr: %s", stderr.String())
		return InstallResult{
			Installed: false,
			Message:   fmt.Sprintf("Failed to install extension: %v", err),
			Error:     err,
		}
	}

	return InstallResult{
		Installed: true,
		Message:   "MoMorph VS Code extension installed successfully",
		Error:     nil,
	}
}

// getLatestVersion fetches the latest VSIX filename from the server
func getLatestVersion() (string, error) {
	client := &http.Client{Timeout: HTTPTimeout}

	resp, err := client.Get(LatestVersionURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	filename := strings.TrimSpace(string(body))
	if filename == "" {
		return "", fmt.Errorf("empty response from latest.txt")
	}

	logger.Debug("Latest VSIX version: %s", filename)
	return filename, nil
}

// downloadVSIX downloads the VSIX file to a temporary location
func downloadVSIX(filename string) (string, error) {
	downloadURL := DownloadBaseURL + filename

	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download VSIX: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create temp file
	tempFile, err := os.CreateTemp("", "momorph-*.vsix")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Copy response body to temp file
	_, err = io.Copy(tempFile, resp.Body)
	tempFile.Close()
	if err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to write VSIX file: %w", err)
	}

	logger.Debug("Downloaded VSIX to: %s", tempPath)
	return tempPath, nil
}

// findVSCodeCLI finds the VS Code CLI command based on the platform
func findVSCodeCLI() (string, error) {
	// Common CLI names to try
	cliNames := []string{"code", "code-insiders"}

	// On macOS, also check the full path
	if runtime.GOOS == "darwin" {
		cliNames = append(cliNames,
			"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
			"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code-insiders",
		)
	}

	// On Windows, check common paths
	if runtime.GOOS == "windows" {
		home := os.Getenv("USERPROFILE")
		if home != "" {
			cliNames = append(cliNames,
				filepath.Join(home, "AppData", "Local", "Programs", "Microsoft VS Code", "bin", "code.cmd"),
				filepath.Join(home, "AppData", "Local", "Programs", "Microsoft VS Code Insiders", "bin", "code-insiders.cmd"),
			)
		}
	}

	for _, name := range cliNames {
		// Try exec.LookPath first for names in PATH
		if !strings.HasPrefix(name, "/") && !strings.Contains(name, "\\") {
			path, err := exec.LookPath(name)
			if err == nil {
				logger.Debug("Found VS Code CLI at: %s", path)
				return path, nil
			}
			continue
		}

		// Try the full path directly
		if _, err := os.Stat(name); err == nil {
			logger.Debug("Found VS Code CLI at: %s", name)
			return name, nil
		}
	}

	return "", fmt.Errorf("VS Code CLI not found")
}

// isExtensionInstalled checks if the MoMorph extension is already installed
func isExtensionInstalled(codePath string) bool {
	cmd := exec.Command(codePath, "--list-extensions")
	output, err := cmd.Output()
	if err != nil {
		logger.Debug("Failed to list extensions: %v", err)
		return false
	}

	extensions := strings.Split(string(output), "\n")
	for _, ext := range extensions {
		ext = strings.TrimSpace(ext)
		// Check for momorph extension (case insensitive)
		if strings.Contains(strings.ToLower(ext), "momorph") {
			logger.Debug("Extension already installed: %s", ext)
			return true
		}
	}
	return false
}

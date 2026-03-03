package uv

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/momorph/cli/internal/logger"
)

// InstallResult represents the result of uv installation
type InstallResult struct {
	Installed bool
	Message   string
	Error     error
}

// IsInstalled checks if uv is installed by running "uv --version"
func IsInstalled() bool {
	cmd := exec.Command("uv", "--version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		logger.Debug("uv not found: %v", err)
		return false
	}
	version := strings.TrimSpace(stdout.String())
	logger.Debug("uv found: %s", version)
	return true
}

// Install attempts to install uv using the official installer script
func Install() InstallResult {
	// Check if already installed
	if IsInstalled() {
		return InstallResult{
			Installed: true,
			Message:   "uv is already installed",
			Error:     nil,
		}
	}

	var cmd *exec.Cmd
	var installCmd string

	switch runtime.GOOS {
	case "linux", "darwin":
		// Use curl to download and execute the installer script
		installCmd = "curl -LsSf https://astral.sh/uv/install.sh | sh"
		cmd = exec.Command("sh", "-c", installCmd)
	case "windows":
		// Use PowerShell to download and execute the installer script
		installCmd = "irm https://astral.sh/uv/install.ps1 | iex"
		cmd = exec.Command("powershell", "-Command", installCmd)
	default:
		return InstallResult{
			Installed: false,
			Message:   fmt.Sprintf("Unsupported OS: %s. Please install uv manually: https://docs.astral.sh/uv/getting-started/installation/", runtime.GOOS),
			Error:     fmt.Errorf("unsupported OS: %s", runtime.GOOS),
		}
	}

	logger.Debug("Installing uv with command: %s", installCmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Debug("uv install stdout: %s", stdout.String())
		logger.Debug("uv install stderr: %s", stderr.String())
		return InstallResult{
			Installed: false,
			Message:   fmt.Sprintf("Failed to install uv: %v. Please install manually: https://docs.astral.sh/uv/getting-started/installation/", err),
			Error:     err,
		}
	}

	// Verify installation
	if !IsInstalled() {
		// uv might be installed but not in PATH yet
		return InstallResult{
			Installed: true,
			Message:   "uv installed. You may need to restart your terminal or add ~/.local/bin to PATH",
			Error:     nil,
		}
	}

	return InstallResult{
		Installed: true,
		Message:   "uv installed successfully",
		Error:     nil,
	}
}

// EnsureInstalled checks if uv is installed, and installs it if not
func EnsureInstalled() InstallResult {
	if IsInstalled() {
		return InstallResult{
			Installed: true,
			Message:   "uv is already installed",
			Error:     nil,
		}
	}
	return Install()
}

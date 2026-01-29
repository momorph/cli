package beads

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/uv"
)

// InstallResult represents the result of beads-mcp installation
type InstallResult struct {
	Installed bool
	Message   string
	Error     error
}

// IsInstalled checks if beads-mcp is installed
func IsInstalled() bool {
	// Check if beads-mcp is available via uvx
	cmd := exec.Command("uvx", "beads-mcp", "--version")
	if err := cmd.Run(); err != nil {
		logger.Debug("beads-mcp not found via uvx: %v", err)
		return false
	}
	logger.Debug("beads-mcp is installed")
	return true
}

// Install installs beads-mcp using uv tool install
func Install() InstallResult {
	// First ensure uv is installed
	if !uv.IsInstalled() {
		uvResult := uv.Install()
		if uvResult.Error != nil {
			return InstallResult{
				Installed: false,
				Message:   fmt.Sprintf("Cannot install beads-mcp: uv is required. %s", uvResult.Message),
				Error:     uvResult.Error,
			}
		}
		// If uv was just installed, it might not be in PATH yet
		if !uv.IsInstalled() {
			return InstallResult{
				Installed: false,
				Message:   "uv was installed but is not available in PATH. Please restart your terminal and run 'uv tool install beads-mcp' manually",
				Error:     nil,
			}
		}
	}

	// Install beads-mcp using uv tool install (with packaging dependency)
	logger.Debug("Installing beads-mcp with: uv tool install beads-mcp --with packaging")

	cmd := exec.Command("uv", "tool", "install", "beads-mcp", "--with", "packaging")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if it's already installed (uv returns error if already installed)
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "already installed") || strings.Contains(stdout.String(), "already installed") {
			return InstallResult{
				Installed: true,
				Message:   "beads-mcp is already installed",
				Error:     nil,
			}
		}

		logger.Debug("beads-mcp install stdout: %s", stdout.String())
		logger.Debug("beads-mcp install stderr: %s", stderrStr)
		return InstallResult{
			Installed: false,
			Message:   fmt.Sprintf("Failed to install beads-mcp: %v", err),
			Error:     err,
		}
	}

	return InstallResult{
		Installed: true,
		Message:   "beads-mcp installed successfully",
		Error:     nil,
	}
}

// EnsureInstalled ensures beads-mcp is installed
func EnsureInstalled() InstallResult {
	// First ensure uv is available
	if !uv.IsInstalled() {
		uvResult := uv.EnsureInstalled()
		if uvResult.Error != nil || !uvResult.Installed {
			return InstallResult{
				Installed: false,
				Message:   fmt.Sprintf("Cannot install beads-mcp: %s", uvResult.Message),
				Error:     uvResult.Error,
			}
		}
	}

	return Install()
}

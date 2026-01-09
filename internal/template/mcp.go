package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/momorph/cli/internal/logger"
)

// ConfigUpdater defines the interface for updating AI tool specific configs
type ConfigUpdater interface {
	UpdateGitHubToken(projectDir, githubToken string) error
}

// ClaudeMCPConfig represents the structure of Claude's .mcp.json file
type ClaudeMCPConfig struct {
	Servers map[string]ClaudeMCPServer `json:"mcpServers"`
}

// ClaudeMCPServer represents a Claude MCP server configuration
type ClaudeMCPServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// claudeConfigUpdater handles Claude-specific config updates
type claudeConfigUpdater struct{}

// UpdateGitHubToken updates the GitHub token in Claude's .mcp.json file
// This function preserves all existing fields and only updates the x-mcp-x-github-token value
func (c *claudeConfigUpdater) UpdateGitHubToken(projectDir, githubToken string) error {
	mcpFilePath := filepath.Join(projectDir, ".mcp.json")

	// Check if .mcp.json exists
	if _, err := os.Stat(mcpFilePath); os.IsNotExist(err) {
		logger.Debug("No .mcp.json file found for Claude, skipping GitHub token update")
		return nil // Not an error, just skip
	}

	// Read .mcp.json file
	data, err := os.ReadFile(mcpFilePath)
	if err != nil {
		return fmt.Errorf("failed to read .mcp.json: %w", err)
	}

	// Parse JSON as generic map to preserve all fields
	var mcpConfig map[string]interface{}
	if err := json.Unmarshal(data, &mcpConfig); err != nil {
		return fmt.Errorf("failed to parse .mcp.json: %w", err)
	}

	// Navigate to mcpServers
	serversInterface, exists := mcpConfig["mcpServers"]
	if !exists {
		logger.Debug("No 'mcpServers' field found in .mcp.json, skipping GitHub token update")
		return nil
	}

	servers, ok := serversInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("mcpServers is not a valid object")
	}

	// Check if momorph server exists
	momorphInterface, exists := servers["momorph"]
	if !exists {
		logger.Debug("No 'momorph' server found in .mcp.json, skipping GitHub token update")
		return nil // Not an error, just skip
	}

	momorphServer, ok := momorphInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("momorph server is not a valid object")
	}

	// Get or create headers
	var headers map[string]interface{}
	headersInterface, exists := momorphServer["headers"]
	if !exists || headersInterface == nil {
		headers = make(map[string]interface{})
		momorphServer["headers"] = headers
	} else {
		headers, ok = headersInterface.(map[string]interface{})
		if !ok {
			return fmt.Errorf("momorph headers is not a valid object")
		}
	}

	// Update only the GitHub token field
	headers["x-mcp-x-github-token"] = githubToken

	// Marshal back to JSON with indentation
	updatedData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal .mcp.json: %w", err)
	}

	// Write back to file
	if err := os.WriteFile(mcpFilePath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write .mcp.json: %w", err)
	}

	logger.Info("Updated GitHub token in Claude's .mcp.json")
	return nil
}

// copilotConfigUpdater handles Copilot-specific config updates (placeholder for future)
type copilotConfigUpdater struct{}

// UpdateGitHubToken updates Copilot config (not implemented yet)
func (c *copilotConfigUpdater) UpdateGitHubToken(projectDir, githubToken string) error {
	logger.Debug("Copilot config update not yet implemented, skipping")
	// TODO: Implement when Copilot MCP config format is available
	return nil
}

// cursorConfigUpdater handles Cursor-specific config updates (placeholder for future)
type cursorConfigUpdater struct{}

// UpdateGitHubToken updates Cursor config (not implemented yet)
func (c *cursorConfigUpdater) UpdateGitHubToken(projectDir, githubToken string) error {
	logger.Debug("Cursor config update not yet implemented, skipping")
	// TODO: Implement when Cursor MCP config format is available
	return nil
}

// GetConfigUpdater returns the appropriate config updater for the given AI tool
func GetConfigUpdater(aiTool string) ConfigUpdater {
	switch aiTool {
	case "claude":
		return &claudeConfigUpdater{}
	case "copilot":
		return &copilotConfigUpdater{}
	case "cursor":
		return &cursorConfigUpdater{}
	default:
		logger.Warn("Unknown AI tool: %s, no config updater available", aiTool)
		return nil
	}
}

// UpdateAIToolConfig updates the AI tool config with GitHub token
// This is the main entry point that delegates to the specific updater
func UpdateAIToolConfig(aiTool, projectDir, githubToken string) error {
	updater := GetConfigUpdater(aiTool)
	if updater == nil {
		return fmt.Errorf("no config updater available for AI tool: %s", aiTool)
	}

	return updater.UpdateGitHubToken(projectDir, githubToken)
}

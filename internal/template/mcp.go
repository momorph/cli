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
	ConfigureMCPServer(projectDir, githubToken, mcpServerEndpoint string) error
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

// ConfigureMCPServer updates the GitHub token in Claude's .mcp.json file
// This function preserves all existing fields and only updates the x-github-token value
func (c *claudeConfigUpdater) ConfigureMCPServer(projectDir, githubToken, mcpServerEndpoint string) error {
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

	// Update the GitHub token field
	headers["x-github-token"] = githubToken

	// Update the URL field with the MCP server endpoint
	momorphServer["url"] = mcpServerEndpoint

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

// ConfigureMCPServer updates Copilot config (not implemented yet)
func (c *copilotConfigUpdater) ConfigureMCPServer(projectDir, githubToken, mcpServerEndpoint string) error {
	logger.Debug("MCP servers are integrated via MoMorph VSCode Extension, skipping Copilot config update")
	return nil
}

// cursorConfigUpdater handles Cursor-specific config updates
type cursorConfigUpdater struct{}

// ConfigureMCPServer updates Cursor's global mcp.json with servers from project's .mcp.json
// Config file: ~/.cursor/mcp.json
func (c *cursorConfigUpdater) ConfigureMCPServer(projectDir, githubToken, mcpServerEndpoint string) error {
	// Cursor config is in user's home directory, not project directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	cursorDir := filepath.Join(homeDir, ".cursor")
	mcpFilePath := filepath.Join(cursorDir, "mcp.json")

	// Ensure .cursor directory exists
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		return fmt.Errorf("failed to create .cursor directory: %w", err)
	}

	// Read existing global config or create new one
	var mcpConfig map[string]interface{}
	if data, err := os.ReadFile(mcpFilePath); err == nil {
		if err := json.Unmarshal(data, &mcpConfig); err != nil {
			// If parsing fails, start fresh but log warning
			logger.Warn("Failed to parse existing Cursor mcp.json, creating new: %v", err)
			mcpConfig = make(map[string]interface{})
		}
	} else {
		mcpConfig = make(map[string]interface{})
	}

	// Get or create mcpServers in global config
	var globalServers map[string]interface{}
	if serversInterface, exists := mcpConfig["mcpServers"]; exists {
		globalServers, _ = serversInterface.(map[string]interface{})
	}
	if globalServers == nil {
		globalServers = make(map[string]interface{})
		mcpConfig["mcpServers"] = globalServers
	}

	// Read project's .mcp.json and merge servers
	projectMCPPath := filepath.Join(projectDir, ".mcp.json")
	if projectServers, err := readMCPServers(projectMCPPath); err == nil {
		for name, server := range projectServers {
			// Skip if server already exists in global config (don't overwrite user's config)
			if _, exists := globalServers[name]; !exists {
				globalServers[name] = server
				logger.Debug("Added server '%s' to Cursor config", name)
			}
		}
	} else {
		logger.Debug("Could not read project .mcp.json for Cursor: %v", err)
	}

	// Always update momorph server with current token and endpoint
	globalServers["momorph"] = map[string]interface{}{
		"url": mcpServerEndpoint,
		"headers": map[string]string{
			"x-github-token": githubToken,
		},
	}

	// Write back to file
	updatedData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Cursor mcp.json: %w", err)
	}

	if err := os.WriteFile(mcpFilePath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write Cursor mcp.json: %w", err)
	}

	logger.Info("Updated MoMorph config in Cursor's mcp.json at %s", mcpFilePath)
	return nil
}

// windsurfConfigUpdater handles Windsurf-specific config updates
type windsurfConfigUpdater struct{}

// ConfigureMCPServer updates Windsurf's global mcp_config.json with servers from project's .mcp.json
// Config file: ~/.codeium/windsurf/mcp_config.json
func (w *windsurfConfigUpdater) ConfigureMCPServer(projectDir, githubToken, mcpServerEndpoint string) error {
	// Windsurf config is in user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	windsurfDir := filepath.Join(homeDir, ".codeium", "windsurf")
	mcpFilePath := filepath.Join(windsurfDir, "mcp_config.json")

	// Ensure directory exists
	if err := os.MkdirAll(windsurfDir, 0755); err != nil {
		return fmt.Errorf("failed to create windsurf config directory: %w", err)
	}

	// Read existing global config or create new one
	var mcpConfig map[string]interface{}
	if data, err := os.ReadFile(mcpFilePath); err == nil {
		if err := json.Unmarshal(data, &mcpConfig); err != nil {
			logger.Warn("Failed to parse existing Windsurf mcp_config.json, creating new: %v", err)
			mcpConfig = make(map[string]interface{})
		}
	} else {
		mcpConfig = make(map[string]interface{})
	}

	// Get or create mcpServers in global config
	var globalServers map[string]interface{}
	if serversInterface, exists := mcpConfig["mcpServers"]; exists {
		globalServers, _ = serversInterface.(map[string]interface{})
	}
	if globalServers == nil {
		globalServers = make(map[string]interface{})
		mcpConfig["mcpServers"] = globalServers
	}

	// Read project's .mcp.json and merge servers (with key transformation)
	projectMCPPath := filepath.Join(projectDir, ".mcp.json")
	if projectServers, err := readMCPServers(projectMCPPath); err == nil {
		for name, server := range projectServers {
			// Skip if server already exists in global config (don't overwrite user's config)
			if _, exists := globalServers[name]; !exists {
				// Transform server config for Windsurf (url -> serverUrl)
				if serverMap, ok := server.(map[string]interface{}); ok {
					globalServers[name] = transformServerForWindsurf(serverMap)
				} else {
					globalServers[name] = server
				}
				logger.Debug("Added server '%s' to Windsurf config", name)
			}
		}
	} else {
		logger.Debug("Could not read project .mcp.json for Windsurf: %v", err)
	}

	// Always update momorph server with current token and endpoint
	// Windsurf uses "serverUrl" instead of "url"
	globalServers["momorph"] = map[string]interface{}{
		"serverUrl": mcpServerEndpoint,
		"headers": map[string]string{
			"x-github-token": githubToken,
		},
	}

	// Write back to file
	updatedData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Windsurf mcp_config.json: %w", err)
	}

	if err := os.WriteFile(mcpFilePath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write Windsurf mcp_config.json: %w", err)
	}

	logger.Info("Updated MoMorph config in Windsurf's mcp_config.json at %s", mcpFilePath)
	return nil
}

// readMCPServers reads the mcpServers from a .mcp.json file
func readMCPServers(mcpFilePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(mcpFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var mcpConfig map[string]interface{}
	if err := json.Unmarshal(data, &mcpConfig); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	serversInterface, exists := mcpConfig["mcpServers"]
	if !exists {
		return nil, fmt.Errorf("no mcpServers field found")
	}

	servers, ok := serversInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("mcpServers is not a valid object")
	}

	return servers, nil
}

// transformServerForWindsurf converts server config for Windsurf (url -> serverUrl)
func transformServerForWindsurf(server map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range server {
		if k == "url" {
			result["serverUrl"] = v
		} else {
			result[k] = v
		}
	}
	return result
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
	case "windsurf":
		return &windsurfConfigUpdater{}
	default:
		logger.Warn("Unknown AI tool: %s, no config updater available", aiTool)
		return nil
	}
}

// UpdateAIToolConfig updates the AI tool config with GitHub token
// This is the main entry point that delegates to the specific updater
func UpdateAIToolConfig(aiTool, projectDir, githubToken, mcpServerEndpoint string) error {
	updater := GetConfigUpdater(aiTool)
	if updater == nil {
		return fmt.Errorf("no config updater available for AI tool: %s", aiTool)
	}

	return updater.ConfigureMCPServer(projectDir, githubToken, mcpServerEndpoint)
}

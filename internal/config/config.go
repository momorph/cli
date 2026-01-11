package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// UserConfig represents CLI configuration
type UserConfig struct {
	APIEndpoint        string    `json:"api_endpoint"`
	MCPServerEndpoint  string    `json:"mcp_server_endpoint"`
	DefaultAITool      string    `json:"default_ai_tool"`
	LogLevel           string    `json:"log_level"`
	LastUpdateCheck    time.Time `json:"last_update_check"`
	UpdateCheckEnabled bool      `json:"update_check_enabled"`
	TelemetryEnabled   bool      `json:"telemetry_enabled"`
	ConfigVersion      string    `json:"config_version"`
	// Basic Auth credentials (not persisted to disk, loaded from env vars only)
	BasicAuthUsername string `json:"-"`
	BasicAuthPassword string `json:"-"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *UserConfig {
	apiEndpoint := "https://momorph.ai"

	// Check for environment-based endpoint override
	if env := os.Getenv("MOMORPH_ENV"); env == "staging" || env == "stg" {
		apiEndpoint = "https://stg.momorph.com"
	}

	// Allow direct override via MOMORPH_API_ENDPOINT
	if endpoint := os.Getenv("MOMORPH_API_ENDPOINT"); endpoint != "" {
		apiEndpoint = endpoint
	}

	return &UserConfig{
		APIEndpoint:        apiEndpoint,
		MCPServerEndpoint:  "https://momorph.ai/mcp",
		DefaultAITool:      "", // Prompt user
		LogLevel:           "info",
		LastUpdateCheck:    time.Time{},
		UpdateCheckEnabled: true,
		TelemetryEnabled:   false,
		ConfigVersion:      "1.0",
		// Load Basic Auth from environment (never saved to disk for security)
		BasicAuthUsername: os.Getenv("MOMORPH_BASIC_AUTH_USERNAME"),
		BasicAuthPassword: os.Getenv("MOMORPH_BASIC_AUTH_PASSWORD"),
	}
}

// Load loads the configuration from disk, or returns default if not found
func Load() (*UserConfig, error) {
	configFile := GetConfigFile()

	// Return default config if file doesn't exist
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var config UserConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Always load Basic Auth from environment (never persisted to disk)
	config.BasicAuthUsername = os.Getenv("MOMORPH_BASIC_AUTH_USERNAME")
	config.BasicAuthPassword = os.Getenv("MOMORPH_BASIC_AUTH_PASSWORD")

	return &config, nil
}

// Save saves the configuration to disk with atomic write
func (c *UserConfig) Save() error {
	// Ensure config directory exists
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	configFile := GetConfigFile()

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// Write to temporary file first (atomic write pattern)
	tempFile := configFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return err
	}

	// Rename temp file to actual config file (atomic operation)
	if err := os.Rename(tempFile, configFile); err != nil {
		os.Remove(tempFile) // Clean up temp file on error
		return err
	}

	return nil
}

// Validate validates the configuration
func (c *UserConfig) Validate() error {
	// Validate API endpoint
	if c.APIEndpoint == "" {
		return os.ErrInvalid
	}

	// Validate AI tool if set
	if c.DefaultAITool != "" {
		validTools := map[string]bool{
			"copilot": true,
			"cursor":  true,
			"claude":  true,
		}
		if !validTools[c.DefaultAITool] {
			return os.ErrInvalid
		}
	}

	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.LogLevel] {
		return os.ErrInvalid
	}

	return nil
}

// GetAPIEndpoint returns the API endpoint with version path
func (c *UserConfig) GetAPIEndpoint() string {
	return c.APIEndpoint
}

// GetTemplateEndpoint returns the full template API endpoint
func (c *UserConfig) GetTemplateEndpoint() string {
	return filepath.Join(c.APIEndpoint, "api", "v1", "get-project-template")
}

// HasBasicAuth checks if Basic Auth credentials are configured
func (c *UserConfig) HasBasicAuth() bool {
	return c.BasicAuthUsername != "" && c.BasicAuthPassword != ""
}

// IsStaging checks if the current environment is staging
func (c *UserConfig) IsStaging() bool {
	env := os.Getenv("MOMORPH_ENV")
	return env == "staging" || env == "stg" || c.HasBasicAuth()
}

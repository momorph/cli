package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"

	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/version"
)

// TemplateMetadata represents template information from the API
type TemplateMetadata struct {
	Key         string `json:"key"`       // S3 key path
	DownloadURL string `json:"url"`       // Presigned URL
	ExpiresIn   int    `json:"expiresIn"` // URL expiration in seconds
	Cached      bool   `json:"cached"`    // Whether response was cached
}

// APIErrorResponse represents an error response from the API
type APIErrorResponse struct {
	Message string `json:"message"`
	Key     string `json:"key"`
}

// GetProjectTemplate retrieves template metadata for the specified AI tool
func (c *Client) GetProjectTemplate(ctx context.Context, aiTool string) (*TemplateMetadata, error) {
	// Validate AI tool
	validTools := map[string]bool{
		"copilot": true,
		"cursor":  true,
		"claude":  true,
	}
	if !validTools[aiTool] {
		return nil, fmt.Errorf("invalid AI tool: %s (must be one of: copilot, cursor, claude)", aiTool)
	}

	// Determine shell based on OS
	// API accepts: sh (Unix/Linux/macOS) or ps (PowerShell/Windows)
	shell := "sh"
	if runtime.GOOS == "windows" {
		shell = "ps"
	}

	// Determine version parameter
	// Use "stable" for production releases, "latest" for development builds
	versionParam := "stable"
	if version.Version == "" || version.Version == "dev" {
		versionParam = "latest"
	}

	// Build path with query parameters for BFF endpoint
	// Format: /g/bff/api/project-template/presign?agent=copilot&shell=sh&version=stable
	// version can be: stable (production release) or latest (including pre-releases)
	path := fmt.Sprintf("/g/bff/api/project-template/presign?agent=%s&shell=%s&version=%s", aiTool, shell, versionParam)

	// Make request
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log response body for debugging
	logger.Debug("API Response Status: %d", resp.StatusCode)
	logger.Debug("API Response Body: %s", string(bodyBytes))

	// Check if response is an error (e.g., 404 Object not found)
	if resp.StatusCode >= 400 {
		var apiError APIErrorResponse
		if err := json.Unmarshal(bodyBytes, &apiError); err == nil && apiError.Message != "" {
			// Return a more user-friendly error message
			if apiError.Message == "Object not found" {
				return nil, fmt.Errorf("template not available on server yet (version=%s, agent=%s, shell=%s)\nThe template file may not be uploaded to S3 yet.\nPlease contact the MoMorph team or try again later", versionParam, aiTool, shell)
			}
			return nil, fmt.Errorf("API error (%d): %s (key: %s)", resp.StatusCode, apiError.Message, apiError.Key)
		}
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var template TemplateMetadata
	if err := json.Unmarshal(bodyBytes, &template); err != nil {
		logger.Debug("Failed to parse response. Body: %s", string(bodyBytes))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Log parsed struct fields for debugging
	logger.Debug("Parsed TemplateMetadata:")
	logger.Debug("  Key: %s", template.Key)
	logger.Debug("  DownloadURL: %s", template.DownloadURL)
	logger.Debug("  ExpiresIn: %d", template.ExpiresIn)
	logger.Debug("  Cached: %v", template.Cached)
	logger.Debug("  DownloadURL empty?: %v", template.DownloadURL == "")

	// Validate response
	if template.DownloadURL == "" {
		logger.Debug("DownloadURL is empty after unmarshaling")
		logger.Debug("Raw response bytes: %v", bodyBytes)
		return nil, fmt.Errorf("API returned empty download URL")
	}

	return &template, nil
}

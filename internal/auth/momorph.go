package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/momorph/cli/internal/config"
)

// MoMorphTokenResponse represents the response from MoMorph token exchange
type MoMorphTokenResponse struct {
	Token string `json:"token"`
}

// ExchangeGitHubToken exchanges a GitHub OAuth token for a MoMorph platform token
func ExchangeGitHubToken(ctx context.Context, githubToken string) (*MoMorphTokenResponse, error) {
	// Load config to get API endpoint
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Build API endpoint - use BFF endpoint
	endpoint := cfg.GetAPIEndpoint() + "/g/bff/api/sessions/token"

	// Create GET request (no body needed)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-github-token", githubToken)
	req.Header.Set("User-Agent", "MoMorph-CLI/1.0.0")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle error responses
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)

		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("invalid GitHub token")
		case http.StatusForbidden:
			return nil, fmt.Errorf("access denied: you may not have permission to use MoMorph")
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("rate limit exceeded, please try again later")
		default:
			return nil, fmt.Errorf("MoMorph API error (status %d): %s", resp.StatusCode, string(body))
		}
	}

	// Parse response
	var tokenResp MoMorphTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &tokenResp, nil
}

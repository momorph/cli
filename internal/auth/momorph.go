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

// MoMorphUser represents a MoMorph user from the whoami API
type MoMorphUser struct {
	ID                string
	Email             string
	Username          string
	AvatarURL         string
	CreatedAt         string
	TimeZone          string
	ConnectedAccounts []ConnectedAccount
}

// ConnectedAccount represents a connected OAuth account
type ConnectedAccount struct {
	ID             int    `json:"id"`
	MorpheusUserID int    `json:"morpheus_user_id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Provider       string `json:"provider"`
	ProviderID     string `json:"provider_id"`
	PhotoURL       string `json:"photo_url"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// WhoAmIUser represents user data in the whoami response
type WhoAmIUser struct {
	ID                int                `json:"id"`
	Email             string             `json:"email"`
	LastActiveFileKey string             `json:"last_active_file_key"`
	LastActiveAt      string             `json:"last_active_at"`
	DefaultLanguage   string             `json:"default_language"`
	TimeZone          string             `json:"time_zone"`
	CreatedAt         string             `json:"created_at"`
	UpdatedAt         string             `json:"updated_at"`
	ConnectedAccounts []ConnectedAccount `json:"connected_accounts"`
}

// WhoAmIExtra represents the extra field in whoami response
type WhoAmIExtra struct {
	Provider string     `json:"provider"`
	User     WhoAmIUser `json:"user"`
}

// WhoAmIResponse represents the full whoami API response
type WhoAmIResponse struct {
	Subject int         `json:"subject"`
	Extra   WhoAmIExtra `json:"extra"`
}

// GetMoMorphUser fetches the authenticated user information from MoMorph API
func GetMoMorphUser(ctx context.Context, githubToken string) (*MoMorphUser, error) {
	// Load config to get API endpoint
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Build API endpoint
	endpoint := cfg.GetAPIEndpoint() + "/api/sessions/whoami"

	// Create GET request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
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
	if resp.StatusCode != http.StatusOK {
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
	var response WhoAmIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract user information from extra.user only
	user := &MoMorphUser{
		ID:                fmt.Sprintf("%d", response.Extra.User.ID),
		Email:             response.Extra.User.Email,
		Username:          response.Extra.User.Email, // Use email as username since it's not in extra.user
		AvatarURL:         "",                        // Not available in extra.user
		CreatedAt:         response.Extra.User.CreatedAt,
		TimeZone:          response.Extra.User.TimeZone,
		ConnectedAccounts: response.Extra.User.ConnectedAccounts,
	}

	return user, nil
}

// MoMorphTokenResponse represents the response from MoMorph token exchange
// Deprecated: No longer needed, use GitHub token directly
type MoMorphTokenResponse struct {
	Token string `json:"token"`
}

// ExchangeGitHubToken exchanges a GitHub OAuth token for a MoMorph platform token
// Deprecated: No longer needed, use GitHub token directly
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

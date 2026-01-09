package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// DeviceCodeResponse represents GitHub's device code response
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse represents GitHub's token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

const (
	// GitHub OAuth endpoints
	deviceCodeURL  = "https://github.com/login/device/code"
	accessTokenURL = "https://github.com/login/oauth/access_token"

	// Default GitHub OAuth client ID for device flow (organization app)
	// Can be overridden by setting MOMORPH_GITHUB_CLIENT_ID environment variable
	defaultClientID = "Ov23lihLTJKLFI2LJfq1"
)

// getClientID returns the GitHub OAuth client ID
// Priority: Environment variable > Default value
func getClientID() string {
	if envClientID := os.Getenv("MOMORPH_GITHUB_CLIENT_ID"); envClientID != "" {
		return envClientID
	}
	return defaultClientID
}

// RequestDeviceCode requests a device code from GitHub
func RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	// Prepare request body
	reqBody := map[string]string{
		"client_id": getClientID(),
		"scope":     "read:user",
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", deviceCodeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var deviceCode DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &deviceCode, nil
}

// PollForToken polls GitHub for the access token
func PollForToken(ctx context.Context, deviceCode string, interval int) (*TokenResponse, error) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := checkToken(ctx, deviceCode)
			if err != nil {
				return nil, err
			}
			
			// Check for errors
			if token.Error != "" {
				switch token.Error {
				case "authorization_pending":
					// Continue polling
					continue
				case "slow_down":
					// Increase interval
					ticker.Reset(time.Duration(interval+5) * time.Second)
					continue
				case "expired_token":
					return nil, fmt.Errorf("device code expired")
				case "access_denied":
					return nil, fmt.Errorf("authorization denied by user")
				default:
					return nil, fmt.Errorf("authorization error: %s - %s", token.Error, token.ErrorDesc)
				}
			}
			
			// Success
			if token.AccessToken != "" {
				return token, nil
			}
		}
	}
}

// checkToken checks the token status with GitHub
func checkToken(ctx context.Context, deviceCode string) (*TokenResponse, error) {
	// Prepare request body
	data := url.Values{}
	data.Set("client_id", getClientID())
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", accessTokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &token, nil
}

package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/momorph/cli/internal/auth"
	"github.com/momorph/cli/internal/config"
	"github.com/momorph/cli/internal/utils"
)

// Client represents a MoMorph API client
type Client struct {
	baseURL    string
	config     *config.UserConfig
	httpClient *http.Client
}

// NewClient creates a new MoMorph API client
func NewClient() (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &Client{
		baseURL:    cfg.GetAPIEndpoint(),
		config:     cfg,
		httpClient: utils.NewHTTPClient(),
	}, nil
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	// Load token
	token, err := auth.LoadToken()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}

	// Check if token is valid
	if !token.IsValid() {
		return nil, fmt.Errorf("token expired, please run 'momorph login' to reauthenticate")
	}

	// Build URL
	url := c.baseURL + path

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - Always include GitHub token
	req.Header.Set("x-github-token", token.GitHubToken)

	// Set Authorization header based on environment
	if c.config.IsStaging() {
		// Staging: Use Basic Auth
		if c.config.HasBasicAuth() {
			credentials := c.config.BasicAuthUsername + ":" + c.config.BasicAuthPassword
			encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
			req.Header.Set("Authorization", "Basic "+encoded)
		} else {
			return nil, fmt.Errorf("staging environment requires MOMORPH_BASIC_AUTH_USERNAME and MOMORPH_BASIC_AUTH_PASSWORD")
		}
	}
	// Production: x-github-token header is sufficient, no Bearer token needed

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MoMorph-CLI/1.0.0")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Don't handle errors here - let the caller decide how to handle them
	// This allows template.go to parse the JSON error response properly
	return resp, nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, "GET", path, nil)
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ParseError parses an error response
func ParseError(resp *http.Response) error {
	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		return fmt.Errorf("failed to parse error response")
	}

	if errResp.Message != "" {
		return fmt.Errorf("%s", errResp.Message)
	}
	return fmt.Errorf("%s", errResp.Error)
}

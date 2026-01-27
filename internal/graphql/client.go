package graphql

import (
	"bytes"
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

// Client represents a GraphQL client for MoMorph API
type Client struct {
	endpoint   string
	config     *config.UserConfig
	httpClient *http.Client
}

// Request represents a GraphQL request
type Request struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// Response represents a GraphQL response
type Response struct {
	Data   json.RawMessage `json:"data,omitempty"`
	Errors []Error         `json:"errors,omitempty"`
}

// Error represents a GraphQL error
type Error struct {
	Message    string                 `json:"message"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// NewClient creates a new GraphQL client
func NewClient() (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	endpoint := cfg.GetAPIEndpoint() + "/g/bff/v1/graphql"

	return &Client{
		endpoint:   endpoint,
		config:     cfg,
		httpClient: utils.NewHTTPClient(),
	}, nil
}

// Execute executes a GraphQL query or mutation
func (c *Client) Execute(ctx context.Context, query string, variables map[string]interface{}) (*Response, error) {
	// Load token
	token, err := auth.LoadToken()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}

	// Check if token is valid
	if !token.IsValid() {
		return nil, fmt.Errorf("token expired, please run 'momorph login' to reauthenticate")
	}

	// Build request body
	reqBody := Request{
		Query:     query,
		Variables: variables,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MoMorph-CLI/1.0.0")
	req.Header.Set("x-github-token", token.GitHubToken)

	// Set Authorization header for staging environment
	if c.config.IsStaging() {
		if c.config.HasBasicAuth() {
			credentials := c.config.BasicAuthUsername + ":" + c.config.BasicAuthPassword
			encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
			req.Header.Set("Authorization", "Basic "+encoded)
		} else {
			return nil, fmt.Errorf("staging environment requires MOMORPH_BASIC_AUTH_USERNAME and MOMORPH_BASIC_AUTH_PASSWORD")
		}
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var gqlResp Response
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for GraphQL errors
	if len(gqlResp.Errors) > 0 {
		return &gqlResp, fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	return &gqlResp, nil
}

// ExecuteWithResult executes a GraphQL query and unmarshals the result
func (c *Client) ExecuteWithResult(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	resp, err := c.Execute(ctx, query, variables)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(resp.Data, result); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return nil
}

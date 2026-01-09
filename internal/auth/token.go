package auth

import (
	"time"
)

// AuthToken stores both GitHub OAuth token and MoMorph platform token
type AuthToken struct {
	// GitHub OAuth Token
	GitHubToken     string   `json:"github_token"`
	GitHubTokenType string   `json:"github_token_type"`
	GitHubScopes    []string `json:"github_scopes"`

	// MoMorph Platform Token
	MoMorphToken     string    `json:"momorph_token"`
	MoMorphExpiresAt time.Time `json:"momorph_expires_at"`

	// User Information
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
	UserID    string `json:"user_id"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsValid checks if the MoMorph token is still valid
func (t *AuthToken) IsValid() bool {
	if t.MoMorphToken == "" {
		return false
	}
	// Check if token has expired
	return time.Now().Before(t.MoMorphExpiresAt)
}

// NeedsRefresh checks if the token needs refresh (expires within 24 hours)
func (t *AuthToken) NeedsRefresh() bool {
	if t.MoMorphToken == "" {
		return true
	}
	// Refresh if expires within 24 hours
	return time.Now().Add(24 * time.Hour).After(t.MoMorphExpiresAt)
}

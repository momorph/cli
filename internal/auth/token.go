package auth

// AuthToken stores GitHub OAuth token for MoMorph authentication
type AuthToken struct {
	// GitHub OAuth Token (used directly with MoMorph API)
	GitHubToken string `json:"github_token"`
}

// IsValid checks if the GitHub token exists
func (t *AuthToken) IsValid() bool {
	return t.GitHubToken != ""
}

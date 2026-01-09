package auth

import (
	"encoding/json"

	"github.com/99designs/keyring"
)

const (
	keyringService = "momorph-cli"
	keyringKey     = "auth_token"
)

// SaveToken saves the authentication token to the OS credential manager
func SaveToken(token *AuthToken) error {
	// Open keyring
	ring, err := keyring.Open(keyring.Config{
		ServiceName:              keyringService,
		AllowedBackends:          []keyring.BackendType{keyring.KeychainBackend, keyring.SecretServiceBackend, keyring.WinCredBackend, keyring.FileBackend},
		KeychainTrustApplication: true,
		FileDir:                  "~/.config/momorph",
	})
	if err != nil {
		return err
	}

	// Marshal token to JSON
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	// Store in keyring
	return ring.Set(keyring.Item{
		Key:  keyringKey,
		Data: data,
	})
}

// LoadToken loads the authentication token from the OS credential manager
func LoadToken() (*AuthToken, error) {
	// Open keyring
	ring, err := keyring.Open(keyring.Config{
		ServiceName:              keyringService,
		AllowedBackends:          []keyring.BackendType{keyring.KeychainBackend, keyring.SecretServiceBackend, keyring.WinCredBackend, keyring.FileBackend},
		KeychainTrustApplication: true,
		FileDir:                  "~/.config/momorph",
	})
	if err != nil {
		return nil, err
	}

	// Get from keyring
	item, err := ring.Get(keyringKey)
	if err != nil {
		return nil, err
	}

	// Unmarshal token from JSON
	var token AuthToken
	if err := json.Unmarshal(item.Data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// ClearToken removes the authentication token from the OS credential manager
func ClearToken() error {
	// Open keyring
	ring, err := keyring.Open(keyring.Config{
		ServiceName:              keyringService,
		AllowedBackends:          []keyring.BackendType{keyring.KeychainBackend, keyring.SecretServiceBackend, keyring.WinCredBackend, keyring.FileBackend},
		KeychainTrustApplication: true,
		FileDir:                  "~/.config/momorph",
	})
	if err != nil {
		return err
	}

	// Remove from keyring
	return ring.Remove(keyringKey)
}

// IsAuthenticated checks if a valid token exists
func IsAuthenticated() bool {
	token, err := LoadToken()
	if err != nil {
		return false
	}
	return token.IsValid()
}

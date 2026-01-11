package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
)

const (
	keyringService = "momorph-cli"
	keyringKey     = "auth_token"
)

// getKeyringConfig returns a keyring configuration that works with CGO_ENABLED=0
func getKeyringConfig() keyring.Config {
	// Get config directory
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "momorph")
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		configDir = filepath.Join(xdgConfig, "momorph")
	}

	// Create a deterministic password based on machine ID and home directory
	// This allows the file backend to work without prompting for a password
	machineID := getMachineID()
	password := sha256.Sum256([]byte(machineID + os.Getenv("HOME")))

	return keyring.Config{
		ServiceName: keyringService,
		// When CGO_ENABLED=0, prefer FileBackend as it doesn't require C libraries
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,      // macOS (requires CGO)
			keyring.SecretServiceBackend, // Linux (requires CGO)
			keyring.WinCredBackend,       // Windows
			keyring.FileBackend,          // Fallback for all platforms
		},
		KeychainTrustApplication: true,
		FileDir:                  configDir,
		// Provide a password function to avoid prompting
		FilePasswordFunc: func(prompt string) (string, error) {
			return hex.EncodeToString(password[:]), nil
		},
	}
}

// getMachineID returns a unique identifier for the current machine
func getMachineID() string {
	// Try to read machine-id from various locations
	paths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}

	for _, path := range paths {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}

	// Fallback to hostname
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return "default-machine-id"
}

// SaveToken saves the authentication token to the OS credential manager
func SaveToken(token *AuthToken) error {
	// Open keyring
	ring, err := keyring.Open(getKeyringConfig())
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
	ring, err := keyring.Open(getKeyringConfig())
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
	ring, err := keyring.Open(getKeyringConfig())
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

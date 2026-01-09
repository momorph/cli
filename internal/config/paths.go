package config

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// GetConfigDir returns the configuration directory path
func GetConfigDir() string {
	return filepath.Join(xdg.ConfigHome, "momorph")
}

// GetConfigFile returns the configuration file path
func GetConfigFile() string {
	return filepath.Join(GetConfigDir(), "config.json")
}

// GetCacheDir returns the cache directory path
func GetCacheDir() string {
	return filepath.Join(xdg.CacheHome, "momorph")
}

// GetTemplatesDir returns the templates cache directory path
func GetTemplatesDir() string {
	return filepath.Join(GetCacheDir(), "templates")
}

// GetLogsDir returns the logs directory path
func GetLogsDir() string {
	return filepath.Join(GetConfigDir(), "logs")
}

// EnsureConfigDir creates the configuration directory if it doesn't exist
func EnsureConfigDir() error {
	configDir := GetConfigDir()
	return os.MkdirAll(configDir, 0700)
}

// EnsureLogsDir creates the logs directory if it doesn't exist
func EnsureLogsDir() error {
	logsDir := GetLogsDir()
	return os.MkdirAll(logsDir, 0700)
}

// EnsureCacheDir creates the cache directory if it doesn't exist
func EnsureCacheDir() error {
	cacheDir := GetCacheDir()
	return os.MkdirAll(cacheDir, 0700)
}

// EnsureTemplatesDir creates the templates directory if it doesn't exist
func EnsureTemplatesDir() error {
	templatesDir := GetTemplatesDir()
	return os.MkdirAll(templatesDir, 0700)
}

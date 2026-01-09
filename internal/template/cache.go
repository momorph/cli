/*
Copyright © 2025 Sun Asterisk Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package template

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/momorph/cli/internal/config"
	"github.com/momorph/cli/internal/logger"
)

// CacheEntry represents a cached template
type CacheEntry struct {
	AITool      string    `json:"ai_tool"`
	Version     string    `json:"version"`
	Checksum    string    `json:"checksum"`
	CachedAt    time.Time `json:"cached_at"`
	FilePath    string    `json:"file_path"`
	OriginalURL string    `json:"original_url"`
	Size        int64     `json:"size"`
}

// CacheIndex represents the cache index file
type CacheIndex struct {
	Version   string                `json:"version"`
	Entries   map[string]CacheEntry `json:"entries"` // Key is AI tool name
	UpdatedAt time.Time             `json:"updated_at"`
}

// Cache manages template caching for offline mode
type Cache struct {
	cacheDir string
	index    *CacheIndex
}

// DefaultCacheTTL is the default time-to-live for cached templates
const DefaultCacheTTL = 24 * time.Hour

// NewCache creates a new template cache
func NewCache() (*Cache, error) {
	cacheDir := filepath.Join(config.GetConfigDir(), "template-cache")

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &Cache{
		cacheDir: cacheDir,
	}

	// Load existing index
	if err := cache.loadIndex(); err != nil {
		logger.Debug("No existing cache index, creating new one: %v", err)
		cache.index = &CacheIndex{
			Version: "1.0",
			Entries: make(map[string]CacheEntry),
		}
	}

	return cache, nil
}

// loadIndex loads the cache index from disk
func (c *Cache) loadIndex() error {
	indexPath := filepath.Join(c.cacheDir, "index.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	var index CacheIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("failed to parse cache index: %w", err)
	}

	c.index = &index
	return nil
}

// saveIndex saves the cache index to disk
func (c *Cache) saveIndex() error {
	c.index.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(c.index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache index: %w", err)
	}

	indexPath := filepath.Join(c.cacheDir, "index.json")
	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache index: %w", err)
	}

	return nil
}

// Get retrieves a cached template if available and not expired
func (c *Cache) Get(aiTool string, ttl time.Duration) (*CacheEntry, error) {
	entry, exists := c.index.Entries[aiTool]
	if !exists {
		return nil, fmt.Errorf("template not in cache: %s", aiTool)
	}

	// Check if cache entry has expired
	if time.Since(entry.CachedAt) > ttl {
		logger.Debug("Cache entry expired for %s (cached at %v)", aiTool, entry.CachedAt)
		return nil, fmt.Errorf("cache entry expired")
	}

	// Verify the cached file still exists
	if _, err := os.Stat(entry.FilePath); os.IsNotExist(err) {
		logger.Debug("Cached file no longer exists: %s", entry.FilePath)
		delete(c.index.Entries, aiTool)
		c.saveIndex()
		return nil, fmt.Errorf("cached file not found")
	}

	return &entry, nil
}

// Put stores a template in the cache
func (c *Cache) Put(aiTool, version, originalURL string, data []byte) error {
	// Calculate checksum
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	// Generate cache file path
	cacheFileName := fmt.Sprintf("%s-%s-%s.zip", aiTool, version, checksum[:8])
	cachePath := filepath.Join(c.cacheDir, cacheFileName)

	// Write the template data to cache
	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Update index
	c.index.Entries[aiTool] = CacheEntry{
		AITool:      aiTool,
		Version:     version,
		Checksum:    checksum,
		CachedAt:    time.Now(),
		FilePath:    cachePath,
		OriginalURL: originalURL,
		Size:        int64(len(data)),
	}

	if err := c.saveIndex(); err != nil {
		// Try to clean up the cache file
		os.Remove(cachePath)
		return err
	}

	logger.Debug("Cached template %s (version %s, size %d bytes)", aiTool, version, len(data))
	return nil
}

// GetCachedFile returns an io.ReadCloser for a cached template
func (c *Cache) GetCachedFile(aiTool string) (io.ReadCloser, error) {
	entry, exists := c.index.Entries[aiTool]
	if !exists {
		return nil, fmt.Errorf("template not in cache: %s", aiTool)
	}

	file, err := os.Open(entry.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cached file: %w", err)
	}

	return file, nil
}

// Remove removes a template from the cache
func (c *Cache) Remove(aiTool string) error {
	entry, exists := c.index.Entries[aiTool]
	if !exists {
		return nil
	}

	// Remove the cache file
	if err := os.Remove(entry.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	// Update index
	delete(c.index.Entries, aiTool)
	return c.saveIndex()
}

// Clear removes all cached templates
func (c *Cache) Clear() error {
	// Remove all cache files
	for aiTool := range c.index.Entries {
		if err := c.Remove(aiTool); err != nil {
			logger.Debug("Failed to remove cache entry %s: %v", aiTool, err)
		}
	}

	// Reset index
	c.index = &CacheIndex{
		Version: "1.0",
		Entries: make(map[string]CacheEntry),
	}

	return c.saveIndex()
}

// List returns all cached templates
func (c *Cache) List() []CacheEntry {
	entries := make([]CacheEntry, 0, len(c.index.Entries))
	for _, entry := range c.index.Entries {
		entries = append(entries, entry)
	}
	return entries
}

// Size returns the total size of cached templates in bytes
func (c *Cache) Size() int64 {
	var total int64
	for _, entry := range c.index.Entries {
		total += entry.Size
	}
	return total
}

// Prune removes expired cache entries
func (c *Cache) Prune(ttl time.Duration) error {
	for aiTool, entry := range c.index.Entries {
		if time.Since(entry.CachedAt) > ttl {
			logger.Debug("Pruning expired cache entry: %s", aiTool)
			if err := c.Remove(aiTool); err != nil {
				logger.Debug("Failed to prune cache entry %s: %v", aiTool, err)
			}
		}
	}
	return nil
}

// VerifyIntegrity checks that all cached files match their recorded checksums
func (c *Cache) VerifyIntegrity() (bool, []string) {
	var corrupted []string

	for aiTool, entry := range c.index.Entries {
		data, err := os.ReadFile(entry.FilePath)
		if err != nil {
			corrupted = append(corrupted, aiTool)
			continue
		}

		hash := sha256.Sum256(data)
		checksum := hex.EncodeToString(hash[:])

		if checksum != entry.Checksum {
			corrupted = append(corrupted, aiTool)
		}
	}

	return len(corrupted) == 0, corrupted
}

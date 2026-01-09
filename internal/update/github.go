package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/momorph/cli/internal/utils"
)

const (
	// GitHub repository for releases
	repoOwner = "sun-asterisk-internal"
	repoName  = "momorph-cli"

	// GitHub API endpoints
	releasesAPI = "https://api.github.com/repos/%s/%s/releases/latest"
)

// Release represents a GitHub release
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// GetLatestRelease fetches the latest release from GitHub
func GetLatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf(releasesAPI, repoOwner, repoName)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Send request
	client := utils.NewHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error (status %d)", resp.StatusCode)
	}

	// Parse response
	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// GetVersion extracts the version from a tag name (e.g., "v1.2.3" -> "1.2.3")
func (r *Release) GetVersion() string {
	version := r.TagName
	if strings.HasPrefix(version, "v") {
		version = version[1:]
	}
	return version
}

// GetAssetForPlatform returns the download asset for the current platform
func (r *Release) GetAssetForPlatform() (*Asset, error) {
	// Determine platform-specific asset name
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map architecture names
	archMap := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
		"386":   "386",
	}

	mappedArch, ok := archMap[arch]
	if !ok {
		return nil, fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Build expected asset name patterns
	patterns := []string{
		fmt.Sprintf("momorph-cli_%s_%s", os, mappedArch),
		fmt.Sprintf("mm_%s_%s", os, mappedArch),
	}

	// Search for matching asset
	for _, asset := range r.Assets {
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(pattern)) {
				return &asset, nil
			}
		}
	}

	return nil, fmt.Errorf("no release asset found for %s/%s", os, arch)
}

// CompareVersions compares two semver versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Handle "dev" version
	if v1 == "dev" {
		return -1
	}
	if v2 == "dev" {
		return 1
	}

	// Split into parts
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}

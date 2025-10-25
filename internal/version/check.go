package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
)

const (
	// GitHub API endpoint for latest release
	githubReleasesAPI = "https://api.github.com/repos/CerebriumAI/cerebrium/releases/latest"

	// Cache file for version check (avoid checking too frequently)
	versionCacheFile = ".cerebrium/version_cache.json"

	// Cache duration - only check once per day
	cacheDuration = 24 * time.Hour
)

// VersionCache stores the cached version check result
type VersionCache struct {
	LatestVersion string    `json:"latestVersion"`
	CheckedAt     time.Time `json:"checkedAt"`
}

// GitHubRelease represents a GitHub release response
type GitHubRelease struct {
	TagName string `json:"tag_name"` // e.g., "v2.0.1"
	HTMLURL string `json:"html_url"`
}

// CheckForUpdate checks if a newer version is available
// Returns (latestVersion, updateAvailable, error)
func CheckForUpdate(ctx context.Context) (latestVersion string, updateAvailable bool, err error) {
	// Skip check in dev builds
	if Version == "dev" {
		return "", false, nil
	}

	// Try to get from cache first
	if cached, ok := getCachedVersion(); ok {
		return compareVersions(cached)
	}

	// Fetch latest version from GitHub
	latest, err := fetchLatestVersion(ctx)
	if err != nil {
		// Don't fail the command if version check fails
		// Just silently skip the update notification
		//nolint:nilerr
		return "", false, nil
	}

	// Cache the result
	cacheVersion(latest)

	return compareVersions(latest)
}

// compareVersions compares current version with latest
func compareVersions(latestVersion string) (string, bool, error) {
	// Parse versions (strip 'v' prefix if present)
	currentVer := strings.TrimPrefix(Version, "v")
	latestVer := strings.TrimPrefix(latestVersion, "v")

	current, err := version.NewVersion(currentVer)
	if err != nil {
		return latestVersion, false, fmt.Errorf("invalid current version: %w", err)
	}

	latest, err := version.NewVersion(latestVer)
	if err != nil {
		return latestVersion, false, fmt.Errorf("invalid latest version: %w", err)
	}

	// Check if update is available
	updateAvailable := latest.GreaterThan(current)

	return latestVersion, updateAvailable, nil
}

// fetchLatestVersion fetches the latest version from GitHub API
func fetchLatestVersion(ctx context.Context) (string, error) {
	client := &http.Client{
		Timeout: 3 * time.Second, // Don't wait too long
	}

	req, err := http.NewRequestWithContext(ctx, "GET", githubReleasesAPI, nil)
	if err != nil {
		return "", err
	}

	// Set User-Agent (GitHub API requires this)
	req.Header.Set("User-Agent", "cerebrium-cli")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	//nolint:errcheck // Deferred close, error not actionable
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// getCachedVersion retrieves the cached version check result
func getCachedVersion() (string, bool) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}

	cachePath := filepath.Join(homeDir, versionCacheFile)

	// Check if cache file exists
	data, err := os.ReadFile(cachePath) //nolint:gosec // Cache file in user's home directory
	if err != nil {
		return "", false
	}

	var cache VersionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return "", false
	}

	// Check if cache is still valid
	if time.Since(cache.CheckedAt) > cacheDuration {
		return "", false
	}

	return cache.LatestVersion, true
}

// cacheVersion stores the version check result
func cacheVersion(latestVersion string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	cachePath := filepath.Join(homeDir, versionCacheFile)

	// Ensure directory exists
	cacheDir := filepath.Dir(cachePath)
	//nolint:errcheck,gosec // Best effort directory creation, error not actionable
	os.MkdirAll(cacheDir, 0755)

	cache := VersionCache{
		LatestVersion: latestVersion,
		CheckedAt:     time.Now(),
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return
	}

	//nolint:errcheck,gosec // Best effort cache write, error not actionable
	os.WriteFile(cachePath, data, 0644)
}

// PrintUpdateNotification prints an update notification if available
// Respects user's config preference for skipping version checks
func PrintUpdateNotification(ctx context.Context, skipVersionCheck bool) {
	// Skip if user has disabled version checks
	if skipVersionCheck {
		return
	}

	latestVersion, updateAvailable, err := CheckForUpdate(ctx)
	if err != nil || !updateAvailable {
		return
	}

	// Print update notification to stderr
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "⚠️  A new version of Cerebrium CLI is available: %s (you have %s)\n", latestVersion, Version)
	fmt.Fprintf(os.Stderr, "Update with:\n")
	fmt.Fprintf(os.Stderr, "  • Homebrew: brew upgrade cerebrium\n")
	fmt.Fprintf(os.Stderr, "  • Pip: pip install --upgrade cerebrium\n")
	fmt.Fprintf(os.Stderr, "  • Download: https://github.com/CerebriumAI/cerebrium/releases/latest\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "To disable these notifications: cerebrium config --version-check=false\n")
	fmt.Fprintf(os.Stderr, "\n")
}

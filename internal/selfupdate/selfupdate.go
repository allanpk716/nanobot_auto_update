// Package selfupdate provides self-update functionality for nanobot-auto-updater.
// It checks GitHub Releases for new versions, compares versions using semver,
// and will eventually download and apply updates.
package selfupdate

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/minio/selfupdate"
	"golang.org/x/mod/semver"
)

// Package constants (per D-04, hardcoded values)
const (
	cacheTTL              = 1 * time.Hour
	httpTimeout           = 30 * time.Second
	userAgent             = "nanobot-auto-updater/selfupdate"
	defaultGitHubAPIBase  = "https://api.github.com"
	zipAssetSuffix        = "_windows_amd64.zip"
	checksumsAssetSuffix  = "_checksums.txt"
	exeName               = "nanobot-auto-updater.exe" // binary name inside ZIP (per RESEARCH Pitfall 3)
)

// SelfUpdateConfig holds configuration for self-update functionality.
// Per D-04, this is the minimal config — other parameters are hardcoded as package constants.
type SelfUpdateConfig struct {
	GithubOwner string `yaml:"github_owner" mapstructure:"github_owner"`
	GithubRepo  string `yaml:"github_repo" mapstructure:"github_repo"`
}

// ReleaseInfo contains information about a GitHub Release.
type ReleaseInfo struct {
	Version      string       // e.g. "v1.0.0"
	DownloadURL  string       // browser_download_url for the ZIP asset
	ChecksumURL  string       // browser_download_url for checksums.txt asset
	ReleaseNotes string       // body field from GitHub
	HTMLURL      string       // link to the release page
	PublishedAt  time.Time    // published_at from GitHub
	Assets       []AssetInfo  // all assets in the release
}

// AssetInfo contains information about a release asset.
type AssetInfo struct {
	Name               string
	BrowserDownloadURL string
	Size               int64
}

// githubRelease represents the GitHub API response for a release (unexported).
type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	Assets      []githubAsset `json:"assets"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt time.Time     `json:"published_at"`
}

// githubAsset represents a single asset in a GitHub Release (unexported).
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// Updater handles self-update operations including version checking and binary replacement.
// Per D-03, it encapsulates configuration, HTTP client, cache, and logger.
type Updater struct {
	cfg           SelfUpdateConfig
	httpClient    *http.Client
	cachedRelease *ReleaseInfo
	cacheTime     time.Time
	logger        *slog.Logger
	baseURL       string // defaults to "https://api.github.com", overrideable for tests
}

// NewUpdater creates a new Updater with the given configuration.
func NewUpdater(cfg SelfUpdateConfig, logger *slog.Logger) *Updater {
	return &Updater{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: httpTimeout},
		logger:     logger.With("component", "selfupdate"),
		baseURL:    defaultGitHubAPIBase,
	}
}

// CheckLatest checks GitHub for the latest release. Results are cached for cacheTTL (1 hour).
// Returns ReleaseInfo with version, download URL, and checksum URL (UPDATE-01, UPDATE-06).
func (u *Updater) CheckLatest() (*ReleaseInfo, error) {
	// Check cache
	if u.cachedRelease != nil && time.Since(u.cacheTime) < cacheTTL {
		u.logger.Debug("returning cached release info",
			"version", u.cachedRelease.Version,
			"cache_age", time.Since(u.cacheTime).Round(time.Second),
		)
		return u.cachedRelease, nil
	}

	// Build GitHub API URL
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", u.baseURL, u.cfg.GithubOwner, u.cfg.GithubRepo)

	// Create request with User-Agent header
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	u.logger.Debug("checking GitHub for latest release", "url", url)

	// Send request
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	// Decode JSON
	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("decode GitHub release JSON: %w", err)
	}

	// Convert to ReleaseInfo
	info := &ReleaseInfo{
		Version:      release.TagName,
		ReleaseNotes: release.Body,
		HTMLURL:      release.HTMLURL,
		PublishedAt:  release.PublishedAt,
	}

	// Find ZIP and checksums assets
	var foundZip, foundChecksum bool
	for _, asset := range release.Assets {
		assetInfo := AssetInfo{
			Name:               asset.Name,
			BrowserDownloadURL: asset.BrowserDownloadURL,
			Size:               asset.Size,
		}
		info.Assets = append(info.Assets, assetInfo)

		if strings.HasSuffix(asset.Name, zipAssetSuffix) {
			info.DownloadURL = asset.BrowserDownloadURL
			foundZip = true
		}
		if strings.Contains(asset.Name, checksumsAssetSuffix) {
			info.ChecksumURL = asset.BrowserDownloadURL
			foundChecksum = true
		}
	}

	// Validate required assets
	if !foundZip {
		return nil, fmt.Errorf("no windows amd64 zip asset found in release %s", release.TagName)
	}
	if !foundChecksum {
		return nil, fmt.Errorf("no checksums asset found in release %s", release.TagName)
	}

	// Cache the result
	u.cachedRelease = info
	u.cacheTime = time.Now()

	u.logger.Info("found latest release",
		"version", info.Version,
		"download_url", info.DownloadURL,
	)

	return info, nil
}

// NeedUpdate checks if the current version needs to be updated by comparing
// with the latest GitHub Release using semver (UPDATE-02).
// Dev version ("dev") always returns true as it needs updating.
func (u *Updater) NeedUpdate(currentVersion string) (bool, *ReleaseInfo, error) {
	release, err := u.CheckLatest()
	if err != nil {
		return false, nil, fmt.Errorf("check latest: %w", err)
	}

	// Dev version always needs update (UPDATE-02)
	if currentVersion == "dev" {
		u.logger.Debug("dev version detected, update needed")
		return true, release, nil
	}

	// Ensure "v" prefix for semver comparison (per RESEARCH Pitfall 1)
	current := "v" + strings.TrimPrefix(currentVersion, "v")
	latest := "v" + strings.TrimPrefix(release.Version, "v")

	result := semver.Compare(current, latest)
	needsUpdate := result < 0

	u.logger.Debug("version comparison",
		"current", current,
		"latest", latest,
		"needs_update", needsUpdate,
	)

	return needsUpdate, release, nil
}

// verifyChecksum computes SHA256 of data and compares with expected hash (UPDATE-03).
func verifyChecksum(data []byte, expectedSHA256 []byte) bool {
	hash := sha256.Sum256(data)
	return bytes.Equal(hash[:], expectedSHA256)
}

// parseChecksum parses GoReleaser checksums.txt format to find the SHA256 hash for a given filename.
// Format: "<sha256_hex>  <filename>\n" (two spaces between hash and filename).
func parseChecksum(checksumsTxt []byte, filename string) ([]byte, error) {
	lines := strings.Split(string(checksumsTxt), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) == 2 && parts[1] == filename {
			return hex.DecodeString(parts[0])
		}
	}
	return nil, fmt.Errorf("checksum for %q not found", filename)
}

// extractExeFromZip extracts the target exe from a ZIP archive in memory (per D-01).
// No temp files are created; the exe content is returned as an io.Reader.
func extractExeFromZip(zipData []byte, targetExeName string) (io.Reader, error) {
	readerAt := bytes.NewReader(zipData)
	zipReader, err := zip.NewReader(readerAt, int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("zip open: %w", err)
	}
	for _, f := range zipReader.File {
		if f.Name == targetExeName {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("zip entry open: %w", err)
			}
			defer rc.Close()
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				return nil, fmt.Errorf("zip entry read: %w", err)
			}
			return &buf, nil
		}
	}
	return nil, fmt.Errorf("exe %q not found in zip", targetExeName)
}

// download fetches a URL and returns the response body bytes.
func (u *Updater) download(url string) ([]byte, error) {
	resp, err := u.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read download %s: %w", url, err)
	}
	return data, nil
}

// Update performs the full self-update pipeline: check version, download ZIP,
// verify SHA256 checksum, extract exe in memory, and apply via minio/selfupdate
// (UPDATE-04, UPDATE-05).
// If already up to date, returns nil without downloading.
// On Apply failure, checks RollbackError to determine if rollback succeeded.
func (u *Updater) Update(currentVersion string) error {
	// 1. Check if update needed
	needsUpdate, release, err := u.NeedUpdate(currentVersion)
	if err != nil {
		return fmt.Errorf("check update: %w", err)
	}
	if !needsUpdate {
		u.logger.Info("Already up to date")
		return nil
	}

	u.logger.Info("Update available",
		"current", currentVersion,
		"latest", release.Version)

	// 2. Find the ZIP asset name from release assets
	var zipAssetName string
	for _, a := range release.Assets {
		if strings.HasSuffix(a.Name, zipAssetSuffix) {
			zipAssetName = a.Name
			break
		}
	}
	if zipAssetName == "" {
		return fmt.Errorf("no windows amd64 zip asset found in release %s", release.Version)
	}

	// 3. Download checksums.txt
	u.logger.Info("Downloading checksums", "url", release.ChecksumURL)
	checksumsData, err := u.download(release.ChecksumURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}

	// 4. Download ZIP
	u.logger.Info("Downloading update", "url", release.DownloadURL)
	zipData, err := u.download(release.DownloadURL)
	if err != nil {
		return fmt.Errorf("download zip: %w", err)
	}

	// 5. Verify ZIP checksum (D-02, UPDATE-03)
	expectedHash, err := parseChecksum(checksumsData, zipAssetName)
	if err != nil {
		return fmt.Errorf("parse checksum: %w", err)
	}
	if !verifyChecksum(zipData, expectedHash) {
		return fmt.Errorf("checksum verification failed for %s", zipAssetName)
	}
	u.logger.Info("Checksum verified", "asset", zipAssetName)

	// 6. Extract exe from ZIP in memory (D-01)
	exeReader, err := extractExeFromZip(zipData, exeName)
	if err != nil {
		return fmt.Errorf("extract exe: %w", err)
	}
	u.logger.Info("Extracted exe from zip", "exe", exeName)

	// 7. Apply update using minio/selfupdate (UPDATE-04, UPDATE-05)
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get exe path: %w", err)
	}
	opts := selfupdate.Options{
		OldSavePath: exePath + ".old",
	}

	u.logger.Info("Applying update", "exe", exePath, "backup", opts.OldSavePath)
	err = selfupdate.Apply(exeReader, opts)
	if err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback also failed: %w (rollback: %v)", err, rerr)
		}
		return fmt.Errorf("update failed (rolled back): %w", err)
	}

	u.logger.Info("Update applied successfully",
		"old_version", currentVersion,
		"new_version", release.Version)
	return nil
}

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
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validReleaseJSON returns a GitHub Release API response with the given tag and assets.
func validReleaseJSON(tag string) string {
	assets := []map[string]interface{}{
		{
			"name":               fmt.Sprintf("nanobot-auto-updater_%s_windows_amd64.zip", strings.TrimPrefix(tag, "v")),
			"browser_download_url": fmt.Sprintf("https://github.com/test/repo/releases/download/%s/nanobot-auto-updater_%s_windows_amd64.zip", tag, strings.TrimPrefix(tag, "v")),
			"size":               1024,
			"content_type":       "application/zip",
		},
		{
			"name":               fmt.Sprintf("nanobot-auto-updater_%s_checksums.txt", strings.TrimPrefix(tag, "v")),
			"browser_download_url": fmt.Sprintf("https://github.com/test/repo/releases/download/%s/nanobot-auto-updater_%s_checksums.txt", tag, strings.TrimPrefix(tag, "v")),
			"size":               256,
			"content_type":       "text/plain",
		},
	}
	release := map[string]interface{}{
		"tag_name":     tag,
		"name":         tag,
		"body":         "Release notes for " + tag,
		"html_url":     fmt.Sprintf("https://github.com/test/repo/releases/tag/%s", tag),
		"published_at": "2026-03-29T00:00:00Z",
		"assets":       assets,
	}
	data, _ := json.Marshal(release)
	return string(data)
}

// newTestUpdater creates an Updater pointed at the given test server URL.
func newTestUpdater(serverURL string) *Updater {
	return &Updater{
		cfg: SelfUpdateConfig{
			GithubOwner: "test",
			GithubRepo:  "repo",
		},
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.Default().With("component", "selfupdate"),
		baseURL:    serverURL,
	}
}

func TestCheckLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/test/repo/releases/latest", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, userAgent, r.Header.Get("User-Agent"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v1.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	info, err := u.CheckLatest()

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "v1.0.0", info.Version)
	assert.Contains(t, info.DownloadURL, "windows_amd64.zip")
	assert.Contains(t, info.ChecksumURL, "checksums.txt")
	assert.Equal(t, "Release notes for v1.0.0", info.ReleaseNotes)
	assert.Contains(t, info.HTMLURL, "github.com")
	assert.Len(t, info.Assets, 2)
}

func TestCheckLatest_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	_, err := u.CheckLatest()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub API")
}

func TestCheckLatest_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	_, err := u.CheckLatest()

	require.Error(t, err)
}

func TestNeedUpdate_OlderVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v1.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	needsUpdate, release, err := u.NeedUpdate("0.9.0")

	require.NoError(t, err)
	assert.True(t, needsUpdate)
	require.NotNil(t, release)
	assert.Equal(t, "v1.0.0", release.Version)
}

func TestNeedUpdate_SameVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v1.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	needsUpdate, release, err := u.NeedUpdate("1.0.0")

	require.NoError(t, err)
	assert.False(t, needsUpdate)
	require.NotNil(t, release)
}

func TestNeedUpdate_NewerVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v1.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	needsUpdate, release, err := u.NeedUpdate("1.1.0")

	require.NoError(t, err)
	assert.False(t, needsUpdate)
	require.NotNil(t, release)
}

func TestNeedUpdate_Dev(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v1.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	needsUpdate, release, err := u.NeedUpdate("dev")

	require.NoError(t, err)
	assert.True(t, needsUpdate)
	require.NotNil(t, release)
	assert.Equal(t, "v1.0.0", release.Version)
}

func TestCache_Hit(t *testing.T) {
	hitCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v1.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)

	// First call hits the server
	info1, err := u.CheckLatest()
	require.NoError(t, err)
	assert.Equal(t, 1, hitCount)

	// Second call within TTL should use cache (no additional server hit)
	info2, err := u.CheckLatest()
	require.NoError(t, err)
	assert.Equal(t, 1, hitCount) // Still 1, cache hit
	assert.Equal(t, info1.Version, info2.Version)
}

func TestCache_Expiry(t *testing.T) {
	hitCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v1.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)

	// First call
	_, err := u.CheckLatest()
	require.NoError(t, err)
	assert.Equal(t, 1, hitCount)

	// Expire cache by manipulating cacheTime
	u.cacheTime = time.Now().Add(-2 * time.Hour)

	// Second call should hit server again
	_, err = u.CheckLatest()
	require.NoError(t, err)
	assert.Equal(t, 2, hitCount)
}

func TestCheckLatest_NoZipAsset(t *testing.T) {
	// Return a release without a windows_amd64.zip asset
	release := map[string]interface{}{
		"tag_name":     "v1.0.0",
		"name":         "v1.0.0",
		"body":         "notes",
		"html_url":     "https://github.com/test/repo/releases/tag/v1.0.0",
		"published_at": "2026-03-29T00:00:00Z",
		"assets": []map[string]interface{}{
			{
				"name":               "some_other_file.txt",
				"browser_download_url": "https://example.com/file.txt",
				"size":               100,
				"content_type":       "text/plain",
			},
		},
	}
	data, _ := json.Marshal(release)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	_, err := u.CheckLatest()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no windows amd64 zip asset")
}

func TestCheckLatest_NoChecksumAsset(t *testing.T) {
	// Return a release with ZIP but no checksums.txt
	release := map[string]interface{}{
		"tag_name":     "v1.0.0",
		"name":         "v1.0.0",
		"body":         "notes",
		"html_url":     "https://github.com/test/repo/releases/tag/v1.0.0",
		"published_at": "2026-03-29T00:00:00Z",
		"assets": []map[string]interface{}{
			{
				"name":               "nanobot-auto-updater_1.0.0_windows_amd64.zip",
				"browser_download_url": "https://github.com/test/repo/releases/download/v1.0.0/nanobot-auto-updater_1.0.0_windows_amd64.zip",
				"size":               1024,
				"content_type":       "application/zip",
			},
		},
	}
	data, _ := json.Marshal(release)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	_, err := u.CheckLatest()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no checksums")
}

func TestNeedUpdate_VersionWithVPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v2.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	// Current version already has "v" prefix
	needsUpdate, _, err := u.NeedUpdate("v1.0.0")

	require.NoError(t, err)
	assert.True(t, needsUpdate)
}

func TestCheckLatest_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "this is not json")
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	_, err := u.CheckLatest()

	require.Error(t, err)
}

func TestNeedUpdate_NilCacheAndAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "error")
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	needsUpdate, release, err := u.NeedUpdate("1.0.0")

	require.Error(t, err)
	assert.False(t, needsUpdate)
	assert.Nil(t, release)
}

// --- Plan 02 tests: Update pipeline (download, checksum, extract, apply) ---

// createTestZip creates a ZIP archive in memory containing a single file with the given content.
func createTestZip(t *testing.T, filename, content string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(filename)
	require.NoError(t, err)
	_, err = f.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestVerifyChecksum_Valid(t *testing.T) {
	data := []byte("hello")
	hash := sha256.Sum256(data)
	assert.True(t, verifyChecksum(data, hash[:]))
}

func TestVerifyChecksum_Invalid(t *testing.T) {
	assert.False(t, verifyChecksum([]byte("hello"), []byte("0000")))
}

func TestParseChecksum_Valid(t *testing.T) {
	// GoReleaser format: <sha256_hex>  <filename>\n
	data := []byte("a1b2c3d4  test.zip\notherhash  other.zip\n")
	result, err := parseChecksum(data, "test.zip")
	require.NoError(t, err)
	expected, _ := hex.DecodeString("a1b2c3d4")
	assert.Equal(t, expected, result)
}

func TestParseChecksum_NotFound(t *testing.T) {
	data := []byte("a1b2c3d4  test.zip\n")
	_, err := parseChecksum(data, "missing.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractExeFromZip(t *testing.T) {
	zipData := createTestZip(t, exeName, "fake exe content")
	reader, err := extractExeFromZip(zipData, exeName)
	require.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "fake exe content", string(content))
}

func TestExtractExeFromZip_NotFound(t *testing.T) {
	zipData := createTestZip(t, "other.exe", "other content")
	_, err := extractExeFromZip(zipData, exeName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in zip")
}

// TestUpdate_FullFlow tests the individual pipeline steps (download, checksum, extract)
// Update() full integration tested via Phase 36 PoC pattern (selfupdate.Apply replaces running exe).
func TestUpdate_FullFlow(t *testing.T) {
	exeContent := "fake exe binary content"
	zipData := createTestZip(t, exeName, exeContent)
	zipHash := sha256.Sum256(zipData)
	checksumsContent := fmt.Sprintf("%s  nanobot-auto-updater_2.0.0_windows_amd64.zip\n",
		hex.EncodeToString(zipHash[:]))

	var downloadCount int32
	var serverURL string // captured by closure, set after httptest.NewServer

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/test/repo/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			release := map[string]interface{}{
				"tag_name":     "v2.0.0",
				"name":         "v2.0.0",
				"body":         "Release notes for v2.0.0",
				"html_url":     "https://github.com/test/repo/releases/tag/v2.0.0",
				"published_at": "2026-03-29T00:00:00Z",
				"assets": []map[string]interface{}{
					{
						"name":               "nanobot-auto-updater_2.0.0_windows_amd64.zip",
						"browser_download_url": serverURL + "/download/zip",
						"size":               len(zipData),
						"content_type":       "application/zip",
					},
					{
						"name":               "nanobot-auto-updater_2.0.0_checksums.txt",
						"browser_download_url": serverURL + "/download/checksums",
						"size":               len(checksumsContent),
						"content_type":       "text/plain",
					},
				},
			}
			data, _ := json.Marshal(release)
			w.Write(data)

		case r.URL.Path == "/download/checksums":
			atomic.AddInt32(&downloadCount, 1)
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, checksumsContent)

		case r.URL.Path == "/download/zip":
			atomic.AddInt32(&downloadCount, 1)
			w.WriteHeader(http.StatusOK)
			w.Write(zipData)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	u := newTestUpdater(server.URL)

	// Step 1: Check if update needed
	needsUpdate, release, err := u.NeedUpdate("1.0.0")
	require.NoError(t, err)
	assert.True(t, needsUpdate)
	require.NotNil(t, release)

	// Step 2: Find ZIP asset name
	var zipAssetName string
	for _, a := range release.Assets {
		if strings.HasSuffix(a.Name, zipAssetSuffix) {
			zipAssetName = a.Name
			break
		}
	}
	assert.NotEmpty(t, zipAssetName)

	// Step 3: Download checksums
	checksumsData, err := u.download(release.ChecksumURL)
	require.NoError(t, err)
	assert.Equal(t, checksumsContent, string(checksumsData))

	// Step 4: Download ZIP
	downloadedZip, err := u.download(release.DownloadURL)
	require.NoError(t, err)
	assert.Equal(t, zipData, downloadedZip)

	// Step 5: Verify checksum
	expectedHash, err := parseChecksum(checksumsData, zipAssetName)
	require.NoError(t, err)
	assert.True(t, verifyChecksum(downloadedZip, expectedHash))

	// Step 6: Extract exe from ZIP
	exeReader, err := extractExeFromZip(downloadedZip, exeName)
	require.NoError(t, err)
	extractedContent, err := io.ReadAll(exeReader)
	require.NoError(t, err)
	assert.Equal(t, exeContent, string(extractedContent))

	// Verify downloads happened
	assert.Equal(t, int32(2), atomic.LoadInt32(&downloadCount))
}

func TestUpdate_AlreadyUpToDate(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validReleaseJSON("v1.0.0"))
	}))
	defer server.Close()

	u := newTestUpdater(server.URL)
	err := u.Update("1.0.0")

	require.NoError(t, err)
	// Only 1 request (the API call to check version), no download requests
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount))
}

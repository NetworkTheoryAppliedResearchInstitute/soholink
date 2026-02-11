package update

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"golang.org/x/mod/semver"
)

// ReleaseInfo represents information about a software release
type ReleaseInfo struct {
	Version     string    `json:"version"`
	ReleaseDate time.Time `json:"release_date"`
	DownloadURL string    `json:"download_url"`
	Signature   string    `json:"signature"` // Hex-encoded Ed25519 signature
	Changelog   string    `json:"changelog"`
	Critical    bool      `json:"critical"` // Whether this is a critical security update
	MinVersion  string    `json:"min_version,omitempty"` // Minimum version that can upgrade
}

// UpdateChecker checks for software updates
type UpdateChecker struct {
	currentVersion string
	updateEndpoint string
	publicKey      ed25519.PublicKey
	httpClient     *http.Client
}

// NewUpdateChecker creates a new update checker
func NewUpdateChecker(currentVersion, updateEndpoint string, publicKey ed25519.PublicKey) *UpdateChecker {
	return &UpdateChecker{
		currentVersion: currentVersion,
		updateEndpoint: updateEndpoint,
		publicKey:      publicKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckForUpdates queries the update endpoint for new releases
func (uc *UpdateChecker) CheckForUpdates(ctx context.Context) (*ReleaseInfo, error) {
	// Build request URL with current version and OS/arch
	url := fmt.Sprintf("%s?version=%s&os=%s&arch=%s",
		uc.updateEndpoint,
		uc.currentVersion,
		runtime.GOOS,
		runtime.GOARCH,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("SoHoLINK/%s (%s/%s)", uc.currentVersion, runtime.GOOS, runtime.GOARCH))

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		// No updates available
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update server returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release info: %w", err)
	}

	// Verify the release is newer
	if !uc.isNewer(release.Version) {
		return nil, nil
	}

	// Verify minimum version requirement
	if release.MinVersion != "" && !uc.meetsMinVersion(release.MinVersion) {
		return nil, fmt.Errorf("current version %s does not meet minimum upgrade requirement %s", uc.currentVersion, release.MinVersion)
	}

	return &release, nil
}

// isNewer returns true if the provided version is newer than current
func (uc *UpdateChecker) isNewer(version string) bool {
	// Ensure versions are prefixed with 'v' for semver comparison
	current := uc.currentVersion
	if current[0] != 'v' {
		current = "v" + current
	}
	if version[0] != 'v' {
		version = "v" + version
	}

	return semver.Compare(version, current) > 0
}

// meetsMinVersion returns true if current version meets the minimum requirement
func (uc *UpdateChecker) meetsMinVersion(minVersion string) bool {
	current := uc.currentVersion
	if current[0] != 'v' {
		current = "v" + current
	}
	if minVersion[0] != 'v' {
		minVersion = "v" + minVersion
	}

	return semver.Compare(current, minVersion) >= 0
}

// DownloadUpdate downloads the update binary
func (uc *UpdateChecker) DownloadUpdate(ctx context.Context, release *ReleaseInfo) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, release.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Read the entire binary into memory
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read update data: %w", err)
	}

	return data, nil
}

// VerifySignature verifies the Ed25519 signature of the binary
func (uc *UpdateChecker) VerifySignature(data []byte, signatureHex string) error {
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length: expected %d, got %d", ed25519.SignatureSize, len(signature))
	}

	if !ed25519.Verify(uc.publicKey, data, signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// CheckAndDownload checks for updates and downloads if available
func (uc *UpdateChecker) CheckAndDownload(ctx context.Context) (*ReleaseInfo, []byte, error) {
	// Check for updates
	release, err := uc.CheckForUpdates(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("check failed: %w", err)
	}

	if release == nil {
		// No updates available
		return nil, nil, nil
	}

	// Download the update
	data, err := uc.DownloadUpdate(ctx, release)
	if err != nil {
		return nil, nil, fmt.Errorf("download failed: %w", err)
	}

	// Verify signature
	if err := uc.VerifySignature(data, release.Signature); err != nil {
		return nil, nil, fmt.Errorf("signature verification failed: %w", err)
	}

	return release, data, nil
}

// GetReleaseNotes fetches the changelog for a specific version
func (uc *UpdateChecker) GetReleaseNotes(ctx context.Context, version string) (string, error) {
	url := fmt.Sprintf("%s/changelog?version=%s", uc.updateEndpoint, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch changelog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("changelog request failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read changelog: %w", err)
	}

	return string(data), nil
}

// CompareVersions compares two version strings
// Returns:
//   -1 if v1 < v2
//    0 if v1 == v2
//    1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	if v1[0] != 'v' {
		v1 = "v" + v1
	}
	if v2[0] != 'v' {
		v2 = "v" + v2
	}
	return semver.Compare(v1, v2)
}

package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUpdateChecker_CheckForUpdates(t *testing.T) {
	// Generate test key pair
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("version") == "1.0.0" {
			release := ReleaseInfo{
				Version:     "1.1.0",
				ReleaseDate: time.Now(),
				DownloadURL: "https://example.com/fedaaa-1.1.0",
				Signature:   "deadbeef",
				Changelog:   "Bug fixes and improvements",
				Critical:    false,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(release)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	checker := NewUpdateChecker("1.0.0", server.URL, pubKey)

	ctx := context.Background()
	release, err := checker.CheckForUpdates(ctx)
	if err != nil {
		t.Fatalf("CheckForUpdates failed: %v", err)
	}

	if release == nil {
		t.Fatal("Expected release info, got nil")
	}

	if release.Version != "1.1.0" {
		t.Errorf("Expected version 1.1.0, got %s", release.Version)
	}
}

func TestUpdateChecker_NoUpdatesAvailable(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	checker := NewUpdateChecker("1.0.0", server.URL, pubKey)

	ctx := context.Background()
	release, err := checker.CheckForUpdates(ctx)
	if err != nil {
		t.Fatalf("CheckForUpdates failed: %v", err)
	}

	if release != nil {
		t.Error("Expected no updates, got release info")
	}
}

func TestUpdateChecker_VerifySignature(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	data := []byte("test binary data")
	signature := ed25519.Sign(privKey, data)
	signatureHex := hex.EncodeToString(signature)

	checker := NewUpdateChecker("1.0.0", "https://example.com", pubKey)

	// Test valid signature
	err = checker.VerifySignature(data, signatureHex)
	if err != nil {
		t.Errorf("Valid signature verification failed: %v", err)
	}

	// Test invalid signature
	err = checker.VerifySignature([]byte("tampered data"), signatureHex)
	if err == nil {
		t.Error("Expected signature verification to fail for tampered data")
	}

	// Test invalid signature format
	err = checker.VerifySignature(data, "invalid-hex")
	if err == nil {
		t.Error("Expected signature verification to fail for invalid hex")
	}
}

func TestUpdateChecker_IsNewer(t *testing.T) {
	checker := NewUpdateChecker("1.0.0", "https://example.com", nil)

	testCases := []struct {
		name     string
		current  string
		compare  string
		expected bool
	}{
		{"Patch update", "1.0.0", "1.0.1", true},
		{"Minor update", "1.0.0", "1.1.0", true},
		{"Major update", "1.0.0", "2.0.0", true},
		{"Same version", "1.0.0", "1.0.0", false},
		{"Older version", "1.0.0", "0.9.0", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			checker.currentVersion = tc.current
			result := checker.isNewer(tc.compare)
			if result != tc.expected {
				t.Errorf("Expected %v for %s vs %s, got %v", tc.expected, tc.current, tc.compare, result)
			}
		})
	}
}

func TestUpdateChecker_MeetsMinVersion(t *testing.T) {
	testCases := []struct {
		name       string
		current    string
		minVersion string
		expected   bool
	}{
		{"Exact match", "1.0.0", "1.0.0", true},
		{"Above minimum", "1.1.0", "1.0.0", true},
		{"Below minimum", "0.9.0", "1.0.0", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			checker := NewUpdateChecker(tc.current, "https://example.com", nil)
			result := checker.meetsMinVersion(tc.minVersion)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestUpdateChecker_DownloadUpdate(t *testing.T) {
	binaryData := []byte("fake binary data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(binaryData)
	}))
	defer server.Close()

	checker := NewUpdateChecker("1.0.0", "https://example.com", nil)

	release := &ReleaseInfo{
		Version:     "1.1.0",
		DownloadURL: server.URL,
	}

	ctx := context.Background()
	data, err := checker.DownloadUpdate(ctx, release)
	if err != nil {
		t.Fatalf("DownloadUpdate failed: %v", err)
	}

	if string(data) != string(binaryData) {
		t.Error("Downloaded data doesn't match expected")
	}
}

func TestUpdateChecker_CheckAndDownload(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	binaryData := []byte("test binary")
	signature := ed25519.Sign(privKey, binaryData)

	// Create two servers: one for update check, one for download
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryData)
	}))
	defer downloadServer.Close()

	checkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := ReleaseInfo{
			Version:     "1.1.0",
			ReleaseDate: time.Now(),
			DownloadURL: downloadServer.URL,
			Signature:   hex.EncodeToString(signature),
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer checkServer.Close()

	checker := NewUpdateChecker("1.0.0", checkServer.URL, pubKey)

	ctx := context.Background()
	release, data, err := checker.CheckAndDownload(ctx)
	if err != nil {
		t.Fatalf("CheckAndDownload failed: %v", err)
	}

	if release == nil {
		t.Fatal("Expected release info")
	}

	if release.Version != "1.1.0" {
		t.Errorf("Expected version 1.1.0, got %s", release.Version)
	}

	if string(data) != string(binaryData) {
		t.Error("Downloaded data doesn't match")
	}
}

func TestCompareVersions(t *testing.T) {
	testCases := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.9.9", 1},
		{"v1.0.0", "v1.0.0", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.v1+"_vs_"+tc.v2, func(t *testing.T) {
			result := CompareVersions(tc.v1, tc.v2)
			if result != tc.expected {
				t.Errorf("Expected %d, got %d", tc.expected, result)
			}
		})
	}
}

func BenchmarkUpdateChecker_CheckForUpdates(b *testing.B) {
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := ReleaseInfo{
			Version:     "1.1.0",
			DownloadURL: "https://example.com/binary",
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	checker := NewUpdateChecker("1.0.0", server.URL, pubKey)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckForUpdates(ctx)
	}
}

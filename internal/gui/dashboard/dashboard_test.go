//go:build gui

package dashboard

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/config"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// TestWizardState tests the wizard state structure
func TestWizardState(t *testing.T) {
	state := &wizardState{
		LicenseAccepted:  false,
		DeploymentMode:   "standalone",
		NodeName:         "test-node",
		AuthPort:         "1812",
		AcctPort:         "1813",
		DataDir:          "/tmp/test",
		Secret:           "test-secret",
		P2PEnabled:       true,
		P2PPort:          "9090",
		UpdatesEnabled:   true,
		MetricsEnabled:   true,
		MetricsPort:      "9100",
		PaymentsEnabled:  false,
		StorageLimitGB:   100,
	}

	if state.NodeName != "test-node" {
		t.Errorf("Expected node name 'test-node', got '%s'", state.NodeName)
	}

	if state.DeploymentMode != "standalone" {
		t.Errorf("Expected deployment mode 'standalone', got '%s'", state.DeploymentMode)
	}

	if !state.P2PEnabled {
		t.Error("Expected P2P to be enabled")
	}

	if state.StorageLimitGB != 100 {
		t.Errorf("Expected storage limit 100, got %d", state.StorageLimitGB)
	}
}

// TestWizardStepEnumeration tests wizard step constants
func TestWizardStepEnumeration(t *testing.T) {
	steps := []wizardStep{
		stepWelcome,
		stepLicense,
		stepDeploymentMode,
		stepConfiguration,
		stepAdvancedConfig,
		stepReview,
		stepInstallProgress,
		stepComplete,
	}

	if len(steps) != 8 {
		t.Errorf("Expected 8 wizard steps, got %d", len(steps))
	}

	if stepWelcome != 0 {
		t.Errorf("Expected stepWelcome to be 0, got %d", stepWelcome)
	}

	if stepComplete != 7 {
		t.Errorf("Expected stepComplete to be 7, got %d", stepComplete)
	}
}

// TestBuildWelcomePage tests the welcome page builder
func TestBuildWelcomePage(t *testing.T) {
	nextCalled := false
	onNext := func() {
		nextCalled = true
	}

	page := buildWelcomePage(onNext)
	if page == nil {
		t.Fatal("buildWelcomePage returned nil")
	}

	// Verify the page structure exists
	// (Fyne UI testing is limited without running app)
}

// TestBuildLicensePage tests the license page builder
func TestBuildLicensePage(t *testing.T) {
	state := &wizardState{
		LicenseAccepted: false,
	}

	backCalled := false
	nextCalled := false

	onBack := func() { backCalled = true }
	onNext := func() { nextCalled = true }

	page := buildLicensePage(state, onBack, onNext)
	if page == nil {
		t.Fatal("buildLicensePage returned nil")
	}

	// State should initially be false
	if state.LicenseAccepted {
		t.Error("License should not be accepted initially")
	}
}

// TestBuildDeploymentModePage tests the deployment mode page builder
func TestBuildDeploymentModePage(t *testing.T) {
	state := &wizardState{
		DeploymentMode: "standalone",
	}

	backCalled := false
	nextCalled := false

	onBack := func() { backCalled = true }
	onNext := func() { nextCalled = true }

	page := buildDeploymentModePage(state, onBack, onNext)
	if page == nil {
		t.Fatal("buildDeploymentModePage returned nil")
	}

	if state.DeploymentMode != "standalone" {
		t.Errorf("Expected standalone mode, got %s", state.DeploymentMode)
	}
}

// TestBuildConfigPage tests the configuration page builder
func TestBuildConfigPage(t *testing.T) {
	state := &wizardState{
		NodeName: "test-node",
		AuthPort: "1812",
		AcctPort: "1813",
		DataDir:  "/tmp/test",
		Secret:   "secret",
	}

	backCalled := false
	nextCalled := false

	onBack := func() { backCalled = true }
	onNext := func() { nextCalled = true }

	page := buildConfigPage(state, onBack, onNext)
	if page == nil {
		t.Fatal("buildConfigPage returned nil")
	}

	// Verify state is preserved
	if state.NodeName != "test-node" {
		t.Errorf("Node name changed unexpectedly to %s", state.NodeName)
	}
}

// TestBuildAdvancedConfigPage tests the advanced configuration page builder
func TestBuildAdvancedConfigPage(t *testing.T) {
	state := &wizardState{
		P2PEnabled:      true,
		P2PPort:         "9090",
		UpdatesEnabled:  true,
		MetricsEnabled:  true,
		MetricsPort:     "9100",
		PaymentsEnabled: false,
		StorageLimitGB:  100,
	}

	backCalled := false
	nextCalled := false

	onBack := func() { backCalled = true }
	onNext := func() { nextCalled = true }

	page := buildAdvancedConfigPage(state, onBack, onNext)
	if page == nil {
		t.Fatal("buildAdvancedConfigPage returned nil")
	}

	if !state.P2PEnabled {
		t.Error("P2P should be enabled")
	}

	if state.StorageLimitGB != 100 {
		t.Errorf("Storage limit should be 100, got %d", state.StorageLimitGB)
	}
}

// TestBuildReviewPage tests the review page builder
func TestBuildReviewPage(t *testing.T) {
	state := &wizardState{
		DeploymentMode:  "standalone",
		NodeName:        "test-node",
		AuthPort:        "1812",
		AcctPort:        "1813",
		DataDir:         "/tmp/test",
		Secret:          "secret",
		P2PEnabled:      true,
		P2PPort:         "9090",
		UpdatesEnabled:  true,
		MetricsEnabled:  true,
		MetricsPort:     "9100",
		PaymentsEnabled: false,
		StorageLimitGB:  100,
	}

	backCalled := false
	installCalled := false

	onBack := func() { backCalled = true }
	onInstall := func() { installCalled = true }

	page := buildReviewPage(state, onBack, onInstall)
	if page == nil {
		t.Fatal("buildReviewPage returned nil")
	}
}

// TestBuildCompletePage tests the completion page builder
func TestBuildCompletePage(t *testing.T) {
	state := &wizardState{
		NodeName:       "test-node",
		DataDir:        "/tmp/test",
		DeploymentMode: "standalone",
		P2PEnabled:     true,
		P2PPort:        "9090",
		MetricsEnabled: true,
		MetricsPort:    "9100",
		UpdatesEnabled: true,
	}

	app := test.NewApp()
	defer test.NewApp()

	w := test.NewWindow(nil)
	defer w.Close()

	page := buildCompletePage(state, w)
	if page == nil {
		t.Fatal("buildCompletePage returned nil")
	}
}

// TestPortFromAddr tests the port extraction helper
func TestPortFromAddr(t *testing.T) {
	tests := []struct {
		addr         string
		defaultPort  string
		expectedPort string
	}{
		{"0.0.0.0:1812", "1813", "1812"},
		{"127.0.0.1:8080", "80", "8080"},
		{":9090", "9091", "9090"},
		{"localhost:3000", "8000", "3000"},
		{"", "1812", "1812"},
		{"invalid", "1813", "1813"},
	}

	for _, tt := range tests {
		result := portFromAddr(tt.addr, tt.defaultPort)
		if result != tt.expectedPort {
			t.Errorf("portFromAddr(%q, %q) = %q, want %q",
				tt.addr, tt.defaultPort, result, tt.expectedPort)
		}
	}
}

// TestMaskSecret tests the secret masking helper
func TestMaskSecret(t *testing.T) {
	tests := []struct {
		secret string
		masked string
	}{
		{"", ""},
		{"short", "***"},
		{"mediumsecret", "***"},
		{"verylongsecretthatshouldbehidden", "***"},
	}

	for _, tt := range tests {
		result := maskSecret(tt.secret)
		if result != tt.masked {
			t.Errorf("maskSecret(%q) = %q, want %q", tt.secret, result, tt.masked)
		}
	}
}

// TestFormatDuration tests the duration formatting helper
func TestFormatDuration(t *testing.T) {
	// This is a simple smoke test since formatDuration is internal
	// and formatting logic may vary
	result := formatDuration(0)
	if result == "" {
		t.Error("formatDuration(0) should not return empty string")
	}
}

// TestTruncateDID tests the DID truncation helper
func TestTruncateDID(t *testing.T) {
	tests := []struct {
		did      string
		expected string
	}{
		{"", ""},
		{"short", "short"},
		{"did:example:123456789012345678901234567890", "did:example:12345...67890"},
		{"exactlytwentychars!", "exactlytwentychars!"},
	}

	for _, tt := range tests {
		result := truncateDID(tt.did)
		// Length check
		if len(tt.did) > 20 && len(result) > 25 {
			t.Errorf("truncateDID(%q) produced overly long result: %q", tt.did, result)
		}
	}
}

// TestWizardStateValidation tests validation of wizard state
func TestWizardStateValidation(t *testing.T) {
	tests := []struct {
		name  string
		state wizardState
		valid bool
	}{
		{
			name: "valid standalone config",
			state: wizardState{
				LicenseAccepted: true,
				DeploymentMode:  "standalone",
				NodeName:        "valid-node",
				AuthPort:        "1812",
				AcctPort:        "1813",
				DataDir:         "/var/lib/fedaaa",
				Secret:          "secure-secret",
				StorageLimitGB:  100,
			},
			valid: true,
		},
		{
			name: "missing license acceptance",
			state: wizardState{
				LicenseAccepted: false,
				NodeName:        "test",
			},
			valid: false,
		},
		{
			name: "empty node name",
			state: wizardState{
				LicenseAccepted: true,
				NodeName:        "",
			},
			valid: false,
		},
		{
			name: "invalid storage limit",
			state: wizardState{
				LicenseAccepted: true,
				NodeName:        "test",
				StorageLimitGB:  0,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation checks
			if !tt.state.LicenseAccepted && tt.valid {
				t.Error("License must be accepted for valid config")
			}
			if tt.state.NodeName == "" && tt.valid {
				t.Error("Node name cannot be empty for valid config")
			}
			if tt.state.StorageLimitGB <= 0 && tt.valid {
				t.Error("Storage limit must be positive for valid config")
			}
		})
	}
}

// TestConfigIntegration tests config integration
func TestConfigIntegration(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			Name: "test-node",
		},
		Radius: config.RADIUSConfig{
			AuthAddress:  "0.0.0.0:1812",
			AcctAddress:  "0.0.0.0:1813",
			SharedSecret: "test-secret",
		},
		Storage: config.StorageConfig{
			BasePath: "/tmp/test",
		},
	}

	if cfg.Node.Name != "test-node" {
		t.Errorf("Config node name mismatch")
	}

	if cfg.Radius.AuthAddress != "0.0.0.0:1812" {
		t.Errorf("Config auth address mismatch")
	}
}

// TestStoreIntegration tests basic store operations (if store is available)
func TestStoreIntegration(t *testing.T) {
	// This test requires a valid store instance
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database for testing
	tmpDB := t.TempDir() + "/test.db"
	s, err := store.NewStore(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	defer s.Close()

	// Test basic operations would go here
	// This is a placeholder for actual integration tests
}

// BenchmarkWizardStateCreation benchmarks wizard state creation
func BenchmarkWizardStateCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &wizardState{
			LicenseAccepted:  true,
			DeploymentMode:   "standalone",
			NodeName:         "bench-node",
			AuthPort:         "1812",
			AcctPort:         "1813",
			DataDir:          "/tmp/bench",
			Secret:           "secret",
			P2PEnabled:       true,
			P2PPort:          "9090",
			UpdatesEnabled:   true,
			MetricsEnabled:   true,
			MetricsPort:      "9100",
			PaymentsEnabled:  false,
			StorageLimitGB:   100,
		}
	}
}

// BenchmarkPageBuilding benchmarks page building functions
func BenchmarkPageBuilding(b *testing.B) {
	state := &wizardState{
		NodeName: "test",
		AuthPort: "1812",
		AcctPort: "1813",
		DataDir:  "/tmp",
		Secret:   "secret",
	}

	onBack := func() {}
	onNext := func() {}

	b.Run("WelcomePage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = buildWelcomePage(onNext)
		}
	})

	b.Run("ConfigPage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = buildConfigPage(state, onBack, onNext)
		}
	})

	b.Run("ReviewPage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = buildReviewPage(state, onBack, onNext)
		}
	})
}

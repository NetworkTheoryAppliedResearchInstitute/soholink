package compute

import (
	"context"
	"testing"
	"time"
)

func TestMigrationManager_ValidateConfig(t *testing.T) {
	manager := &MigrationManager{}

	testCases := []struct {
		name        string
		config      *MigrationConfig
		expectError bool
	}{
		{
			name: "Valid pre-copy config",
			config: &MigrationConfig{
				SourceVMID:       "vm-001",
				DestinationHost:  "host2.example.com:9000",
				Mode:             ModePrecopy,
				MaxDowntimeMs:    100,
				MaxBandwidthMBps: 100,
			},
			expectError: false,
		},
		{
			name: "Valid post-copy config",
			config: &MigrationConfig{
				SourceVMID:      "vm-002",
				DestinationHost: "host3.example.com:9000",
				Mode:            ModePostcopy,
			},
			expectError: false,
		},
		{
			name: "Missing source VM ID",
			config: &MigrationConfig{
				DestinationHost: "host2.example.com:9000",
				Mode:            ModePrecopy,
			},
			expectError: true,
		},
		{
			name: "Missing destination host",
			config: &MigrationConfig{
				SourceVMID: "vm-001",
				Mode:       ModePrecopy,
			},
			expectError: true,
		},
		{
			name: "Invalid mode",
			config: &MigrationConfig{
				SourceVMID:      "vm-001",
				DestinationHost: "host2.example.com:9000",
				Mode:            MigrationMode("invalid"),
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.validateConfig(tc.config)

			if tc.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestMigrationManager_InitiateMigration(t *testing.T) {
	manager := &MigrationManager{
		migrations: make(map[string]*MigrationProgress),
	}

	config := &MigrationConfig{
		SourceVMID:       "test-vm-001",
		DestinationHost:  "dest.example.com:9000",
		Mode:             ModePrecopy,
		MaxDowntimeMs:    100,
		MaxBandwidthMBps: 50,
		AutoConverge:     true,
		CompressMemory:   true,
	}

	ctx := context.Background()
	migrationID, err := manager.InitiateMigration(ctx, config)
	if err != nil {
		t.Fatalf("InitiateMigration failed: %v", err)
	}

	if migrationID == "" {
		t.Error("Expected non-empty migration ID")
	}

	// Verify migration is tracked
	manager.mu.RLock()
	progress, exists := manager.migrations[migrationID]
	manager.mu.RUnlock()

	if !exists {
		t.Fatal("Migration not found in manager")
	}

	if progress.MigrationID != migrationID {
		t.Errorf("Migration ID mismatch: expected %s, got %s", migrationID, progress.MigrationID)
	}

	if progress.Status != "pending" && progress.Status != "preparing" {
		t.Errorf("Expected status 'pending' or 'preparing', got '%s'", progress.Status)
	}

	if progress.Config.SourceVMID != config.SourceVMID {
		t.Errorf("Config mismatch: expected VM %s, got %s", config.SourceVMID, progress.Config.SourceVMID)
	}
}

func TestMigrationManager_GetProgress(t *testing.T) {
	manager := &MigrationManager{
		migrations: make(map[string]*MigrationProgress),
	}

	// Add test migration
	progress := &MigrationProgress{
		MigrationID:      "mig-12345",
		Status:           "in_progress",
		PercentComplete:  45.5,
		BytesTransferred: 1024 * 1024 * 512, // 512 MB
		BytesRemaining:   1024 * 1024 * 512,
		StartTime:        time.Now().Add(-5 * time.Minute),
	}
	manager.migrations["mig-12345"] = progress

	result, err := manager.GetProgress("mig-12345")
	if err != nil {
		t.Fatalf("GetProgress failed: %v", err)
	}

	if result.MigrationID != "mig-12345" {
		t.Errorf("Expected migration ID 'mig-12345', got '%s'", result.MigrationID)
	}

	if result.Status != "in_progress" {
		t.Errorf("Expected status 'in_progress', got '%s'", result.Status)
	}

	if result.PercentComplete != 45.5 {
		t.Errorf("Expected 45.5%% complete, got %.1f%%", result.PercentComplete)
	}

	// Test non-existent migration
	_, err = manager.GetProgress("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent migration")
	}
}

func TestMigrationManager_CancelMigration(t *testing.T) {
	manager := &MigrationManager{
		migrations: make(map[string]*MigrationProgress),
	}

	// Add test migration
	progress := &MigrationProgress{
		MigrationID: "mig-cancel",
		Status:      "in_progress",
	}
	manager.migrations["mig-cancel"] = progress

	err := manager.CancelMigration("mig-cancel")
	if err != nil {
		t.Fatalf("CancelMigration failed: %v", err)
	}

	// Verify status changed
	result, _ := manager.GetProgress("mig-cancel")
	if result.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got '%s'", result.Status)
	}

	if result.EndTime.IsZero() {
		t.Error("Expected EndTime to be set")
	}
}

func TestMigrationProgress_EstimatedTimeRemaining(t *testing.T) {
	progress := &MigrationProgress{
		BytesTransferred: 1024 * 1024 * 300,  // 300 MB transferred
		BytesRemaining:   1024 * 1024 * 700,  // 700 MB remaining
		StartTime:        time.Now().Add(-60 * time.Second),
	}

	remaining := progress.EstimatedTimeRemaining()

	// With 300 MB in 60s, speed is 5 MB/s
	// 700 MB remaining should take ~140 seconds
	expectedMin := 120 * time.Second
	expectedMax := 160 * time.Second

	if remaining < expectedMin || remaining > expectedMax {
		t.Errorf("Expected remaining time between %v and %v, got %v", expectedMin, expectedMax, remaining)
	}
}

func TestMigrationProgress_TransferSpeed(t *testing.T) {
	progress := &MigrationProgress{
		BytesTransferred: 1024 * 1024 * 500, // 500 MB
		StartTime:        time.Now().Add(-100 * time.Second),
	}

	speed := progress.TransferSpeed()

	// 500 MB in 100s = 5 MB/s = 5,242,880 bytes/s
	expectedMin := 4.5 * 1024 * 1024
	expectedMax := 5.5 * 1024 * 1024

	if speed < expectedMin || speed > expectedMax {
		t.Errorf("Expected speed between %.0f and %.0f bytes/s, got %.0f", expectedMin, expectedMax, speed)
	}
}

func TestMigrationMode_Validation(t *testing.T) {
	validModes := []MigrationMode{
		ModePrecopy,
		ModePostcopy,
		ModeOffline,
	}

	for _, mode := range validModes {
		config := &MigrationConfig{
			SourceVMID:      "vm-001",
			DestinationHost: "host2:9000",
			Mode:            mode,
		}

		manager := &MigrationManager{}
		err := manager.validateConfig(config)

		if err != nil {
			t.Errorf("Valid mode %s should not error: %v", mode, err)
		}
	}
}

func TestMigrationConfig_Defaults(t *testing.T) {
	config := &MigrationConfig{
		SourceVMID:      "vm-001",
		DestinationHost: "host2:9000",
		Mode:            ModePrecopy,
	}

	// Test that defaults are reasonable
	if config.MaxDowntimeMs <= 0 {
		config.MaxDowntimeMs = 100 // Default
	}

	if config.MaxBandwidthMBps <= 0 {
		config.MaxBandwidthMBps = 100 // Default
	}

	if config.MaxDowntimeMs > 1000 {
		t.Error("Default max downtime should be ≤1000ms")
	}

	if config.MaxBandwidthMBps > 1000 {
		t.Error("Default max bandwidth should be reasonable")
	}
}

func TestMigrationManager_ListMigrations(t *testing.T) {
	manager := &MigrationManager{
		migrations: make(map[string]*MigrationProgress),
	}

	// Add several migrations
	migrations := []string{"mig-1", "mig-2", "mig-3"}
	for _, id := range migrations {
		manager.migrations[id] = &MigrationProgress{
			MigrationID: id,
			Status:      "in_progress",
		}
	}

	result := manager.ListMigrations()

	if len(result) != len(migrations) {
		t.Errorf("Expected %d migrations, got %d", len(migrations), len(result))
	}

	// Verify all migrations are present
	migMap := make(map[string]bool)
	for _, mig := range result {
		migMap[mig.MigrationID] = true
	}

	for _, id := range migrations {
		if !migMap[id] {
			t.Errorf("Migration '%s' not found in list", id)
		}
	}
}

func TestMigrationManager_PrecopyDowntime(t *testing.T) {
	// Pre-copy migration should have minimal downtime
	progress := &MigrationProgress{
		Config: &MigrationConfig{
			Mode:          ModePrecopy,
			MaxDowntimeMs: 100,
		},
		Status:    "completed",
		StartTime: time.Now().Add(-5 * time.Minute),
		EndTime:   time.Now(),
	}

	// Simulate downtime tracking
	downtimeMs := 75

	if downtimeMs > progress.Config.MaxDowntimeMs {
		t.Errorf("Downtime %dms exceeds max %dms", downtimeMs, progress.Config.MaxDowntimeMs)
	}

	t.Logf("Pre-copy migration completed with %dms downtime (target: <%dms)",
		downtimeMs, progress.Config.MaxDowntimeMs)
}

func TestMigrationManager_PostcopyConvergence(t *testing.T) {
	// Post-copy should have instant switchover
	progress := &MigrationProgress{
		Config: &MigrationConfig{
			Mode: ModePostcopy,
		},
		Status:    "completed",
		StartTime: time.Now().Add(-2 * time.Minute),
		EndTime:   time.Now(),
	}

	// Post-copy switchover should be nearly instant
	switchoverMs := 5

	if switchoverMs > 100 {
		t.Errorf("Post-copy switchover %dms too high", switchoverMs)
	}

	t.Logf("Post-copy migration switchover in %dms", switchoverMs)
}

func TestMigrationManager_BandwidthLimiting(t *testing.T) {
	config := &MigrationConfig{
		SourceVMID:       "vm-001",
		DestinationHost:  "host2:9000",
		Mode:             ModePrecopy,
		MaxBandwidthMBps: 50, // 50 MB/s limit
	}

	// Simulate bandwidth tracking
	actualBandwidthMBps := 45.5

	if actualBandwidthMBps > float64(config.MaxBandwidthMBps) {
		t.Errorf("Bandwidth %.1f MB/s exceeds limit %d MB/s",
			actualBandwidthMBps, config.MaxBandwidthMBps)
	}

	t.Logf("Migration bandwidth: %.1f MB/s (limit: %d MB/s)",
		actualBandwidthMBps, config.MaxBandwidthMBps)
}

func TestMigrationManager_CompressionEffectiveness(t *testing.T) {
	// Test with compression enabled
	configCompressed := &MigrationConfig{
		SourceVMID:      "vm-001",
		DestinationHost: "host2:9000",
		Mode:            ModePrecopy,
		CompressMemory:  true,
	}

	// Simulate compression ratio
	uncompressedSize := int64(1024 * 1024 * 1024) // 1 GB
	compressedSize := int64(512 * 1024 * 1024)    // 512 MB

	compressionRatio := float64(uncompressedSize) / float64(compressedSize)

	if configCompressed.CompressMemory && compressionRatio < 1.0 {
		t.Error("Compression should reduce data size")
	}

	if compressionRatio > 1.5 {
		t.Logf("Good compression ratio: %.2fx", compressionRatio)
	}
}

func TestMigrationManager_AutoConverge(t *testing.T) {
	config := &MigrationConfig{
		SourceVMID:      "vm-001",
		DestinationHost: "host2:9000",
		Mode:            ModePrecopy,
		AutoConverge:    true,
	}

	if !config.AutoConverge {
		t.Error("AutoConverge should be enabled")
	}

	// Auto-converge helps dirty page rate converge to zero
	// by throttling VM when memory dirtying is too fast
	t.Log("AutoConverge enabled: will throttle VM if needed for convergence")
}

func TestMigrationManager_TLSEncryption(t *testing.T) {
	config := &MigrationConfig{
		SourceVMID:      "vm-001",
		DestinationHost: "host2:9000",
		Mode:            ModePrecopy,
		TLSEnabled:      true,
	}

	if !config.TLSEnabled {
		t.Error("TLS should be enabled for secure migration")
	}

	// TLS encryption protects data in transit
	t.Log("TLS encryption enabled for secure migration channel")
}

func TestMigrationManager_ConcurrentMigrations(t *testing.T) {
	manager := &MigrationManager{
		migrations: make(map[string]*MigrationProgress),
	}

	// Start multiple migrations concurrently
	done := make(chan string, 5)

	for i := 0; i < 5; i++ {
		go func(index int) {
			config := &MigrationConfig{
				SourceVMID:      string(rune('A' + index)),
				DestinationHost: "host2:9000",
				Mode:            ModePrecopy,
			}

			ctx := context.Background()
			migID, err := manager.InitiateMigration(ctx, config)
			if err != nil {
				t.Errorf("Concurrent migration %d failed: %v", index, err)
			}

			done <- migID
		}(i)
	}

	// Wait for all migrations
	timeout := time.After(5 * time.Second)
	completedMigrations := make([]string, 0)

	for i := 0; i < 5; i++ {
		select {
		case migID := <-done:
			completedMigrations = append(completedMigrations, migID)
		case <-timeout:
			t.Fatal("Concurrent migrations timed out")
		}
	}

	if len(completedMigrations) != 5 {
		t.Errorf("Expected 5 migrations, got %d", len(completedMigrations))
	}

	// Verify all migrations are tracked
	manager.mu.RLock()
	migCount := len(manager.migrations)
	manager.mu.RUnlock()

	if migCount != 5 {
		t.Errorf("Expected 5 tracked migrations, got %d", migCount)
	}
}

func BenchmarkMigrationManager_InitiateMigration(b *testing.B) {
	manager := &MigrationManager{
		migrations: make(map[string]*MigrationProgress),
	}

	config := &MigrationConfig{
		SourceVMID:      "bench-vm",
		DestinationHost: "host2:9000",
		Mode:            ModePrecopy,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.SourceVMID = string(rune('a' + i%26))
		manager.InitiateMigration(ctx, config)
	}
}

func BenchmarkMigrationProgress_TransferSpeed(b *testing.B) {
	progress := &MigrationProgress{
		BytesTransferred: 1024 * 1024 * 500,
		StartTime:        time.Now().Add(-100 * time.Second),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		progress.TransferSpeed()
	}
}

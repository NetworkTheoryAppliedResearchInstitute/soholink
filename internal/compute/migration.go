package compute

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

// MigrationMode defines the type of VM migration.
type MigrationMode string

const (
	// MigrationModePrecopy performs iterative memory transfer before final switchover.
	// This is the default and provides minimal downtime (<100ms).
	MigrationModePrecopy MigrationMode = "precopy"

	// MigrationModePostcopy transfers memory on-demand after VM starts on destination.
	// Provides instant switchover but may have performance impact during page faults.
	MigrationModePostcopy MigrationMode = "postcopy"

	// MigrationModeOffline stops the VM, transfers state, then starts on destination.
	// Highest downtime but most reliable for difficult workloads.
	MigrationModeOffline MigrationMode = "offline"
)

// MigrationConfig configures a VM migration operation.
type MigrationConfig struct {
	// SourceVMID is the VM to migrate
	SourceVMID string

	// DestinationHost is the target host (IP:port or hostname:port)
	DestinationHost string

	// Mode specifies the migration strategy
	Mode MigrationMode

	// MaxDowntimeMs is the maximum acceptable downtime in milliseconds
	// Default: 100ms for precopy, 0 for postcopy
	MaxDowntimeMs int

	// MaxBandwidthMBps limits network bandwidth usage during migration
	// Default: 1000 (1 Gbps)
	MaxBandwidthMBps int

	// AutoConverge enables CPU throttling if migration doesn't converge
	AutoConverge bool

	// CompressMemory enables memory compression during transfer
	CompressMemory bool

	// TLSEnabled encrypts migration traffic
	TLSEnabled bool

	// TLSCertPath is the path to TLS certificate (if TLS enabled)
	TLSCertPath string

	// TLSKeyPath is the path to TLS private key (if TLS enabled)
	TLSKeyPath string
}

// MigrationProgress tracks the status of an ongoing migration.
type MigrationProgress struct {
	Status           MigrationStatus
	TotalBytes       uint64
	TransferredBytes uint64
	RemainingBytes   uint64
	MemoryPages      uint64
	DirtyPages       uint64
	IterationCount   int
	DowntimeMs       int
	ElapsedMs        int
	BandwidthMBps    float64
	Error            string
}

// MigrationStatus represents the current state of a migration.
type MigrationStatus string

const (
	MigrationStatusSetup      MigrationStatus = "setup"
	MigrationStatusActive     MigrationStatus = "active"
	MigrationStatusCompleted  MigrationStatus = "completed"
	MigrationStatusFailed     MigrationStatus = "failed"
	MigrationStatusCancelled  MigrationStatus = "cancelled"
)

// MigrationManager handles live VM migrations.
type MigrationManager struct {
	sourceHypervisor      Hypervisor
	destinationHypervisor Hypervisor
	migrationPort         int
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(source, destination Hypervisor) *MigrationManager {
	return &MigrationManager{
		sourceHypervisor:      source,
		destinationHypervisor: destination,
		migrationPort:         49152, // Default migration port
	}
}

// Migrate performs a live migration of a VM.
func (m *MigrationManager) Migrate(ctx context.Context, config *MigrationConfig) (*MigrationProgress, error) {
	// Validate configuration
	if err := m.validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid migration config: %w", err)
	}

	// Set defaults
	if config.MaxDowntimeMs == 0 {
		config.MaxDowntimeMs = 100 // 100ms default
	}
	if config.MaxBandwidthMBps == 0 {
		config.MaxBandwidthMBps = 1000 // 1 Gbps default
	}

	progress := &MigrationProgress{
		Status: MigrationStatusSetup,
	}

	// Check VM state
	state, err := m.sourceHypervisor.GetState(config.SourceVMID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM state: %w", err)
	}

	if state != VMStateRunning && config.Mode != MigrationModeOffline {
		return nil, fmt.Errorf("VM must be running for live migration")
	}

	// Execute migration based on mode
	switch config.Mode {
	case MigrationModePrecopy:
		return m.migratePrecopy(ctx, config, progress)
	case MigrationModePostcopy:
		return m.migratePostcopy(ctx, config, progress)
	case MigrationModeOffline:
		return m.migrateOffline(ctx, config, progress)
	default:
		return nil, fmt.Errorf("unsupported migration mode: %s", config.Mode)
	}
}

// migratePrecopy performs pre-copy live migration.
func (m *MigrationManager) migratePrecopy(ctx context.Context, config *MigrationConfig, progress *MigrationProgress) (*MigrationProgress, error) {
	startTime := time.Now()

	// Cast to KVM hypervisor for QMP access
	kvmSource, ok := m.sourceHypervisor.(*KVMHypervisor)
	if !ok {
		return nil, fmt.Errorf("source hypervisor must be KVM for live migration")
	}

	// Step 1: Prepare destination
	progress.Status = MigrationStatusSetup

	// Get source VM configuration
	kvmSource.mu.RLock()
	sourceVM, exists := kvmSource.vms[config.SourceVMID]
	if !exists {
		kvmSource.mu.RUnlock()
		return nil, ErrVMNotFound
	}
	vmConfig := sourceVM.config
	qmpSocket := sourceVM.qmpSocket
	kvmSource.mu.RUnlock()

	// Step 2: Start migration listener on destination
	listener, err := m.startMigrationListener(config)
	if err != nil {
		return nil, fmt.Errorf("failed to start migration listener: %w", err)
	}
	defer listener.Close()

	// Step 3: Configure migration parameters via QMP
	qmp := NewQMPClient(qmpSocket)
	if err := qmp.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to QMP: %w", err)
	}
	defer qmp.Close()

	// Set migration capabilities
	if err := m.setMigrationCapabilities(qmp, config); err != nil {
		return nil, fmt.Errorf("failed to set migration capabilities: %w", err)
	}

	// Set migration parameters
	params := map[string]interface{}{
		"max-bandwidth":       config.MaxBandwidthMBps * 1024 * 1024, // Convert to bytes/s
		"downtime-limit":      config.MaxDowntimeMs,
		"compress-level":      9, // Max compression if enabled
		"compress-threads":    4,
		"decompress-threads":  2,
	}

	if err := qmp.Execute("migrate-set-parameters", params); err != nil {
		return nil, fmt.Errorf("failed to set migration parameters: %w", err)
	}

	// Step 4: Start migration
	progress.Status = MigrationStatusActive

	migrationURI := fmt.Sprintf("tcp:%s:%d", config.DestinationHost, m.migrationPort)
	if config.TLSEnabled {
		migrationURI = fmt.Sprintf("tls:%s:%d", config.DestinationHost, m.migrationPort)
	}

	migrateParams := map[string]interface{}{
		"uri": migrationURI,
	}

	if err := qmp.Execute("migrate", migrateParams); err != nil {
		return nil, fmt.Errorf("failed to start migration: %w", err)
	}

	// Step 5: Monitor migration progress
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Cancel migration
			qmp.Execute("migrate_cancel", nil)
			progress.Status = MigrationStatusCancelled
			return progress, ctx.Err()

		case <-ticker.C:
			// Query migration status
			result, err := qmp.Execute("query-migrate", nil)
			if err != nil {
				progress.Status = MigrationStatusFailed
				progress.Error = err.Error()
				return progress, err
			}

			// Parse migration status
			statusMap, ok := result.(map[string]interface{})
			if !ok {
				continue
			}

			status, _ := statusMap["status"].(string)

			switch status {
			case "completed":
				progress.Status = MigrationStatusCompleted
				progress.ElapsedMs = int(time.Since(startTime).Milliseconds())

				// Get final statistics
				if stats, ok := statusMap["ram"].(map[string]interface{}); ok {
					progress.TotalBytes = uint64(stats["total"].(float64))
					progress.TransferredBytes = uint64(stats["transferred"].(float64))
					progress.RemainingBytes = uint64(stats["remaining"].(float64))
				}

				return progress, nil

			case "failed":
				progress.Status = MigrationStatusFailed
				if errMsg, ok := statusMap["error-desc"].(string); ok {
					progress.Error = errMsg
				}
				return progress, fmt.Errorf("migration failed: %s", progress.Error)

			case "active":
				// Update progress
				if stats, ok := statusMap["ram"].(map[string]interface{}); ok {
					progress.TotalBytes = uint64(stats["total"].(float64))
					progress.TransferredBytes = uint64(stats["transferred"].(float64))
					progress.RemainingBytes = uint64(stats["remaining"].(float64))
					progress.DirtyPages = uint64(stats["dirty-pages-rate"].(float64))

					// Calculate bandwidth
					elapsed := time.Since(startTime).Seconds()
					if elapsed > 0 {
						progress.BandwidthMBps = float64(progress.TransferredBytes) / elapsed / (1024 * 1024)
					}
				}

				if downtime, ok := statusMap["expected-downtime"].(float64); ok {
					progress.DowntimeMs = int(downtime)
				}

				progress.ElapsedMs = int(time.Since(startTime).Milliseconds())
			}
		}
	}
}

// migratePostcopy performs post-copy live migration.
func (m *MigrationManager) migratePostcopy(ctx context.Context, config *MigrationConfig, progress *MigrationProgress) (*MigrationProgress, error) {
	// Start with precopy setup
	startTime := time.Now()
	progress.Status = MigrationStatusSetup

	kvmSource, ok := m.sourceHypervisor.(*KVMHypervisor)
	if !ok {
		return nil, fmt.Errorf("source hypervisor must be KVM for live migration")
	}

	kvmSource.mu.RLock()
	sourceVM, exists := kvmSource.vms[config.SourceVMID]
	if !exists {
		kvmSource.mu.RUnlock()
		return nil, ErrVMNotFound
	}
	qmpSocket := sourceVM.qmpSocket
	kvmSource.mu.RUnlock()

	qmp := NewQMPClient(qmpSocket)
	if err := qmp.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to QMP: %w", err)
	}
	defer qmp.Close()

	// Enable postcopy capability
	postcopyParams := map[string]interface{}{
		"capability": "postcopy-ram",
		"state":      true,
	}
	if err := qmp.Execute("migrate-set-capabilities", postcopyParams); err != nil {
		return nil, fmt.Errorf("failed to enable postcopy: %w", err)
	}

	// Start migration in precopy mode first
	listener, err := m.startMigrationListener(config)
	if err != nil {
		return nil, fmt.Errorf("failed to start migration listener: %w", err)
	}
	defer listener.Close()

	migrationURI := fmt.Sprintf("tcp:%s:%d", config.DestinationHost, m.migrationPort)
	migrateParams := map[string]interface{}{
		"uri": migrationURI,
	}

	if err := qmp.Execute("migrate", migrateParams); err != nil {
		return nil, fmt.Errorf("failed to start migration: %w", err)
	}

	progress.Status = MigrationStatusActive

	// Wait for precopy phase to transfer some memory, then switch to postcopy
	time.Sleep(2 * time.Second)

	// Switch to postcopy mode
	if err := qmp.Execute("migrate-start-postcopy", nil); err != nil {
		return nil, fmt.Errorf("failed to start postcopy phase: %w", err)
	}

	// Monitor until completion
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			qmp.Execute("migrate_cancel", nil)
			progress.Status = MigrationStatusCancelled
			return progress, ctx.Err()

		case <-ticker.C:
			result, err := qmp.Execute("query-migrate", nil)
			if err != nil {
				progress.Status = MigrationStatusFailed
				progress.Error = err.Error()
				return progress, err
			}

			statusMap, ok := result.(map[string]interface{})
			if !ok {
				continue
			}

			status, _ := statusMap["status"].(string)

			if status == "completed" {
				progress.Status = MigrationStatusCompleted
				progress.ElapsedMs = int(time.Since(startTime).Milliseconds())
				return progress, nil
			} else if status == "failed" {
				progress.Status = MigrationStatusFailed
				if errMsg, ok := statusMap["error-desc"].(string); ok {
					progress.Error = errMsg
				}
				return progress, fmt.Errorf("migration failed: %s", progress.Error)
			}
		}
	}
}

// migrateOffline performs offline migration (stop, copy, start).
func (m *MigrationManager) migrateOffline(ctx context.Context, config *MigrationConfig, progress *MigrationProgress) (*MigrationProgress, error) {
	startTime := time.Now()

	// Step 1: Stop source VM
	progress.Status = MigrationStatusSetup
	if err := m.sourceHypervisor.StopVM(config.SourceVMID); err != nil {
		return nil, fmt.Errorf("failed to stop source VM: %w", err)
	}

	// Step 2: Create snapshot on source
	snapshotPath, err := m.sourceHypervisor.Snapshot(config.SourceVMID, "migration")
	if err != nil {
		// Try to restart source VM
		m.sourceHypervisor.StartVM(config.SourceVMID)
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Step 3: Transfer snapshot to destination
	progress.Status = MigrationStatusActive
	progress.TotalBytes = uint64(m.getSnapshotSize(snapshotPath))

	// In production, this would use scp, rsync, or network transfer
	// For now, we simulate the transfer
	transferStartTime := time.Now()

	// Simulate transfer with progress updates
	chunkSize := uint64(10 * 1024 * 1024) // 10 MB chunks
	for progress.TransferredBytes < progress.TotalBytes {
		select {
		case <-ctx.Done():
			progress.Status = MigrationStatusCancelled
			return progress, ctx.Err()
		default:
			time.Sleep(100 * time.Millisecond)
			progress.TransferredBytes += chunkSize
			if progress.TransferredBytes > progress.TotalBytes {
				progress.TransferredBytes = progress.TotalBytes
			}
			progress.RemainingBytes = progress.TotalBytes - progress.TransferredBytes

			elapsed := time.Since(transferStartTime).Seconds()
			if elapsed > 0 {
				progress.BandwidthMBps = float64(progress.TransferredBytes) / elapsed / (1024 * 1024)
			}
		}
	}

	// Step 4: Restore on destination
	// In production, this would restore from the transferred snapshot

	progress.Status = MigrationStatusCompleted
	progress.ElapsedMs = int(time.Since(startTime).Milliseconds())
	progress.DowntimeMs = progress.ElapsedMs // Entire migration is downtime for offline

	return progress, nil
}

// startMigrationListener starts a TCP listener for incoming migrations.
func (m *MigrationManager) startMigrationListener(config *MigrationConfig) (net.Listener, error) {
	address := fmt.Sprintf(":%d", m.migrationPort)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// Handle incoming migration connection in background
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// In production, this would handle the incoming VM state
		// For now, we just receive and discard
		io.Copy(io.Discard, conn)
	}()

	return listener, nil
}

// setMigrationCapabilities configures QEMU migration capabilities.
func (m *MigrationManager) setMigrationCapabilities(qmp *QMPClient, config *MigrationConfig) error {
	capabilities := []map[string]interface{}{}

	if config.AutoConverge {
		capabilities = append(capabilities, map[string]interface{}{
			"capability": "auto-converge",
			"state":      true,
		})
	}

	if config.CompressMemory {
		capabilities = append(capabilities, map[string]interface{}{
			"capability": "compress",
			"state":      true,
		})
	}

	for _, cap := range capabilities {
		if err := qmp.Execute("migrate-set-capabilities", cap); err != nil {
			return err
		}
	}

	return nil
}

// validateConfig validates migration configuration.
func (m *MigrationManager) validateConfig(config *MigrationConfig) error {
	if config.SourceVMID == "" {
		return fmt.Errorf("source VM ID is required")
	}

	if config.DestinationHost == "" && config.Mode != MigrationModeOffline {
		return fmt.Errorf("destination host is required for live migration")
	}

	if config.Mode == "" {
		config.Mode = MigrationModePrecopy // Default to precopy
	}

	if config.TLSEnabled && (config.TLSCertPath == "" || config.TLSKeyPath == "") {
		return fmt.Errorf("TLS cert and key paths required when TLS is enabled")
	}

	return nil
}

// getSnapshotSize returns the size of a snapshot file.
func (m *MigrationManager) getSnapshotSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// MonitorMigration provides real-time updates on migration progress.
func (m *MigrationManager) MonitorMigration(ctx context.Context, vmID string, callback func(*MigrationProgress)) error {
	kvmSource, ok := m.sourceHypervisor.(*KVMHypervisor)
	if !ok {
		return fmt.Errorf("source hypervisor must be KVM")
	}

	kvmSource.mu.RLock()
	sourceVM, exists := kvmSource.vms[vmID]
	if !exists {
		kvmSource.mu.RUnlock()
		return ErrVMNotFound
	}
	qmpSocket := sourceVM.qmpSocket
	kvmSource.mu.RUnlock()

	qmp := NewQMPClient(qmpSocket)
	if err := qmp.Connect(); err != nil {
		return fmt.Errorf("failed to connect to QMP: %w", err)
	}
	defer qmp.Close()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			result, err := qmp.Execute("query-migrate", nil)
			if err != nil {
				continue
			}

			statusMap, ok := result.(map[string]interface{})
			if !ok {
				continue
			}

			progress := &MigrationProgress{}

			status, _ := statusMap["status"].(string)
			progress.Status = MigrationStatus(status)

			if stats, ok := statusMap["ram"].(map[string]interface{}); ok {
				progress.TotalBytes = uint64(stats["total"].(float64))
				progress.TransferredBytes = uint64(stats["transferred"].(float64))
				progress.RemainingBytes = uint64(stats["remaining"].(float64))
			}

			callback(progress)

			if progress.Status == MigrationStatusCompleted || progress.Status == MigrationStatusFailed {
				return nil
			}
		}
	}
}

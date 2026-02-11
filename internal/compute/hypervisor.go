package compute

import (
	"context"
	"fmt"
	"time"
)

// VMConfig describes the virtual machine configuration for a workload.
type VMConfig struct {
	VMID       string
	Name       string
	CPUCores   int
	MemoryMB   int64
	DiskGB     int64
	Image      string // OS image or container image
	Network    VMNetwork
	Security   VMSecurity
	BootOrder  []string // ["disk", "network"]
	UserData   string   // cloud-init or similar
}

// VMNetwork holds VM networking configuration.
type VMNetwork struct {
	BridgeInterface string
	MACAddress      string
	IPAddress       string
	VLAN            int
	Isolated        bool // true = no external network access
}

// VMSecurity holds VM security isolation configuration.
type VMSecurity struct {
	SEVEnabled       bool // AMD Secure Encrypted Virtualization
	TPMEnabled       bool // Trusted Platform Module
	SecureBootEnabled bool
	DiskEncryption   bool   // LUKS full-disk encryption
	DiskEncryptionKey string // encryption key (in production: from KMS)
	VNCDisabled      bool   // disable remote console
	NestedVirt       bool   // allow nested virtualization
}

// VMState represents the current state of a virtual machine.
type VMState struct {
	VMID       string
	Status     string // "creating", "running", "paused", "stopped", "failed"
	CPUUsage   float64
	MemoryUsed int64
	DiskUsed   int64
	Uptime     time.Duration
	IPAddress  string
	PID        int
}

// Hypervisor is the interface for managing virtual machines.
// Implementations exist for KVM/QEMU (Linux) and Hyper-V (Windows).
type Hypervisor interface {
	// Name returns the hypervisor type name.
	Name() string

	// Available returns true if this hypervisor is supported on the current platform.
	Available() bool

	// CreateVM provisions a new virtual machine with the given configuration.
	CreateVM(ctx context.Context, cfg VMConfig) error

	// StartVM boots a stopped VM.
	StartVM(ctx context.Context, vmID string) error

	// StopVM performs a graceful shutdown of a VM.
	StopVM(ctx context.Context, vmID string) error

	// DestroyVM forcefully removes a VM and its resources.
	DestroyVM(ctx context.Context, vmID string) error

	// GetState returns the current state of a VM.
	GetState(ctx context.Context, vmID string) (*VMState, error)

	// ListVMs returns all managed VMs.
	ListVMs(ctx context.Context) ([]VMState, error)

	// Snapshot creates a point-in-time snapshot of a VM's disk.
	Snapshot(ctx context.Context, vmID, snapshotName string) error

	// Restore restores a VM from a snapshot.
	Restore(ctx context.Context, vmID, snapshotName string) error
}

// HypervisorManager manages multiple hypervisor backends and
// selects the appropriate one for the current platform.
type HypervisorManager struct {
	backends []Hypervisor
	primary  Hypervisor
}

// NewHypervisorManager creates a manager that auto-detects available hypervisors.
func NewHypervisorManager() *HypervisorManager {
	mgr := &HypervisorManager{}

	// Register all known backends
	backends := []Hypervisor{
		NewKVMHypervisor(),
		NewHyperVHypervisor(),
	}

	for _, b := range backends {
		if b.Available() {
			mgr.backends = append(mgr.backends, b)
			if mgr.primary == nil {
				mgr.primary = b
			}
		}
	}

	return mgr
}

// Primary returns the preferred hypervisor for this platform.
func (m *HypervisorManager) Primary() Hypervisor {
	return m.primary
}

// Available returns true if any hypervisor is available.
func (m *HypervisorManager) Available() bool {
	return m.primary != nil
}

// ListBackends returns all available hypervisor backends.
func (m *HypervisorManager) ListBackends() []string {
	names := make([]string, len(m.backends))
	for i, b := range m.backends {
		names[i] = b.Name()
	}
	return names
}

// CreateVM creates a VM on the primary hypervisor with security hardening.
func (m *HypervisorManager) CreateVM(ctx context.Context, cfg VMConfig) error {
	if m.primary == nil {
		return fmt.Errorf("no hypervisor available")
	}

	// Apply default security settings
	applySecurityDefaults(&cfg)

	return m.primary.CreateVM(ctx, cfg)
}

// applySecurityDefaults ensures minimum security configuration.
func applySecurityDefaults(cfg *VMConfig) {
	// Always enable disk encryption for tenant workloads
	if !cfg.Security.DiskEncryption {
		cfg.Security.DiskEncryption = true
	}

	// Enable SecureBoot by default
	if !cfg.Security.SecureBootEnabled {
		cfg.Security.SecureBootEnabled = true
	}

	// Disable VNC by default
	cfg.Security.VNCDisabled = true
}

package compute

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// HyperVHypervisor manages virtual machines using Microsoft Hyper-V on Windows.
type HyperVHypervisor struct {
	mu      sync.RWMutex
	vms     map[string]*hypervVM
	dataDir string // directory for VHD files
}

type hypervVM struct {
	config VMConfig
	state  VMState
}

// NewHyperVHypervisor creates a new Hyper-V hypervisor backend.
func NewHyperVHypervisor() *HyperVHypervisor {
	return &HyperVHypervisor{
		vms:     make(map[string]*hypervVM),
		dataDir: `C:\ProgramData\SoHoLINK\vms\hyperv`,
	}
}

// Name returns the hypervisor type identifier.
func (h *HyperVHypervisor) Name() string {
	return "hyperv"
}

// Available returns true if Hyper-V is supported on this platform.
func (h *HyperVHypervisor) Available() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	// Check if Hyper-V PowerShell module is available
	_, err := exec.LookPath("powershell.exe")
	return err == nil
}

// runPowershell executes a PowerShell script and returns its stdout output.
func (h *HyperVHypervisor) runPowershell(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("powershell error: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// vhdPath returns the full path to a VM's virtual hard disk file.
func (h *HyperVHypervisor) vhdPath(vmID string) string {
	return filepath.Join(h.dataDir, vmID+".vhdx")
}

// CreateVM provisions a new VM using Hyper-V.
func (h *HyperVHypervisor) CreateVM(ctx context.Context, cfg VMConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.vms[cfg.VMID]; exists {
		return fmt.Errorf("VM %s already exists", cfg.VMID)
	}

	vm := &hypervVM{
		config: cfg,
		state: VMState{
			VMID:   cfg.VMID,
			Status: "creating",
		},
	}
	h.vms[cfg.VMID] = vm

	if h.Available() {
		// Real Hyper-V provisioning via PowerShell
		diskPath := h.vhdPath(cfg.VMID)

		// Ensure data directory exists
		mkdirScript := fmt.Sprintf(`New-Item -ItemType Directory -Force -Path '%s' | Out-Null`, h.dataDir)
		if _, err := h.runPowershell(ctx, mkdirScript); err != nil {
			vm.state.Status = "failed"
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Create the VM
		createScript := fmt.Sprintf(`New-VM -Name '%s' -MemoryStartupBytes %dMB -Generation 2`,
			cfg.VMID, cfg.MemoryMB)
		if _, err := h.runPowershell(ctx, createScript); err != nil {
			vm.state.Status = "failed"
			return fmt.Errorf("failed to create VM: %w", err)
		}

		// Set CPU count
		cpuScript := fmt.Sprintf(`Set-VMProcessor -VMName '%s' -Count %d`,
			cfg.VMID, cfg.CPUCores)
		if _, err := h.runPowershell(ctx, cpuScript); err != nil {
			vm.state.Status = "failed"
			return fmt.Errorf("failed to set VM processor: %w", err)
		}

		// Create virtual hard disk
		vhdScript := fmt.Sprintf(`New-VHD -Path '%s' -SizeBytes %dGB -Dynamic`,
			diskPath, cfg.DiskGB)
		if _, err := h.runPowershell(ctx, vhdScript); err != nil {
			vm.state.Status = "failed"
			return fmt.Errorf("failed to create VHD: %w", err)
		}

		// Attach the disk to the VM
		attachScript := fmt.Sprintf(`Add-VMHardDiskDrive -VMName '%s' -Path '%s'`,
			cfg.VMID, diskPath)
		if _, err := h.runPowershell(ctx, attachScript); err != nil {
			vm.state.Status = "failed"
			return fmt.Errorf("failed to attach VHD: %w", err)
		}

		// Enable Secure Boot if requested
		if cfg.Security.SecureBootEnabled {
			secbootScript := fmt.Sprintf(`Set-VMFirmware -VMName '%s' -EnableSecureBoot On`,
				cfg.VMID)
			if _, err := h.runPowershell(ctx, secbootScript); err != nil {
				vm.state.Status = "failed"
				return fmt.Errorf("failed to enable Secure Boot: %w", err)
			}
		}

		// Enable TPM if requested
		if cfg.Security.TPMEnabled {
			tpmScript := fmt.Sprintf(`Enable-VMTPM -VMName '%s'`, cfg.VMID)
			if _, err := h.runPowershell(ctx, tpmScript); err != nil {
				vm.state.Status = "failed"
				return fmt.Errorf("failed to enable TPM: %w", err)
			}
		}

		// Start the VM
		startScript := fmt.Sprintf(`Start-VM -Name '%s'`, cfg.VMID)
		if _, err := h.runPowershell(ctx, startScript); err != nil {
			vm.state.Status = "failed"
			return fmt.Errorf("failed to start VM: %w", err)
		}

		vm.state.Status = "running"
		vm.state.IPAddress = cfg.Network.IPAddress
		log.Printf("[hyperv] VM %s created and started (cpu=%d, mem=%dMB, disk=%dGB, tpm=%v, secboot=%v)",
			cfg.VMID, cfg.CPUCores, cfg.MemoryMB, cfg.DiskGB,
			cfg.Security.TPMEnabled, cfg.Security.SecureBootEnabled)
	} else {
		// Simulation fallback for non-Windows or no PowerShell
		go func() {
			time.Sleep(200 * time.Millisecond)
			h.mu.Lock()
			if v, ok := h.vms[cfg.VMID]; ok {
				v.state.Status = "running"
				v.state.IPAddress = cfg.Network.IPAddress
			}
			h.mu.Unlock()
			log.Printf("[hyperv] VM %s created [simulated] (cpu=%d, mem=%dMB, tpm=%v, secboot=%v)",
				cfg.VMID, cfg.CPUCores, cfg.MemoryMB,
				cfg.Security.TPMEnabled, cfg.Security.SecureBootEnabled)
		}()
	}

	return nil
}

// StartVM boots a stopped Hyper-V VM.
func (h *HyperVHypervisor) StartVM(ctx context.Context, vmID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	vm, ok := h.vms[vmID]
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}
	if vm.state.Status == "running" {
		return fmt.Errorf("VM %s is already running", vmID)
	}

	if h.Available() {
		script := fmt.Sprintf(`Start-VM -Name '%s'`, vmID)
		if _, err := h.runPowershell(ctx, script); err != nil {
			return fmt.Errorf("failed to start VM %s: %w", vmID, err)
		}
	}

	vm.state.Status = "running"
	log.Printf("[hyperv] VM %s started", vmID)
	return nil
}

// StopVM performs a graceful shutdown of a Hyper-V VM.
func (h *HyperVHypervisor) StopVM(ctx context.Context, vmID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	vm, ok := h.vms[vmID]
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if h.Available() {
		script := fmt.Sprintf(`Stop-VM -Name '%s' -TurnOff:$false`, vmID)
		if _, err := h.runPowershell(ctx, script); err != nil {
			return fmt.Errorf("failed to stop VM %s: %w", vmID, err)
		}
	}

	vm.state.Status = "stopped"
	log.Printf("[hyperv] VM %s stopped", vmID)
	return nil
}

// DestroyVM forcefully removes a Hyper-V VM and its resources.
func (h *HyperVHypervisor) DestroyVM(ctx context.Context, vmID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.vms[vmID]; !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if h.Available() {
		diskPath := h.vhdPath(vmID)

		// Force stop the VM
		stopScript := fmt.Sprintf(`Stop-VM -Name '%s' -Force`, vmID)
		if _, err := h.runPowershell(ctx, stopScript); err != nil {
			log.Printf("[hyperv] warning: failed to stop VM %s during destroy: %v", vmID, err)
		}

		// Remove the VM
		removeScript := fmt.Sprintf(`Remove-VM -Name '%s' -Force`, vmID)
		if _, err := h.runPowershell(ctx, removeScript); err != nil {
			return fmt.Errorf("failed to remove VM %s: %w", vmID, err)
		}

		// Remove the VHD file
		removeVHDScript := fmt.Sprintf(`Remove-Item -Path '%s' -Force -ErrorAction SilentlyContinue`, diskPath)
		if _, err := h.runPowershell(ctx, removeVHDScript); err != nil {
			log.Printf("[hyperv] warning: failed to remove VHD for VM %s: %v", vmID, err)
		}
	}

	delete(h.vms, vmID)
	log.Printf("[hyperv] VM %s destroyed", vmID)
	return nil
}

// GetState returns the current state of a Hyper-V VM.
func (h *HyperVHypervisor) GetState(ctx context.Context, vmID string) (*VMState, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	vm, ok := h.vms[vmID]
	if !ok {
		return nil, fmt.Errorf("VM %s not found", vmID)
	}

	state := vm.state
	return &state, nil
}

// ListVMs returns all managed Hyper-V VMs.
func (h *HyperVHypervisor) ListVMs(ctx context.Context) ([]VMState, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	states := make([]VMState, 0, len(h.vms))
	for _, vm := range h.vms {
		states = append(states, vm.state)
	}
	return states, nil
}

// Snapshot creates a checkpoint (snapshot) of a VM.
func (h *HyperVHypervisor) Snapshot(ctx context.Context, vmID, snapshotName string) error {
	h.mu.RLock()
	_, ok := h.vms[vmID]
	h.mu.RUnlock()
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if h.Available() {
		script := fmt.Sprintf(`Checkpoint-VM -Name '%s' -SnapshotName '%s'`, vmID, snapshotName)
		if _, err := h.runPowershell(ctx, script); err != nil {
			return fmt.Errorf("failed to create snapshot for VM %s: %w", vmID, err)
		}
	}

	log.Printf("[hyperv] checkpoint %s created for VM %s", snapshotName, vmID)
	return nil
}

// Restore restores a VM from a checkpoint.
func (h *HyperVHypervisor) Restore(ctx context.Context, vmID, snapshotName string) error {
	h.mu.RLock()
	_, ok := h.vms[vmID]
	h.mu.RUnlock()
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if h.Available() {
		script := fmt.Sprintf(`Restore-VMSnapshot -VMName '%s' -Name '%s' -Confirm:$false`, vmID, snapshotName)
		if _, err := h.runPowershell(ctx, script); err != nil {
			return fmt.Errorf("failed to restore VM %s from snapshot %s: %w", vmID, snapshotName, err)
		}
	}

	log.Printf("[hyperv] VM %s restored from checkpoint %s", vmID, snapshotName)
	return nil
}

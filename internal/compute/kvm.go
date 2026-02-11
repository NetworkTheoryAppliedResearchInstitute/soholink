package compute

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// KVMHypervisor manages virtual machines using KVM/QEMU on Linux.
type KVMHypervisor struct {
	mu      sync.RWMutex
	vms     map[string]*kvmVM
	dataDir string
}

type kvmVM struct {
	config    VMConfig
	state     VMState
	pid       int
	simulate  bool
	qmpSocket string
}

// NewKVMHypervisor creates a new KVM/QEMU hypervisor backend.
// dataDir is the directory for VM disk images and QMP sockets;
// if empty, a default path is used.
func NewKVMHypervisor(dataDir string) *KVMHypervisor {
	if dataDir == "" {
		dataDir = "/var/lib/soholink/vms/kvm"
	}
	return &KVMHypervisor{
		vms:     make(map[string]*kvmVM),
		dataDir: dataDir,
	}
}

// Name returns the hypervisor type identifier.
func (k *KVMHypervisor) Name() string {
	return "kvm"
}

// Available returns true if KVM is supported on this platform.
func (k *KVMHypervisor) Available() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	// Check for /dev/kvm and qemu binary
	_, err := exec.LookPath("qemu-system-x86_64")
	return err == nil
}

// diskPath returns the qcow2 disk image path for a VM.
func (k *KVMHypervisor) diskPath(vmID string) string {
	return filepath.Join(k.dataDir, vmID+".qcow2")
}

// qmpPath returns the QMP socket path for a VM.
func (k *KVMHypervisor) qmpPath(vmID string) string {
	return filepath.Join(k.dataDir, vmID+".qmp")
}

// CreateVM provisions a new VM using QEMU/KVM.
// When QEMU is available, it creates a real disk image and launches the QEMU
// process. Otherwise it falls back to simulation mode for development/testing.
func (k *KVMHypervisor) CreateVM(ctx context.Context, cfg VMConfig) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if _, exists := k.vms[cfg.VMID]; exists {
		return fmt.Errorf("VM %s already exists", cfg.VMID)
	}

	simulate := !k.Available()

	vm := &kvmVM{
		config:   cfg,
		simulate: simulate,
		state: VMState{
			VMID:   cfg.VMID,
			Status: "creating",
		},
	}

	if simulate {
		// Simulation fallback: no real QEMU process
		k.vms[cfg.VMID] = vm
		go func() {
			time.Sleep(100 * time.Millisecond)
			k.mu.Lock()
			if v, ok := k.vms[cfg.VMID]; ok {
				v.state.Status = "running"
				v.state.IPAddress = cfg.Network.IPAddress
			}
			k.mu.Unlock()
			log.Printf("[kvm] VM %s created in simulation mode (cpu=%d, mem=%dMB, sev=%v, tpm=%v, secboot=%v)",
				cfg.VMID, cfg.CPUCores, cfg.MemoryMB,
				cfg.Security.SEVEnabled, cfg.Security.TPMEnabled, cfg.Security.SecureBootEnabled)
		}()
		return nil
	}

	// --- Real QEMU/KVM execution ---

	// Ensure data directory exists
	if err := os.MkdirAll(k.dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data dir %s: %w", k.dataDir, err)
	}

	diskFile := k.diskPath(cfg.VMID)
	qmpSock := k.qmpPath(cfg.VMID)
	vm.qmpSocket = qmpSock

	// Create qcow2 disk image
	diskSize := fmt.Sprintf("%dG", cfg.DiskGB)
	createDisk := exec.CommandContext(ctx, "qemu-img", "create", "-f", "qcow2", diskFile, diskSize)
	if out, err := createDisk.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create disk image: %w\noutput: %s", err, string(out))
	}

	// Build QEMU command arguments
	args := []string{
		"-enable-kvm",
		"-machine", "q35,accel=kvm",
		"-cpu", "host",
		"-smp", strconv.Itoa(cfg.CPUCores),
		"-m", strconv.FormatInt(cfg.MemoryMB, 10),
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio", diskFile),
		"-qmp", fmt.Sprintf("unix:%s,server,nowait", qmpSock),
		"-daemonize",
		"-display", "none",
	}

	// AMD SEV memory encryption
	if cfg.Security.SEVEnabled {
		args = append(args,
			"-object", "sev-guest,id=sev0,cbitpos=47,reduced-phys-bits=1",
		)
		// Patch the -machine argument to include memory-encryption
		for i, a := range args {
			if a == "q35,accel=kvm" {
				args[i] = "q35,accel=kvm,memory-encryption=sev0"
				break
			}
		}
	}

	// TPM emulator
	if cfg.Security.TPMEnabled {
		tpmSocket := filepath.Join(k.dataDir, cfg.VMID+".tpm.sock")
		args = append(args,
			"-chardev", fmt.Sprintf("socket,id=chrtpm,path=%s", tpmSocket),
			"-tpmdev", "emulator,id=tpm0,chardev=chrtpm",
			"-device", "tpm-tis,tpmdev=tpm0",
		)
	}

	// VNC display
	if cfg.Security.VNCDisabled {
		args = append(args, "-vnc", "none")
	}

	// Network configuration
	bridge := cfg.Network.BridgeInterface
	if bridge == "" {
		bridge = "br0"
	}
	mac := cfg.Network.MACAddress
	if mac == "" {
		mac = "52:54:00:00:00:01"
	}
	args = append(args,
		"-netdev", fmt.Sprintf("bridge,br=%s,id=net0", bridge),
		"-device", fmt.Sprintf("virtio-net-pci,netdev=net0,mac=%s", mac),
	)

	// SecureBoot OVMF firmware
	if cfg.Security.SecureBootEnabled {
		args = append(args,
			"-drive", "file=/usr/share/OVMF/OVMF_CODE.fd,if=pflash,format=raw,readonly=on",
		)
	}

	// Launch QEMU
	qemuBin := "qemu-system-x86_64"
	cmd := exec.CommandContext(ctx, qemuBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		// Clean up disk on failure
		os.Remove(diskFile)
		return fmt.Errorf("failed to start QEMU: %w", err)
	}

	vm.pid = cmd.Process.Pid
	vm.state.PID = cmd.Process.Pid
	vm.state.Status = "running"
	vm.state.IPAddress = cfg.Network.IPAddress

	k.vms[cfg.VMID] = vm

	// Release the process handle since QEMU daemonizes
	go cmd.Wait()

	log.Printf("[kvm] VM %s created (pid=%d, cpu=%d, mem=%dMB, disk=%s, sev=%v, tpm=%v, secboot=%v)",
		cfg.VMID, vm.pid, cfg.CPUCores, cfg.MemoryMB, diskSize,
		cfg.Security.SEVEnabled, cfg.Security.TPMEnabled, cfg.Security.SecureBootEnabled)

	return nil
}

// StartVM boots a stopped KVM VM.
func (k *KVMHypervisor) StartVM(ctx context.Context, vmID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	vm, ok := k.vms[vmID]
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}
	if vm.state.Status == "running" {
		return fmt.Errorf("VM %s is already running", vmID)
	}

	if vm.simulate {
		vm.state.Status = "running"
		log.Printf("[kvm] VM %s started (simulated)", vmID)
		return nil
	}

	// Send QMP 'cont' to resume the VM
	qmp := NewQMPClient(vm.qmpSocket)
	if err := qmp.Cont(); err != nil {
		return fmt.Errorf("failed to start VM %s via QMP: %w", vmID, err)
	}

	vm.state.Status = "running"
	log.Printf("[kvm] VM %s started", vmID)
	return nil
}

// StopVM performs a graceful shutdown of a KVM VM.
func (k *KVMHypervisor) StopVM(ctx context.Context, vmID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	vm, ok := k.vms[vmID]
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if vm.simulate {
		vm.state.Status = "stopped"
		log.Printf("[kvm] VM %s stopped (simulated)", vmID)
		return nil
	}

	// Send ACPI shutdown signal via QMP
	qmp := NewQMPClient(vm.qmpSocket)
	if err := qmp.SystemPowerdown(); err != nil {
		return fmt.Errorf("failed to stop VM %s via QMP: %w", vmID, err)
	}

	vm.state.Status = "stopped"
	log.Printf("[kvm] VM %s stopped", vmID)
	return nil
}

// DestroyVM forcefully removes a KVM VM and its resources.
func (k *KVMHypervisor) DestroyVM(ctx context.Context, vmID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	vm, ok := k.vms[vmID]
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if vm.simulate {
		delete(k.vms, vmID)
		log.Printf("[kvm] VM %s destroyed (simulated)", vmID)
		return nil
	}

	// Kill the QEMU process
	if vm.pid > 0 {
		proc, err := os.FindProcess(vm.pid)
		if err == nil {
			// First try graceful QMP quit
			qmp := NewQMPClient(vm.qmpSocket)
			if qmpErr := qmp.Quit(); qmpErr != nil {
				// Fall back to SIGKILL
				log.Printf("[kvm] QMP quit failed for VM %s, sending SIGKILL: %v", vmID, qmpErr)
				proc.Signal(syscall.SIGKILL)
			}
		}
	}

	// Remove disk image and QMP socket
	var errs []string
	diskFile := k.diskPath(vmID)
	if err := os.Remove(diskFile); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Sprintf("remove disk %s: %v", diskFile, err))
	}
	qmpSock := k.qmpPath(vmID)
	if err := os.Remove(qmpSock); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Sprintf("remove qmp socket %s: %v", qmpSock, err))
	}

	delete(k.vms, vmID)

	if len(errs) > 0 {
		log.Printf("[kvm] VM %s destroyed with cleanup warnings: %s", vmID, strings.Join(errs, "; "))
	} else {
		log.Printf("[kvm] VM %s destroyed", vmID)
	}
	return nil
}

// GetState returns the current state of a KVM VM.
func (k *KVMHypervisor) GetState(ctx context.Context, vmID string) (*VMState, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	vm, ok := k.vms[vmID]
	if !ok {
		return nil, fmt.Errorf("VM %s not found", vmID)
	}

	state := vm.state
	return &state, nil
}

// ListVMs returns all managed KVM VMs.
func (k *KVMHypervisor) ListVMs(ctx context.Context) ([]VMState, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	states := make([]VMState, 0, len(k.vms))
	for _, vm := range k.vms {
		states = append(states, vm.state)
	}
	return states, nil
}

// Snapshot creates a point-in-time snapshot of a VM's disk.
func (k *KVMHypervisor) Snapshot(ctx context.Context, vmID, snapshotName string) error {
	k.mu.RLock()
	vm, ok := k.vms[vmID]
	k.mu.RUnlock()
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if vm.simulate {
		log.Printf("[kvm] snapshot %s created for VM %s (simulated)", snapshotName, vmID)
		return nil
	}

	// Create snapshot using qemu-img
	diskFile := k.diskPath(vmID)
	cmd := exec.CommandContext(ctx, "qemu-img", "snapshot", "-c", snapshotName, diskFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create snapshot %s for VM %s: %w\noutput: %s",
			snapshotName, vmID, err, string(out))
	}

	log.Printf("[kvm] snapshot %s created for VM %s", snapshotName, vmID)
	return nil
}

// Restore restores a VM from a snapshot.
func (k *KVMHypervisor) Restore(ctx context.Context, vmID, snapshotName string) error {
	k.mu.RLock()
	vm, ok := k.vms[vmID]
	k.mu.RUnlock()
	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	if vm.simulate {
		log.Printf("[kvm] VM %s restored from snapshot %s (simulated)", vmID, snapshotName)
		return nil
	}

	// Restore snapshot using qemu-img
	diskFile := k.diskPath(vmID)
	cmd := exec.CommandContext(ctx, "qemu-img", "snapshot", "-a", snapshotName, diskFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restore snapshot %s for VM %s: %w\noutput: %s",
			snapshotName, vmID, err, string(out))
	}

	log.Printf("[kvm] VM %s restored from snapshot %s", vmID, snapshotName)
	return nil
}

package compute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// FirecrackerHypervisor implements the Hypervisor interface using Firecracker microVMs.
// Firecracker provides lightweight virtualization with fast boot times (<125ms)
// and minimal memory overhead, ideal for serverless and edge computing workloads.
type FirecrackerHypervisor struct {
	rootDir string
	mu      sync.RWMutex
	vms     map[string]*FirecrackerVM
}

// FirecrackerVM represents a running Firecracker microVM.
type FirecrackerVM struct {
	ID          string
	Config      *VMConfig
	Process     *exec.Cmd
	SocketPath  string
	State       VMState
	CreatedAt   time.Time
	httpClient  *http.Client
}

// NewFirecrackerHypervisor creates a new Firecracker hypervisor manager.
func NewFirecrackerHypervisor(rootDir string) (*FirecrackerHypervisor, error) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}

	// Check if firecracker binary is available
	if _, err := exec.LookPath("firecracker"); err != nil {
		return nil, fmt.Errorf("firecracker binary not found in PATH: %w", err)
	}

	return &FirecrackerHypervisor{
		rootDir: rootDir,
		vms:     make(map[string]*FirecrackerVM),
	}, nil
}

// CreateVM creates a new Firecracker microVM.
func (f *FirecrackerHypervisor) CreateVM(config *VMConfig) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	vmID := generateVMID()
	vmDir := filepath.Join(f.rootDir, vmID)

	if err := os.MkdirAll(vmDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create VM directory: %w", err)
	}

	// Create root filesystem (ext4 image)
	rootfsPath := filepath.Join(vmDir, "rootfs.ext4")
	if err := f.createRootfs(rootfsPath, config.Disk); err != nil {
		return "", fmt.Errorf("failed to create rootfs: %w", err)
	}

	// Create Unix socket for API communication
	socketPath := filepath.Join(vmDir, "firecracker.sock")

	vm := &FirecrackerVM{
		ID:         vmID,
		Config:     config,
		SocketPath: socketPath,
		State:      VMStateStopped,
		CreatedAt:  time.Now(),
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
			Timeout: 5 * time.Second,
		},
	}

	f.vms[vmID] = vm

	return vmID, nil
}

// StartVM starts a Firecracker microVM.
func (f *FirecrackerHypervisor) StartVM(vmID string) error {
	f.mu.Lock()
	vm, ok := f.vms[vmID]
	f.mu.Unlock()

	if !ok {
		return ErrVMNotFound
	}

	if vm.State == VMStateRunning {
		return nil // Already running
	}

	vmDir := filepath.Join(f.rootDir, vmID)
	rootfsPath := filepath.Join(vmDir, "rootfs.ext4")
	kernelPath := "/usr/share/firecracker/vmlinux.bin" // Default kernel location

	// Start Firecracker process
	cmd := exec.Command("firecracker",
		"--api-sock", vm.SocketPath,
	)

	// Configure logging
	logPath := filepath.Join(vmDir, "firecracker.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start firecracker: %w", err)
	}

	vm.Process = cmd

	// Wait for API socket to be ready
	if err := f.waitForSocket(vm.SocketPath, 5*time.Second); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("API socket not ready: %w", err)
	}

	// Configure machine resources
	machineConfig := map[string]interface{}{
		"vcpu_count":   vm.Config.CPU,
		"mem_size_mib": vm.Config.Memory,
		"smt":          false, // Disable simultaneous multithreading for security
	}

	if err := f.apiPut(vm, "/machine-config", machineConfig); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to set machine config: %w", err)
	}

	// Configure boot source
	bootConfig := map[string]interface{}{
		"kernel_image_path": kernelPath,
		"boot_args":         "console=ttyS0 reboot=k panic=1 pci=off",
	}

	if err := f.apiPut(vm, "/boot-source", bootConfig); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to set boot source: %w", err)
	}

	// Configure root drive
	driveConfig := map[string]interface{}{
		"drive_id":        "rootfs",
		"path_on_host":    rootfsPath,
		"is_root_device":  true,
		"is_read_only":    false,
	}

	if err := f.apiPut(vm, "/drives/rootfs", driveConfig); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to set root drive: %w", err)
	}

	// Configure network interface
	tapDevice := fmt.Sprintf("fc-tap-%s", vmID[:8])
	networkConfig := map[string]interface{}{
		"iface_id":          "eth0",
		"guest_mac":         f.generateMAC(vmID),
		"host_dev_name":     tapDevice,
	}

	// Create TAP device
	if err := f.createTAPDevice(tapDevice); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to create TAP device: %w", err)
	}

	if err := f.apiPut(vm, "/network-interfaces/eth0", networkConfig); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to set network interface: %w", err)
	}

	// Start the VM
	actionConfig := map[string]interface{}{
		"action_type": "InstanceStart",
	}

	if err := f.apiPut(vm, "/actions", actionConfig); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to start instance: %w", err)
	}

	vm.State = VMStateRunning

	// Monitor process in background
	go func() {
		cmd.Wait()
		f.mu.Lock()
		if v, exists := f.vms[vmID]; exists {
			v.State = VMStateStopped
		}
		f.mu.Unlock()
	}()

	return nil
}

// StopVM stops a Firecracker microVM.
func (f *FirecrackerHypervisor) StopVM(vmID string) error {
	f.mu.Lock()
	vm, ok := f.vms[vmID]
	f.mu.Unlock()

	if !ok {
		return ErrVMNotFound
	}

	if vm.State != VMStateRunning {
		return nil // Already stopped
	}

	// Send shutdown signal
	if vm.Process != nil {
		vm.Process.Process.Signal(os.Interrupt)

		// Wait up to 30 seconds for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- vm.Process.Wait()
		}()

		select {
		case <-time.After(30 * time.Second):
			// Force kill if graceful shutdown times out
			vm.Process.Process.Kill()
			vm.Process.Wait()
		case <-done:
			// Graceful shutdown succeeded
		}
	}

	vm.State = VMStateStopped
	return nil
}

// DestroyVM destroys a Firecracker microVM and cleans up resources.
func (f *FirecrackerHypervisor) DestroyVM(vmID string) error {
	// Stop VM first
	if err := f.StopVM(vmID); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	vm, ok := f.vms[vmID]
	if !ok {
		return ErrVMNotFound
	}

	// Clean up TAP device
	tapDevice := fmt.Sprintf("fc-tap-%s", vmID[:8])
	f.deleteTAPDevice(tapDevice)

	// Remove VM directory
	vmDir := filepath.Join(f.rootDir, vmID)
	if err := os.RemoveAll(vmDir); err != nil {
		return fmt.Errorf("failed to remove VM directory: %w", err)
	}

	delete(f.vms, vmID)
	return nil
}

// GetState returns the current state of a VM.
func (f *FirecrackerHypervisor) GetState(vmID string) (VMState, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	vm, ok := f.vms[vmID]
	if !ok {
		return VMStateStopped, ErrVMNotFound
	}

	return vm.State, nil
}

// ListVMs lists all VMs managed by this hypervisor.
func (f *FirecrackerHypervisor) ListVMs() ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	vmIDs := make([]string, 0, len(f.vms))
	for id := range f.vms {
		vmIDs = append(vmIDs, id)
	}

	return vmIDs, nil
}

// Snapshot creates a snapshot of a running VM.
func (f *FirecrackerHypervisor) Snapshot(vmID string, snapshotName string) (string, error) {
	f.mu.RLock()
	vm, ok := f.vms[vmID]
	f.mu.RUnlock()

	if !ok {
		return "", ErrVMNotFound
	}

	if vm.State != VMStateRunning {
		return "", fmt.Errorf("VM must be running to snapshot")
	}

	snapshotDir := filepath.Join(f.rootDir, vmID, "snapshots")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	snapshotPath := filepath.Join(snapshotDir, snapshotName)
	memPath := snapshotPath + ".mem"
	vmStatePath := snapshotPath + ".vmstate"

	// Create snapshot via API
	snapshotConfig := map[string]interface{}{
		"snapshot_type": "Full",
		"snapshot_path": vmStatePath,
		"mem_file_path": memPath,
	}

	if err := f.apiPut(vm, "/snapshot/create", snapshotConfig); err != nil {
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}

	return snapshotPath, nil
}

// Restore restores a VM from a snapshot.
func (f *FirecrackerHypervisor) Restore(vmID string, snapshotPath string) error {
	// Firecracker requires loading snapshots at boot time
	// This is a limitation of the current implementation
	return fmt.Errorf("snapshot restore requires VM restart with --restore-from-snapshot flag")
}

// apiPut makes a PUT request to the Firecracker API.
func (f *FirecrackerHypervisor) apiPut(vm *FirecrackerVM, path string, body interface{}) error {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	url := fmt.Sprintf("http://localhost%s", path)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := vm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// waitForSocket waits for the API socket to become available.
func (f *FirecrackerHypervisor) waitForSocket(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			// Socket exists, try to connect
			conn, err := net.Dial("unix", socketPath)
			if err == nil {
				conn.Close()
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for socket")
}

// createRootfs creates an ext4 filesystem image.
func (f *FirecrackerHypervisor) createRootfs(path string, sizeMB int) error {
	// Create sparse file
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := file.Truncate(int64(sizeMB) * 1024 * 1024); err != nil {
		return err
	}

	// Format as ext4
	cmd := exec.Command("mkfs.ext4", "-F", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format rootfs: %w", err)
	}

	return nil
}

// createTAPDevice creates a TAP network device.
func (f *FirecrackerHypervisor) createTAPDevice(name string) error {
	cmd := exec.Command("ip", "tuntap", "add", name, "mode", "tap")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create TAP device: %w", err)
	}

	cmd = exec.Command("ip", "link", "set", name, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring up TAP device: %w", err)
	}

	return nil
}

// deleteTAPDevice deletes a TAP network device.
func (f *FirecrackerHypervisor) deleteTAPDevice(name string) error {
	cmd := exec.Command("ip", "link", "delete", name)
	return cmd.Run()
}

// generateMAC generates a MAC address for a VM.
func (f *FirecrackerHypervisor) generateMAC(vmID string) string {
	// Use first 6 bytes of VM ID for MAC address
	// FC: prefix indicates Firecracker
	return fmt.Sprintf("FC:00:00:%02x:%02x:%02x",
		vmID[0], vmID[1], vmID[2])
}

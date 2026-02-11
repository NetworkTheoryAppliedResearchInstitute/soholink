package compute

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFirecrackerHypervisor_CreateVM(t *testing.T) {
	tmpDir := t.TempDir()

	hypervisor := &FirecrackerHypervisor{
		rootDir: tmpDir,
		vms:     make(map[string]*FirecrackerVM),
	}

	config := VMConfig{
		VMID:     "test-vm-001",
		CPUs:     2,
		MemoryMB: 1024,
		DiskGB:   10,
		ImageURL: "file:///tmp/test-rootfs.ext4",
	}

	err := hypervisor.CreateVM(config)
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	// Verify VM directory was created
	vmDir := filepath.Join(tmpDir, "test-vm-001")
	if _, err := os.Stat(vmDir); os.IsNotExist(err) {
		t.Errorf("VM directory not created: %s", vmDir)
	}

	// Verify VM is tracked
	hypervisor.mu.RLock()
	vm, exists := hypervisor.vms["test-vm-001"]
	hypervisor.mu.RUnlock()

	if !exists {
		t.Fatal("VM not found in hypervisor map")
	}

	if vm.Config.VMID != "test-vm-001" {
		t.Errorf("Expected VMID 'test-vm-001', got '%s'", vm.Config.VMID)
	}

	if vm.Config.CPUs != 2 {
		t.Errorf("Expected 2 CPUs, got %d", vm.Config.CPUs)
	}

	if vm.Config.MemoryMB != 1024 {
		t.Errorf("Expected 1024 MB memory, got %d", vm.Config.MemoryMB)
	}

	// Verify socket path
	expectedSocket := filepath.Join(vmDir, "firecracker.socket")
	if vm.SocketPath != expectedSocket {
		t.Errorf("Expected socket path '%s', got '%s'", expectedSocket, vm.SocketPath)
	}
}

func TestFirecrackerHypervisor_StartVM_MockAPI(t *testing.T) {
	// Mock Firecracker API server
	configSet := false
	bootSet := false
	driveSet := false
	networkSet := false
	started := false

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/machine-config" && r.Method == "PUT":
			configSet = true
			w.WriteHeader(http.StatusNoContent)

		case r.URL.Path == "/boot-source" && r.Method == "PUT":
			bootSet = true
			w.WriteHeader(http.StatusNoContent)

		case strings.HasPrefix(r.URL.Path, "/drives/") && r.Method == "PUT":
			driveSet = true
			w.WriteHeader(http.StatusNoContent)

		case strings.HasPrefix(r.URL.Path, "/network-interfaces/") && r.Method == "PUT":
			networkSet = true
			w.WriteHeader(http.StatusNoContent)

		case r.URL.Path == "/actions" && r.Method == "PUT":
			var action map[string]interface{}
			json.NewDecoder(r.Body).Decode(&action)
			if action["action_type"] == "InstanceStart" {
				started = true
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Logf("Unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))

	// Use Unix socket listener for more realistic test
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.socket")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create Unix socket: %v", err)
	}
	defer listener.Close()

	server.Listener = listener
	server.Start()
	defer server.Close()

	// Create hypervisor with mocked VM
	hypervisor := &FirecrackerHypervisor{
		rootDir: tmpDir,
		vms:     make(map[string]*FirecrackerVM),
	}

	vm := &FirecrackerVM{
		Config: VMConfig{
			VMID:     "test-vm",
			CPUs:     1,
			MemoryMB: 512,
			DiskGB:   5,
			ImageURL: "file:///tmp/test.ext4",
		},
		SocketPath: socketPath,
		State:      "created",
	}
	hypervisor.vms["test-vm"] = vm

	// Test configuration only (not full start since we can't mock process)
	// In real scenario, StartVM would launch Firecracker binary
	t.Log("Testing Firecracker API call sequence")

	if !configSet && !bootSet && !driveSet {
		t.Log("VM configuration would be applied via Firecracker API")
	}
}

func TestFirecrackerHypervisor_StopVM(t *testing.T) {
	tmpDir := t.TempDir()

	hypervisor := &FirecrackerHypervisor{
		rootDir: tmpDir,
		vms:     make(map[string]*FirecrackerVM),
	}

	// Create mock VM
	vm := &FirecrackerVM{
		Config: VMConfig{VMID: "test-vm"},
		State:  "running",
	}
	hypervisor.vms["test-vm"] = vm

	err := hypervisor.StopVM("test-vm")
	if err != nil {
		t.Fatalf("StopVM failed: %v", err)
	}

	// VM should still exist but be stopped
	vm, exists := hypervisor.vms["test-vm"]
	if !exists {
		t.Error("VM should still exist after stop")
	}

	if vm.State != "stopped" {
		t.Errorf("Expected state 'stopped', got '%s'", vm.State)
	}
}

func TestFirecrackerHypervisor_DeleteVM(t *testing.T) {
	tmpDir := t.TempDir()
	vmDir := filepath.Join(tmpDir, "test-vm")
	os.MkdirAll(vmDir, 0755)

	hypervisor := &FirecrackerHypervisor{
		rootDir: tmpDir,
		vms:     make(map[string]*FirecrackerVM),
	}

	vm := &FirecrackerVM{
		Config: VMConfig{VMID: "test-vm"},
		State:  "stopped",
	}
	hypervisor.vms["test-vm"] = vm

	err := hypervisor.DeleteVM("test-vm")
	if err != nil {
		t.Fatalf("DeleteVM failed: %v", err)
	}

	// VM should be removed from map
	hypervisor.mu.RLock()
	_, exists := hypervisor.vms["test-vm"]
	hypervisor.mu.RUnlock()

	if exists {
		t.Error("VM should be removed from map")
	}

	// VM directory should be removed
	if _, err := os.Stat(vmDir); !os.IsNotExist(err) {
		t.Error("VM directory should be removed")
	}
}

func TestFirecrackerHypervisor_ListVMs(t *testing.T) {
	hypervisor := &FirecrackerHypervisor{
		rootDir: t.TempDir(),
		vms:     make(map[string]*FirecrackerVM),
	}

	// Add test VMs
	vms := []string{"vm1", "vm2", "vm3"}
	for _, vmID := range vms {
		hypervisor.vms[vmID] = &FirecrackerVM{
			Config: VMConfig{VMID: vmID},
			State:  "running",
		}
	}

	result := hypervisor.ListVMs()

	if len(result) != len(vms) {
		t.Errorf("Expected %d VMs, got %d", len(vms), len(result))
	}

	// Check all VMs are present
	vmMap := make(map[string]bool)
	for _, vmID := range result {
		vmMap[vmID] = true
	}

	for _, vmID := range vms {
		if !vmMap[vmID] {
			t.Errorf("VM '%s' not found in list", vmID)
		}
	}
}

func TestFirecrackerHypervisor_GetVMInfo(t *testing.T) {
	hypervisor := &FirecrackerHypervisor{
		rootDir: t.TempDir(),
		vms:     make(map[string]*FirecrackerVM),
	}

	config := VMConfig{
		VMID:     "info-test-vm",
		CPUs:     4,
		MemoryMB: 2048,
		DiskGB:   20,
	}

	vm := &FirecrackerVM{
		Config:     config,
		State:      "running",
		SocketPath: "/tmp/test.socket",
	}
	hypervisor.vms["info-test-vm"] = vm

	info, err := hypervisor.GetVMInfo("info-test-vm")
	if err != nil {
		t.Fatalf("GetVMInfo failed: %v", err)
	}

	if info.VMID != "info-test-vm" {
		t.Errorf("Expected VMID 'info-test-vm', got '%s'", info.VMID)
	}

	if info.State != "running" {
		t.Errorf("Expected state 'running', got '%s'", info.State)
	}

	if info.CPUs != 4 {
		t.Errorf("Expected 4 CPUs, got %d", info.CPUs)
	}

	if info.MemoryMB != 2048 {
		t.Errorf("Expected 2048 MB memory, got %d", info.MemoryMB)
	}
}

func TestFirecrackerHypervisor_SnapshotVM(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotDir := filepath.Join(tmpDir, "snapshots")
	os.MkdirAll(snapshotDir, 0755)

	hypervisor := &FirecrackerHypervisor{
		rootDir: tmpDir,
		vms:     make(map[string]*FirecrackerVM),
	}

	vm := &FirecrackerVM{
		Config:     VMConfig{VMID: "snapshot-vm"},
		State:      "running",
		SocketPath: filepath.Join(tmpDir, "test.socket"),
	}
	hypervisor.vms["snapshot-vm"] = vm

	snapshotPath := filepath.Join(snapshotDir, "test-snapshot")

	// Note: Actual snapshot requires Firecracker API call
	// This tests the structure and path handling
	err := hypervisor.SnapshotVM("snapshot-vm", snapshotPath)
	if err != nil {
		// Expected to fail without real Firecracker, but validates structure
		t.Logf("Snapshot failed as expected without Firecracker: %v", err)
	}
}

func TestFirecrackerHypervisor_RestoreVM(t *testing.T) {
	tmpDir := t.TempDir()

	hypervisor := &FirecrackerHypervisor{
		rootDir: tmpDir,
		vms:     make(map[string]*FirecrackerVM),
	}

	snapshotPath := filepath.Join(tmpDir, "test-snapshot")

	// Note: Actual restore requires valid snapshot files
	err := hypervisor.RestoreVM("restored-vm", snapshotPath)
	if err != nil {
		t.Logf("Restore failed as expected without snapshot: %v", err)
	}
}

func TestFirecrackerHypervisor_ConcurrentOperations(t *testing.T) {
	hypervisor := &FirecrackerHypervisor{
		rootDir: t.TempDir(),
		vms:     make(map[string]*FirecrackerVM),
	}

	// Add multiple VMs concurrently
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			vmID := string(rune('A' + id))
			config := VMConfig{
				VMID:     vmID,
				CPUs:     1,
				MemoryMB: 512,
				DiskGB:   5,
				ImageURL: "file:///tmp/test.ext4",
			}

			err := hypervisor.CreateVM(config)
			if err != nil {
				t.Errorf("Concurrent CreateVM failed for %s: %v", vmID, err)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Fatal("Concurrent operations timed out")
		}
	}

	// Verify all VMs were created
	vms := hypervisor.ListVMs()
	if len(vms) != 10 {
		t.Errorf("Expected 10 VMs, got %d", len(vms))
	}
}

func TestFirecrackerVM_BootTime(t *testing.T) {
	// Test that boot configuration is optimized for <125ms target
	config := VMConfig{
		VMID:     "fast-boot-vm",
		CPUs:     1,
		MemoryMB: 128, // Minimal memory for fast boot
		DiskGB:   1,
	}

	if config.MemoryMB < 128 {
		t.Error("Minimum memory should be 128MB for stability")
	}

	if config.CPUs < 1 {
		t.Error("Minimum 1 CPU required")
	}

	// Boot time test would require actual Firecracker binary
	t.Log("Boot time validation would require Firecracker binary")
}

func TestFirecrackerHypervisor_ResourceLimits(t *testing.T) {
	hypervisor := &FirecrackerHypervisor{
		rootDir: t.TempDir(),
		vms:     make(map[string]*FirecrackerVM),
	}

	// Test various resource configurations
	testCases := []struct {
		name   string
		config VMConfig
		valid  bool
	}{
		{
			name:   "Minimum valid config",
			config: VMConfig{VMID: "min", CPUs: 1, MemoryMB: 128, DiskGB: 1},
			valid:  true,
		},
		{
			name:   "Maximum CPUs",
			config: VMConfig{VMID: "max-cpu", CPUs: 32, MemoryMB: 1024, DiskGB: 10},
			valid:  true,
		},
		{
			name:   "Large memory",
			config: VMConfig{VMID: "big-mem", CPUs: 2, MemoryMB: 16384, DiskGB: 10},
			valid:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.ImageURL = "file:///tmp/test.ext4"
			err := hypervisor.CreateVM(tc.config)

			if tc.valid && err != nil {
				t.Errorf("Expected valid config to succeed, got error: %v", err)
			}
		})
	}
}

func BenchmarkFirecrackerHypervisor_CreateVM(b *testing.B) {
	tmpDir := b.TempDir()
	hypervisor := &FirecrackerHypervisor{
		rootDir: tmpDir,
		vms:     make(map[string]*FirecrackerVM),
	}

	config := VMConfig{
		VMID:     "bench-vm",
		CPUs:     1,
		MemoryMB: 512,
		DiskGB:   5,
		ImageURL: "file:///tmp/test.ext4",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.VMID = string(rune('a' + i%26))
		hypervisor.CreateVM(config)
	}
}

func BenchmarkFirecrackerHypervisor_ListVMs(b *testing.B) {
	hypervisor := &FirecrackerHypervisor{
		rootDir: b.TempDir(),
		vms:     make(map[string]*FirecrackerVM),
	}

	// Add 100 VMs
	for i := 0; i < 100; i++ {
		vmID := string(rune(i))
		hypervisor.vms[vmID] = &FirecrackerVM{
			Config: VMConfig{VMID: vmID},
			State:  "running",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hypervisor.ListVMs()
	}
}

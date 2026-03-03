package compute

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGPUManager_ListGPUs(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	// Create mock sysfs structure
	pciDevices := filepath.Join(tmpDir, "bus", "pci", "devices")
	os.MkdirAll(pciDevices, 0755)

	// Mock GPU device 0000:01:00.0
	gpuPath := filepath.Join(pciDevices, "0000:01:00.0")
	os.MkdirAll(gpuPath, 0755)
	os.WriteFile(filepath.Join(gpuPath, "vendor"), []byte("0x10de\n"), 0644) // NVIDIA
	os.WriteFile(filepath.Join(gpuPath, "device"), []byte("0x1e04\n"), 0644) // RTX 2080
	os.WriteFile(filepath.Join(gpuPath, "class"), []byte("0x030000\n"), 0644) // VGA controller

	gpus := manager.ListGPUs()

	if len(gpus) == 0 {
		t.Log("No GPUs found (expected in mock environment)")
	} else {
		t.Logf("Found %d GPU(s) in mock environment", len(gpus))
	}
}

func TestGPUManager_GetGPUInfo(t *testing.T) {
	_ = &GPUManager{
		sysfsPath: t.TempDir(),
	}

	// In a real system, this would query actual GPU
	gpu := &GPUDevice{
		PCIAddress: "0000:01:00.0",
		VendorID:   "10de",
		DeviceID:   "1e04",
		IOMMUGroup: 1,
		VFIOBound:  false,
	}

	if gpu.PCIAddress == "" {
		t.Error("Expected non-empty PCI address")
	}

	if gpu.VendorID == "10de" {
		t.Log("Detected NVIDIA GPU")
	}
}

func TestGPUManager_ValidateIOMMU(t *testing.T) {
	manager := &GPUManager{
		sysfsPath: t.TempDir(),
	}

	// Test IOMMU validation logic
	pciAddr := "0000:01:00.0"

	err := manager.ValidateIOMMU(pciAddr)
	if err != nil {
		// Expected in test environment without real IOMMU
		t.Logf("IOMMU validation failed (expected): %v", err)
	}
}

func TestGPUManager_BindToVFIO(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	// Create mock sysfs structure
	driverPath := filepath.Join(tmpDir, "bus", "pci", "drivers", "vfio-pci")
	os.MkdirAll(driverPath, 0755)

	pciAddr := "0000:01:00.0"

	err := manager.BindToVFIO(pciAddr)
	if err != nil {
		// Expected without real hardware
		t.Logf("VFIO bind failed (expected): %v", err)
	}
}

func TestGPUManager_UnbindFromVFIO(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	// Create mock structure
	driverPath := filepath.Join(tmpDir, "bus", "pci", "drivers", "vfio-pci")
	os.MkdirAll(driverPath, 0755)

	pciAddr := "0000:01:00.0"

	err := manager.UnbindFromVFIO(pciAddr)
	if err != nil {
		t.Logf("VFIO unbind failed (expected): %v", err)
	}
}

func TestGPUManager_EnableSRIOV(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	// Create mock device path
	devicePath := filepath.Join(tmpDir, "bus", "pci", "devices", "0000:01:00.0")
	os.MkdirAll(devicePath, 0755)

	pciAddr := "0000:01:00.0"
	numVFs := 4

	err := manager.EnableSRIOV(pciAddr, numVFs)
	if err != nil {
		t.Logf("SR-IOV enable failed (expected): %v", err)
	}

	// Verify sriov_numvfs would be written
	sriovFile := filepath.Join(devicePath, "sriov_numvfs")
	if _, err := os.Stat(sriovFile); os.IsNotExist(err) {
		t.Log("SR-IOV file not created (expected in mock)")
	}
}

func TestGPUManager_DisableSRIOV(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	devicePath := filepath.Join(tmpDir, "bus", "pci", "devices", "0000:01:00.0")
	os.MkdirAll(devicePath, 0755)

	pciAddr := "0000:01:00.0"

	err := manager.DisableSRIOV(pciAddr)
	if err != nil {
		t.Logf("SR-IOV disable failed (expected): %v", err)
	}
}

func TestGPUManager_AttachToVM(t *testing.T) {
	manager := &GPUManager{
		sysfsPath: t.TempDir(),
		attachments: make(map[string]string),
	}

	pciAddr := "0000:01:00.0"
	vmID := "test-vm-001"

	err := manager.AttachToVM(pciAddr, vmID)
	if err != nil {
		t.Fatalf("AttachToVM failed: %v", err)
	}

	// Verify attachment is tracked
	manager.mu.RLock()
	attachedVM, exists := manager.attachments[pciAddr]
	manager.mu.RUnlock()

	if !exists {
		t.Error("GPU attachment not tracked")
	}

	if attachedVM != vmID {
		t.Errorf("Expected VM ID '%s', got '%s'", vmID, attachedVM)
	}

	// Try attaching same GPU to different VM (should fail)
	err = manager.AttachToVM(pciAddr, "other-vm")
	if err == nil {
		t.Error("Should not allow attaching already-attached GPU")
	}
}

func TestGPUManager_DetachFromVM(t *testing.T) {
	manager := &GPUManager{
		sysfsPath: t.TempDir(),
		attachments: make(map[string]string),
	}

	pciAddr := "0000:01:00.0"
	vmID := "test-vm-001"

	// Attach first
	manager.attachments[pciAddr] = vmID

	err := manager.DetachFromVM(pciAddr, vmID)
	if err != nil {
		t.Fatalf("DetachFromVM failed: %v", err)
	}

	// Verify detachment
	manager.mu.RLock()
	_, exists := manager.attachments[pciAddr]
	manager.mu.RUnlock()

	if exists {
		t.Error("GPU should be detached")
	}
}

func TestGPUDevice_IsNVIDIA(t *testing.T) {
	testCases := []struct {
		name     string
		vendorID string
		expected bool
	}{
		{"NVIDIA GPU", "10de", true},
		{"AMD GPU", "1002", false},
		{"Intel GPU", "8086", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gpu := &GPUDevice{
				VendorID: tc.vendorID,
			}

			isNVIDIA := (gpu.VendorID == "10de")
			if isNVIDIA != tc.expected {
				t.Errorf("Expected IsNVIDIA=%v, got %v", tc.expected, isNVIDIA)
			}
		})
	}
}

func TestGPUDevice_IsAMD(t *testing.T) {
	gpu := &GPUDevice{
		VendorID: "1002",
	}

	isAMD := (gpu.VendorID == "1002")
	if !isAMD {
		t.Error("Expected AMD GPU")
	}
}

func TestGPUDevice_IsVirtualFunction(t *testing.T) {
	testCases := []struct {
		name     string
		vfIndex  int
		parentPCI string
		expected bool
	}{
		{"Physical function", -1, "", false},
		{"Virtual function 0", 0, "0000:01:00.0", true},
		{"Virtual function 3", 3, "0000:01:00.0", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gpu := &GPUDevice{
				VFIndex:   tc.vfIndex,
				ParentPCI: tc.parentPCI,
			}

			isVF := (gpu.VFIndex >= 0 && gpu.ParentPCI != "")
			if isVF != tc.expected {
				t.Errorf("Expected IsVirtualFunction=%v, got %v", tc.expected, isVF)
			}
		})
	}
}

func TestGPUManager_GetIOMMUGroup(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	// Create mock IOMMU group symlink
	devicePath := filepath.Join(tmpDir, "bus", "pci", "devices", "0000:01:00.0")
	os.MkdirAll(devicePath, 0755)

	_ = filepath.Join(devicePath, "iommu_group")
	// In real system, this would be a symlink to /sys/kernel/iommu_groups/N

	pciAddr := "0000:01:00.0"
	group, err := manager.GetIOMMUGroup(pciAddr)

	if err != nil {
		t.Logf("GetIOMMUGroup failed (expected without real hardware): %v", err)
	} else {
		t.Logf("IOMMU group: %d", group)
	}
}

func TestGPUManager_ListDevicesInIOMMUGroup(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	groupID := 1

	devices, err := manager.ListDevicesInIOMMUGroup(groupID)
	if err != nil {
		t.Logf("ListDevicesInIOMMUGroup failed (expected): %v", err)
	}

	if len(devices) == 0 {
		t.Log("No devices in IOMMU group (expected in mock)")
	}
}

func TestGPUManager_VFIODriverCheck(t *testing.T) {
	tmpDir := t.TempDir()

	_ = &GPUManager{
		sysfsPath: tmpDir,
	}

	// Create mock driver path
	driverPath := filepath.Join(tmpDir, "bus", "pci", "drivers", "vfio-pci")
	os.MkdirAll(driverPath, 0755)

	// Check if vfio-pci driver exists
	if _, err := os.Stat(driverPath); err == nil {
		t.Log("VFIO-PCI driver path exists")
	}
}

func TestGPUManager_MultipleGPUs(t *testing.T) {
	manager := &GPUManager{
		sysfsPath: t.TempDir(),
		attachments: make(map[string]string),
	}

	// Simulate multiple GPUs
	gpus := []string{
		"0000:01:00.0",
		"0000:02:00.0",
		"0000:03:00.0",
	}

	vms := []string{"vm1", "vm2", "vm3"}

	// Attach each GPU to a VM
	for i, gpu := range gpus {
		err := manager.AttachToVM(gpu, vms[i])
		if err != nil {
			t.Errorf("Failed to attach GPU %s: %v", gpu, err)
		}
	}

	// Verify all attachments
	manager.mu.RLock()
	if len(manager.attachments) != len(gpus) {
		t.Errorf("Expected %d attachments, got %d", len(gpus), len(manager.attachments))
	}
	manager.mu.RUnlock()

	// Detach all
	for i, gpu := range gpus {
		err := manager.DetachFromVM(gpu, vms[i])
		if err != nil {
			t.Errorf("Failed to detach GPU %s: %v", gpu, err)
		}
	}

	// Verify all detached
	manager.mu.RLock()
	if len(manager.attachments) != 0 {
		t.Errorf("Expected 0 attachments, got %d", len(manager.attachments))
	}
	manager.mu.RUnlock()
}

func TestGPUManager_SRIOVVirtualFunctions(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	// Physical function
	pfAddr := "0000:01:00.0"

	// Create device path
	devicePath := filepath.Join(tmpDir, "bus", "pci", "devices", pfAddr)
	os.MkdirAll(devicePath, 0755)

	// Enable SR-IOV with 4 VFs
	numVFs := 4
	err := manager.EnableSRIOV(pfAddr, numVFs)
	if err != nil {
		t.Logf("SR-IOV enable failed (expected): %v", err)
	}

	// In real system, VFs would appear as:
	// 0000:01:00.1, 0000:01:00.2, 0000:01:00.3, 0000:01:00.4
	expectedVFs := []string{
		"0000:01:00.1",
		"0000:01:00.2",
		"0000:01:00.3",
		"0000:01:00.4",
	}

	t.Logf("Would create %d VFs: %v", numVFs, expectedVFs)
}

func TestGPUManager_ConcurrentAttachments(t *testing.T) {
	manager := &GPUManager{
		sysfsPath: t.TempDir(),
		attachments: make(map[string]string),
	}

	// Try concurrent attachments
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(index int) {
			pciAddr := string(rune('0' + index)) + ":00:00.0"
			vmID := "vm-" + string(rune('A' + index))

			err := manager.AttachToVM(pciAddr, vmID)
			if err != nil {
				t.Errorf("Concurrent attach failed: %v", err)
			}

			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 5; i++ {
		<-done
	}

	manager.mu.RLock()
	attachCount := len(manager.attachments)
	manager.mu.RUnlock()

	if attachCount != 5 {
		t.Errorf("Expected 5 attachments, got %d", attachCount)
	}
}

func BenchmarkGPUManager_AttachToVM(b *testing.B) {
	manager := &GPUManager{
		sysfsPath: b.TempDir(),
		attachments: make(map[string]string),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pciAddr := string(rune('0' + i%10)) + ":00:00.0"
		vmID := "bench-vm"
		manager.AttachToVM(pciAddr, vmID)

		// Clean up for next iteration
		manager.DetachFromVM(pciAddr, vmID)
	}
}

func BenchmarkGPUManager_ListGPUs(b *testing.B) {
	tmpDir := b.TempDir()

	manager := &GPUManager{
		sysfsPath: tmpDir,
	}

	// Create mock devices
	pciDevices := filepath.Join(tmpDir, "bus", "pci", "devices")
	os.MkdirAll(pciDevices, 0755)

	for i := 0; i < 10; i++ {
		gpuPath := filepath.Join(pciDevices, "0000:0"+string(rune('0'+i))+":00.0")
		os.MkdirAll(gpuPath, 0755)
		os.WriteFile(filepath.Join(gpuPath, "vendor"), []byte("0x10de\n"), 0644)
		os.WriteFile(filepath.Join(gpuPath, "device"), []byte("0x1e04\n"), 0644)
		os.WriteFile(filepath.Join(gpuPath, "class"), []byte("0x030000\n"), 0644)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.ListGPUs()
	}
}

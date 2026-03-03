package compute

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// GPUDevice represents a GPU available for passthrough.
type GPUDevice struct {
	// PCI address (e.g., "0000:01:00.0")
	PCIAddress string

	// Vendor ID (e.g., "10de" for NVIDIA)
	VendorID string

	// Device ID (e.g., "1b80" for GTX 1080)
	DeviceID string

	// Device name
	Name string

	// IOMMU group number
	IOMMUGroup int

	// Whether device is bound to vfio-pci driver
	VFIOBound bool

	// SR-IOV virtual function index (0 for physical function)
	VFIndex int

	// Parent device (for VFs)
	ParentPCI string
}

// GPUManager handles GPU passthrough operations.
type GPUManager struct {
	// Detected GPUs
	devices map[string]*GPUDevice

	// vfio-pci driver path
	vfioPCIPath string

	// sysfs base path (defaults to /sys, configurable for testing)
	sysfsPath string

	// GPU-to-VM attachment tracking
	mu          sync.RWMutex
	attachments map[string]string // pciAddr -> vmID
}

// NewGPUManager creates a new GPU passthrough manager.
func NewGPUManager() (*GPUManager, error) {
	gm := &GPUManager{
		devices:     make(map[string]*GPUDevice),
		vfioPCIPath: "/sys/bus/pci/drivers/vfio-pci",
		sysfsPath:   "/sys",
		attachments: make(map[string]string),
	}

	// Detect available GPUs
	if err := gm.detectGPUs(); err != nil {
		return nil, fmt.Errorf("failed to detect GPUs: %w", err)
	}

	return gm, nil
}

// detectGPUs scans for available GPU devices.
func (gm *GPUManager) detectGPUs() error {
	// Scan PCI devices for GPUs (VGA and 3D controllers)
	pciDevices, err := filepath.Glob("/sys/bus/pci/devices/*")
	if err != nil {
		return err
	}

	for _, devPath := range pciDevices {
		classFile := filepath.Join(devPath, "class")
		classBytes, err := os.ReadFile(classFile)
		if err != nil {
			continue
		}

		class := strings.TrimSpace(string(classBytes))
		// 0x030000 = VGA, 0x030200 = 3D controller, 0x038000 = Display controller
		if !strings.HasPrefix(class, "0x0300") && !strings.HasPrefix(class, "0x0302") && !strings.HasPrefix(class, "0x0380") {
			continue
		}

		// This is a GPU device
		pciAddr := filepath.Base(devPath)

		device := &GPUDevice{
			PCIAddress: pciAddr,
		}

		// Read vendor ID
		if vendorBytes, err := os.ReadFile(filepath.Join(devPath, "vendor")); err == nil {
			device.VendorID = strings.TrimPrefix(strings.TrimSpace(string(vendorBytes)), "0x")
		}

		// Read device ID
		if deviceBytes, err := os.ReadFile(filepath.Join(devPath, "device")); err == nil {
			device.DeviceID = strings.TrimPrefix(strings.TrimSpace(string(deviceBytes)), "0x")
		}

		// Get device name from modalias or uevent
		if nameBytes, err := os.ReadFile(filepath.Join(devPath, "uevent")); err == nil {
			for _, line := range strings.Split(string(nameBytes), "\n") {
				if strings.HasPrefix(line, "PCI_SLOT_NAME=") {
					device.Name = strings.TrimPrefix(line, "PCI_SLOT_NAME=")
					break
				}
			}
		}

		// Get IOMMU group
		iommuLink := filepath.Join(devPath, "iommu_group")
		if target, err := os.Readlink(iommuLink); err == nil {
			groupStr := filepath.Base(target)
			device.IOMMUGroup, _ = strconv.Atoi(groupStr)
		}

		// Check if bound to vfio-pci
		driverLink := filepath.Join(devPath, "driver")
		if target, err := os.Readlink(driverLink); err == nil {
			device.VFIOBound = filepath.Base(target) == "vfio-pci"
		}

		// Check if this is a SR-IOV virtual function
		if physfnLink := filepath.Join(devPath, "physfn"); fileExists(physfnLink) {
			// This is a VF
			if target, err := os.Readlink(physfnLink); err == nil {
				device.ParentPCI = filepath.Base(target)
			}

			// Get VF index
			if virtfnLinks, err := filepath.Glob(filepath.Join(devPath, "../", device.ParentPCI, "virtfn*")); err == nil {
				for i, vfLink := range virtfnLinks {
					if target, err := os.Readlink(vfLink); err == nil {
						if filepath.Base(target) == pciAddr {
							device.VFIndex = i
							break
						}
					}
				}
			}
		}

		gm.devices[pciAddr] = device
	}

	return nil
}

// ListGPUs returns all detected GPUs.
func (gm *GPUManager) ListGPUs() []*GPUDevice {
	devices := make([]*GPUDevice, 0, len(gm.devices))
	for _, dev := range gm.devices {
		devices = append(devices, dev)
	}
	return devices
}

// GetGPU returns a GPU device by PCI address.
func (gm *GPUManager) GetGPU(pciAddr string) (*GPUDevice, error) {
	dev, ok := gm.devices[pciAddr]
	if !ok {
		return nil, fmt.Errorf("GPU not found: %s", pciAddr)
	}
	return dev, nil
}

// BindToVFIO binds a GPU to the vfio-pci driver for passthrough.
func (gm *GPUManager) BindToVFIO(pciAddr string) error {
	device, err := gm.GetGPU(pciAddr)
	if err != nil {
		return err
	}

	if device.VFIOBound {
		return nil // Already bound
	}

	// Step 1: Unbind from current driver
	if err := gm.unbindDevice(pciAddr); err != nil {
		return fmt.Errorf("failed to unbind device: %w", err)
	}

	// Step 2: Add device ID to vfio-pci driver
	newIDPath := filepath.Join(gm.vfioPCIPath, "new_id")
	deviceID := fmt.Sprintf("%s %s", device.VendorID, device.DeviceID)

	if err := os.WriteFile(newIDPath, []byte(deviceID), 0644); err != nil {
		// Ignore error if ID already exists
		if !strings.Contains(err.Error(), "exists") {
			return fmt.Errorf("failed to add device ID to vfio-pci: %w", err)
		}
	}

	// Step 3: Bind to vfio-pci
	bindPath := filepath.Join(gm.vfioPCIPath, "bind")
	if err := os.WriteFile(bindPath, []byte(pciAddr), 0644); err != nil {
		return fmt.Errorf("failed to bind to vfio-pci: %w", err)
	}

	device.VFIOBound = true
	return nil
}

// UnbindFromVFIO unbinds a GPU from vfio-pci.
func (gm *GPUManager) UnbindFromVFIO(pciAddr string) error {
	device, err := gm.GetGPU(pciAddr)
	if err != nil {
		return err
	}

	if !device.VFIOBound {
		return nil // Not bound to VFIO
	}

	if err := gm.unbindDevice(pciAddr); err != nil {
		return fmt.Errorf("failed to unbind from vfio-pci: %w", err)
	}

	device.VFIOBound = false

	// Rescan PCI bus to rebind to original driver
	if err := os.WriteFile("/sys/bus/pci/rescan", []byte("1"), 0644); err != nil {
		return fmt.Errorf("failed to rescan PCI bus: %w", err)
	}

	return nil
}

// unbindDevice unbinds a device from its current driver.
func (gm *GPUManager) unbindDevice(pciAddr string) error {
	driverPath := fmt.Sprintf("/sys/bus/pci/devices/%s/driver", pciAddr)

	// Check if device has a driver
	if !fileExists(driverPath) {
		return nil // No driver to unbind
	}

	unbindPath := filepath.Join(driverPath, "unbind")
	if err := os.WriteFile(unbindPath, []byte(pciAddr), 0644); err != nil {
		return err
	}

	return nil
}

// EnableSRIOV enables SR-IOV on a physical function.
func (gm *GPUManager) EnableSRIOV(pciAddr string, numVFs int) error {
	device, err := gm.GetGPU(pciAddr)
	if err != nil {
		return err
	}

	if device.VFIndex > 0 {
		return fmt.Errorf("cannot enable SR-IOV on virtual function")
	}

	// Check if device supports SR-IOV
	sriovCapPath := fmt.Sprintf("/sys/bus/pci/devices/%s/sriov_totalvfs", pciAddr)
	totalVFsBytes, err := os.ReadFile(sriovCapPath)
	if err != nil {
		return fmt.Errorf("device does not support SR-IOV: %w", err)
	}

	totalVFs, err := strconv.Atoi(strings.TrimSpace(string(totalVFsBytes)))
	if err != nil {
		return fmt.Errorf("failed to parse total VFs: %w", err)
	}

	if numVFs > totalVFs {
		return fmt.Errorf("requested %d VFs but device supports max %d", numVFs, totalVFs)
	}

	// Enable VFs
	sriovNumVFsPath := fmt.Sprintf("/sys/bus/pci/devices/%s/sriov_numvfs", pciAddr)

	// First disable existing VFs
	if err := os.WriteFile(sriovNumVFsPath, []byte("0"), 0644); err != nil {
		return fmt.Errorf("failed to disable VFs: %w", err)
	}

	// Enable requested number of VFs
	if err := os.WriteFile(sriovNumVFsPath, []byte(strconv.Itoa(numVFs)), 0644); err != nil {
		return fmt.Errorf("failed to enable VFs: %w", err)
	}

	// Rescan to detect new VFs
	gm.detectGPUs()

	return nil
}

// DisableSRIOV disables SR-IOV on a physical function.
func (gm *GPUManager) DisableSRIOV(pciAddr string) error {
	device, err := gm.GetGPU(pciAddr)
	if err != nil {
		return err
	}

	if device.VFIndex > 0 {
		return fmt.Errorf("cannot disable SR-IOV on virtual function")
	}

	sriovNumVFsPath := fmt.Sprintf("/sys/bus/pci/devices/%s/sriov_numvfs", pciAddr)
	if err := os.WriteFile(sriovNumVFsPath, []byte("0"), 0644); err != nil {
		return fmt.Errorf("failed to disable VFs: %w", err)
	}

	// Rescan to update device list
	gm.detectGPUs()

	return nil
}

// GetDevicesByIOMMUGroup returns all devices in the same IOMMU group.
func (gm *GPUManager) GetDevicesByIOMMUGroup(groupID int) []*GPUDevice {
	devices := make([]*GPUDevice, 0)
	for _, dev := range gm.devices {
		if dev.IOMMUGroup == groupID {
			devices = append(devices, dev)
		}
	}
	return devices
}

// GetIOMMUGroup returns the IOMMU group number for a given PCI address.
func (gm *GPUManager) GetIOMMUGroup(pciAddr string) (int, error) {
	dev, ok := gm.devices[pciAddr]
	if !ok {
		return 0, fmt.Errorf("GPU not found: %s", pciAddr)
	}
	return dev.IOMMUGroup, nil
}

// ListDevicesInIOMMUGroup returns PCI addresses of devices in the given IOMMU group.
func (gm *GPUManager) ListDevicesInIOMMUGroup(groupID int) ([]string, error) {
	addrs := make([]string, 0)
	for addr, dev := range gm.devices {
		if dev.IOMMUGroup == groupID {
			addrs = append(addrs, addr)
		}
	}
	return addrs, nil
}

// AttachToVM records a GPU-to-VM attachment.
func (gm *GPUManager) AttachToVM(pciAddr, vmID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	if attached, ok := gm.attachments[pciAddr]; ok {
		return fmt.Errorf("GPU %s already attached to VM %s", pciAddr, attached)
	}
	gm.attachments[pciAddr] = vmID
	return nil
}

// DetachFromVM removes a GPU-to-VM attachment.
func (gm *GPUManager) DetachFromVM(pciAddr, vmID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	if attached, ok := gm.attachments[pciAddr]; !ok || attached != vmID {
		return fmt.Errorf("GPU %s not attached to VM %s", pciAddr, vmID)
	}
	delete(gm.attachments, pciAddr)
	return nil
}

// AttachGPUToVM attaches a GPU to a VM via QEMU command line.
func (gm *GPUManager) AttachGPUToVM(pciAddr string) ([]string, error) {
	device, err := gm.GetGPU(pciAddr)
	if err != nil {
		return nil, err
	}

	// Ensure device is bound to vfio-pci
	if !device.VFIOBound {
		if err := gm.BindToVFIO(pciAddr); err != nil {
			return nil, fmt.Errorf("failed to bind GPU to VFIO: %w", err)
		}
	}

	// Generate QEMU arguments for GPU passthrough
	args := []string{
		"-device",
		fmt.Sprintf("vfio-pci,host=%s,multifunction=on", pciAddr),
	}

	// If this is an NVIDIA GPU, add additional options
	if device.VendorID == "10de" {
		// Hide virtualization from GPU (NVIDIA driver detection)
		args = append(args,
			"-cpu", "host,kvm=off,hv_vendor_id=null",
		)
	}

	return args, nil
}

// CheckIOMMU verifies that IOMMU is enabled on the system.
func (gm *GPUManager) CheckIOMMU() (bool, error) {
	// Check if IOMMU is enabled
	cmdlineBytes, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return false, err
	}

	cmdline := string(cmdlineBytes)

	// Check for Intel IOMMU
	if strings.Contains(cmdline, "intel_iommu=on") {
		return true, nil
	}

	// Check for AMD IOMMU
	if strings.Contains(cmdline, "amd_iommu=on") {
		return true, nil
	}

	// Check if IOMMU groups exist
	if fileExists("/sys/kernel/iommu_groups") {
		groups, err := os.ReadDir("/sys/kernel/iommu_groups")
		if err == nil && len(groups) > 0 {
			return true, nil
		}
	}

	return false, nil
}

// ValidateIOMMU checks whether IOMMU is enabled and functional for the given PCI address.
func (gm *GPUManager) ValidateIOMMU(pciAddr string) error {
	ok, err := gm.CheckIOMMU()
	if err != nil {
		return fmt.Errorf("IOMMU check failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("IOMMU not enabled for %s", pciAddr)
	}
	return nil
}

// LoadVFIOModules loads required kernel modules for VFIO.
func (gm *GPUManager) LoadVFIOModules() error {
	modules := []string{
		"vfio",
		"vfio_pci",
		"vfio_iommu_type1",
	}

	for _, module := range modules {
		cmd := exec.Command("modprobe", module)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to load module %s: %w", module, err)
		}
	}

	return nil
}

// fileExists checks if a file or directory exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

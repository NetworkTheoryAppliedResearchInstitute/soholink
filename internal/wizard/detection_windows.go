//go:build windows

package wizard

import (
	"fmt"
	"os/exec"
	"strings"
)

// detectDriveTypeImpl detects drive type on Windows.
func detectDriveTypeImpl() string {
	// Try to use PowerShell to query drive type
	cmd := exec.Command("powershell", "-Command",
		"Get-PhysicalDisk | Select-Object -First 1 -ExpandProperty MediaType")
	output, err := cmd.Output()
	if err != nil {
		return "Unknown"
	}

	mediaType := strings.TrimSpace(string(output))
	switch mediaType {
	case "SSD":
		return "SSD"
	case "HDD":
		return "HDD"
	case "SCM": // Storage Class Memory (e.g., Intel Optane)
		return "NVMe"
	default:
		return "Unknown"
	}
}

// detectGPUImpl detects GPU on Windows.
func detectGPUImpl() *GPUInfo {
	// Try to use PowerShell to query GPU
	cmd := exec.Command("powershell", "-Command",
		"Get-WmiObject Win32_VideoController | Select-Object -First 1 Name, AdapterRAM, DriverVersion")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 3 {
		return nil
	}

	// Parse output (very basic)
	gpu := &GPUInfo{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				gpu.Model = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "DriverVersion") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				gpu.DriverVersion = strings.TrimSpace(parts[1])
			}
		}
	}

	// Detect vendor from model name
	if strings.Contains(gpu.Model, "NVIDIA") || strings.Contains(gpu.Model, "GeForce") || strings.Contains(gpu.Model, "RTX") {
		gpu.Vendor = "NVIDIA"
	} else if strings.Contains(gpu.Model, "AMD") || strings.Contains(gpu.Model, "Radeon") {
		gpu.Vendor = "AMD"
	} else if strings.Contains(gpu.Model, "Intel") {
		gpu.Vendor = "Intel"
	}

	// Only return if we found a discrete GPU (not integrated)
	if gpu.Vendor == "NVIDIA" || gpu.Vendor == "AMD" {
		return gpu
	}

	return nil
}

// detectHypervisorImpl detects Hyper-V on Windows.
func detectHypervisorImpl() (HypervisorInfo, error) {
	info := HypervisorInfo{
		Type:     "hyper-v",
		Installed: false,
		Enabled:  false,
		Features: []string{},
	}

	// Check if Hyper-V is installed
	cmd := exec.Command("powershell", "-Command",
		"Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All | Select-Object -ExpandProperty State")
	output, err := cmd.Output()
	if err == nil {
		state := strings.TrimSpace(string(output))
		if state == "Enabled" {
			info.Installed = true
			info.Enabled = true
		}
	}

	// Get Hyper-V version if installed
	if info.Installed {
		cmd = exec.Command("powershell", "-Command",
			"(Get-Command vmms.exe).FileVersionInfo.ProductVersion")
		output, err = cmd.Output()
		if err == nil {
			info.Version = strings.TrimSpace(string(output))
		}

		// Check for features
		info.Features = []string{
			"dynamic_memory",
			"virtual_switch",
			"nested_virtualization",
		}
	}

	return info, nil
}

// detectFirewallEnabledImpl detects Windows Firewall status.
func detectFirewallEnabledImpl() bool {
	cmd := exec.Command("netsh", "advfirewall", "show", "allprofiles", "state")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if any profile shows "ON"
	return strings.Contains(string(output), "ON")
}

// detectVirtualizationEnabledImpl detects if virtualization is enabled on Windows.
func detectVirtualizationEnabledImpl() bool {
	// Method 1: Check systeminfo for Hyper-V requirements
	cmd := exec.Command("systeminfo")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	outputStr := string(output)

	// Look for "Hyper-V Requirements" section
	// If virtualization is enabled, we'll see:
	// "VM Monitor Mode Extensions: Yes"
	// "Virtualization Enabled In Firmware: Yes"

	if strings.Contains(outputStr, "Virtualization Enabled In Firmware: Yes") {
		return true
	}

	// Method 2: Check if Hyper-V is running
	cmd = exec.Command("powershell", "-Command",
		"Get-Service -Name vmms -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Status")
	output, err = cmd.Output()
	if err == nil {
		status := strings.TrimSpace(string(output))
		if status == "Running" {
			return true
		}
	}

	return false
}

// GetElectricityRate attempts to get electricity rate based on location.
// Returns 0 if unable to determine (user will input manually).
func GetElectricityRate() float64 {
	// This would ideally query a public API with electricity rates by region
	// For now, return 0 to let user input manually
	// Future: Use IP geolocation + public utility data
	return 0.0
}

// MeasurePowerDraw attempts to measure system power draw on Windows.
// Returns estimated watts based on CPU/RAM usage.
func MeasurePowerDraw() (idle, load float64, err error) {
	// Windows doesn't expose direct power measurement easily
	// We'll estimate based on system specs

	// This is a placeholder - real implementation would:
	// 1. Query performance counters for CPU/RAM usage
	// 2. Estimate power from TDP values
	// 3. Add baseline for motherboard, drives, etc.

	// For now, return conservative estimates
	// These will be refined in cost_calculator.go
	return 0, 0, fmt.Errorf("direct power measurement not available on Windows")
}

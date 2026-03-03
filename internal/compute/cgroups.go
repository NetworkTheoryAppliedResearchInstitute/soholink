package compute

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CgroupController represents a cgroup v2 resource controller.
type CgroupController string

const (
	// CgroupControllerCPU manages CPU time distribution
	CgroupControllerCPU CgroupController = "cpu"

	// CgroupControllerMemory manages memory allocation and limits
	CgroupControllerMemory CgroupController = "memory"

	// CgroupControllerIO manages I/O bandwidth
	CgroupControllerIO CgroupController = "io"

	// CgroupControllerPIDs limits number of processes
	CgroupControllerPIDs CgroupController = "pids"

	// CgroupControllerCPUSet manages CPU affinity
	CgroupControllerCPUSet CgroupController = "cpuset"
)

// CgroupV2Manager manages cgroup v2 resource controls.
type CgroupV2Manager struct {
	// Root cgroup path (typically /sys/fs/cgroup)
	rootPath string

	// SoHoLink cgroup hierarchy
	soholinkPath string
}

// CgroupLimits defines resource limits for a cgroup.
type CgroupLimits struct {
	// CPU limits
	CPUWeight       int    // CPU scheduling weight (1-10000, default 100)
	CPUMax          string // CPU quota (e.g., "50000 100000" = 50% of 1 CPU)
	CPUSetCPUs      string // CPU affinity (e.g., "0-3" or "0,2,4")
	CPUSetMems      string // Memory node affinity

	// Memory limits
	MemoryMin       int64 // Memory minimum (soft guarantee)
	MemoryLow       int64 // Memory low watermark
	MemoryHigh      int64 // Memory high watermark (throttling)
	MemoryMax       int64 // Memory hard limit
	MemorySwapMax   int64 // Swap limit

	// I/O limits
	IOWeight        int               // I/O scheduling weight (1-10000, default 100)
	IOMax           map[string]string // Device-specific I/O limits (e.g., "8:0 rbps=1048576")

	// Process limits
	PIDsMax         int // Maximum number of processes
}

// NewCgroupV2Manager creates a new cgroup v2 manager.
func NewCgroupV2Manager() (*CgroupV2Manager, error) {
	rootPath := "/sys/fs/cgroup"

	// Verify cgroup v2 is mounted
	if !fileExists(filepath.Join(rootPath, "cgroup.controllers")) {
		return nil, fmt.Errorf("cgroup v2 not mounted at %s", rootPath)
	}

	// Create SoHoLink hierarchy
	soholinkPath := filepath.Join(rootPath, "soholink")
	if err := os.MkdirAll(soholinkPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create soholink cgroup: %w", err)
	}

	mgr := &CgroupV2Manager{
		rootPath:     rootPath,
		soholinkPath: soholinkPath,
	}

	// Enable controllers in soholink cgroup
	if err := mgr.enableControllers(soholinkPath); err != nil {
		return nil, fmt.Errorf("failed to enable controllers: %w", err)
	}

	return mgr, nil
}

// CreateCgroup creates a new cgroup for a container/VM.
func (c *CgroupV2Manager) CreateCgroup(name string) (string, error) {
	cgroupPath := filepath.Join(c.soholinkPath, name)

	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create cgroup: %w", err)
	}

	// Enable controllers for this cgroup
	if err := c.enableControllers(cgroupPath); err != nil {
		os.RemoveAll(cgroupPath)
		return "", fmt.Errorf("failed to enable controllers: %w", err)
	}

	return cgroupPath, nil
}

// DeleteCgroup removes a cgroup.
func (c *CgroupV2Manager) DeleteCgroup(name string) error {
	cgroupPath := filepath.Join(c.soholinkPath, name)

	// Move all processes to parent cgroup first
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	if procs, err := os.ReadFile(procsFile); err == nil {
		parentProcs := filepath.Join(c.soholinkPath, "cgroup.procs")
		for _, pid := range strings.Split(string(procs), "\n") {
			if pid != "" {
				os.WriteFile(parentProcs, []byte(pid), 0644) // #nosec G703 -- parentProcs is an internal cgroup path under soholinkPath; not user-controlled
			}
		}
	}

	// Remove cgroup directory
	if err := os.Remove(cgroupPath); err != nil {
		return fmt.Errorf("failed to remove cgroup: %w", err)
	}

	return nil
}

// ApplyLimits applies resource limits to a cgroup.
func (c *CgroupV2Manager) ApplyLimits(name string, limits *CgroupLimits) error {
	cgroupPath := filepath.Join(c.soholinkPath, name)

	// Apply CPU limits
	if limits.CPUWeight > 0 {
		if err := c.writeCgroupFile(cgroupPath, "cpu.weight", strconv.Itoa(limits.CPUWeight)); err != nil {
			return err
		}
	}

	if limits.CPUMax != "" {
		if err := c.writeCgroupFile(cgroupPath, "cpu.max", limits.CPUMax); err != nil {
			return err
		}
	}

	if limits.CPUSetCPUs != "" {
		if err := c.writeCgroupFile(cgroupPath, "cpuset.cpus", limits.CPUSetCPUs); err != nil {
			return err
		}
	}

	if limits.CPUSetMems != "" {
		if err := c.writeCgroupFile(cgroupPath, "cpuset.mems", limits.CPUSetMems); err != nil {
			return err
		}
	}

	// Apply memory limits
	if limits.MemoryMin > 0 {
		if err := c.writeCgroupFile(cgroupPath, "memory.min", strconv.FormatInt(limits.MemoryMin, 10)); err != nil {
			return err
		}
	}

	if limits.MemoryLow > 0 {
		if err := c.writeCgroupFile(cgroupPath, "memory.low", strconv.FormatInt(limits.MemoryLow, 10)); err != nil {
			return err
		}
	}

	if limits.MemoryHigh > 0 {
		if err := c.writeCgroupFile(cgroupPath, "memory.high", strconv.FormatInt(limits.MemoryHigh, 10)); err != nil {
			return err
		}
	}

	if limits.MemoryMax > 0 {
		if err := c.writeCgroupFile(cgroupPath, "memory.max", strconv.FormatInt(limits.MemoryMax, 10)); err != nil {
			return err
		}
	}

	if limits.MemorySwapMax > 0 {
		if err := c.writeCgroupFile(cgroupPath, "memory.swap.max", strconv.FormatInt(limits.MemorySwapMax, 10)); err != nil {
			return err
		}
	}

	// Apply I/O limits
	if limits.IOWeight > 0 {
		if err := c.writeCgroupFile(cgroupPath, "io.weight", strconv.Itoa(limits.IOWeight)); err != nil {
			return err
		}
	}

	if len(limits.IOMax) > 0 {
		for device, limit := range limits.IOMax {
			ioMax := fmt.Sprintf("%s %s", device, limit)
			if err := c.writeCgroupFile(cgroupPath, "io.max", ioMax); err != nil {
				return err
			}
		}
	}

	// Apply PID limits
	if limits.PIDsMax > 0 {
		if err := c.writeCgroupFile(cgroupPath, "pids.max", strconv.Itoa(limits.PIDsMax)); err != nil {
			return err
		}
	}

	return nil
}

// AddProcess adds a process to a cgroup.
func (c *CgroupV2Manager) AddProcess(name string, pid int) error {
	cgroupPath := filepath.Join(c.soholinkPath, name)
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")

	if err := os.WriteFile(procsFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to add process to cgroup: %w", err)
	}

	return nil
}

// GetStats retrieves resource usage statistics from a cgroup.
func (c *CgroupV2Manager) GetStats(name string) (*CgroupStats, error) {
	cgroupPath := filepath.Join(c.soholinkPath, name)

	stats := &CgroupStats{}

	// Read CPU stats
	cpuStat, err := c.readCgroupFile(cgroupPath, "cpu.stat")
	if err == nil {
		stats.CPUStats = c.parseCPUStat(cpuStat)
	}

	// Read memory stats
	memoryCurrent, err := c.readCgroupFile(cgroupPath, "memory.current")
	if err == nil {
		stats.MemoryCurrent, _ = strconv.ParseInt(strings.TrimSpace(memoryCurrent), 10, 64)
	}

	memoryPeak, err := c.readCgroupFile(cgroupPath, "memory.peak")
	if err == nil {
		stats.MemoryPeak, _ = strconv.ParseInt(strings.TrimSpace(memoryPeak), 10, 64)
	}

	memoryStat, err := c.readCgroupFile(cgroupPath, "memory.stat")
	if err == nil {
		stats.MemoryStats = c.parseMemoryStat(memoryStat)
	}

	// Read I/O stats
	ioStat, err := c.readCgroupFile(cgroupPath, "io.stat")
	if err == nil {
		stats.IOStats = c.parseIOStat(ioStat)
	}

	// Read PID stats
	pidsCurrent, err := c.readCgroupFile(cgroupPath, "pids.current")
	if err == nil {
		stats.PIDsCurrent, _ = strconv.Atoi(strings.TrimSpace(pidsCurrent))
	}

	return stats, nil
}

// CgroupStats represents resource usage statistics.
type CgroupStats struct {
	// CPU statistics
	CPUStats map[string]int64

	// Memory statistics
	MemoryCurrent int64
	MemoryPeak    int64
	MemoryStats   map[string]int64

	// I/O statistics
	IOStats map[string]map[string]int64

	// Process statistics
	PIDsCurrent int
}

// enableControllers enables all controllers for a cgroup.
func (c *CgroupV2Manager) enableControllers(cgroupPath string) error {
	// Get available controllers from parent
	parentPath := filepath.Dir(cgroupPath)
	controllersFile := filepath.Join(parentPath, "cgroup.controllers")

	controllers, err := os.ReadFile(controllersFile)
	if err != nil {
		return err
	}

	// Enable all available controllers
	subtreeFile := filepath.Join(parentPath, "cgroup.subtree_control")
	enabledControllers := strings.Fields(string(controllers))

	for _, controller := range enabledControllers {
		enable := fmt.Sprintf("+%s", controller)
		// Ignore errors - controller may already be enabled
		os.WriteFile(subtreeFile, []byte(enable), 0644) // #nosec G703 -- subtreeFile is an internal cgroup path; not user-controlled
	}

	return nil
}

// writeCgroupFile writes a value to a cgroup file.
func (c *CgroupV2Manager) writeCgroupFile(cgroupPath, filename, value string) error {
	path := filepath.Join(cgroupPath, filename)
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}
	return nil
}

// readCgroupFile reads a value from a cgroup file.
func (c *CgroupV2Manager) readCgroupFile(cgroupPath, filename string) (string, error) {
	path := filepath.Join(cgroupPath, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// parseCPUStat parses cpu.stat file.
func (c *CgroupV2Manager) parseCPUStat(data string) map[string]int64 {
	stats := make(map[string]int64)
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			value, _ := strconv.ParseInt(fields[1], 10, 64)
			stats[fields[0]] = value
		}
	}
	return stats
}

// parseMemoryStat parses memory.stat file.
func (c *CgroupV2Manager) parseMemoryStat(data string) map[string]int64 {
	stats := make(map[string]int64)
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			value, _ := strconv.ParseInt(fields[1], 10, 64)
			stats[fields[0]] = value
		}
	}
	return stats
}

// parseIOStat parses io.stat file.
func (c *CgroupV2Manager) parseIOStat(data string) map[string]map[string]int64 {
	stats := make(map[string]map[string]int64)
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			device := fields[0]
			stats[device] = make(map[string]int64)
			for _, field := range fields[1:] {
				parts := strings.Split(field, "=")
				if len(parts) == 2 {
					value, _ := strconv.ParseInt(parts[1], 10, 64)
					stats[device][parts[0]] = value
				}
			}
		}
	}
	return stats
}

// DefaultLimitsForVM returns default cgroup limits for a VM.
func DefaultLimitsForVM(cpus int, memoryMB int64) *CgroupLimits {
	return &CgroupLimits{
		CPUWeight:     100,                                    // Default weight
		CPUMax:        fmt.Sprintf("%d 100000", cpus*100000), // cpus * 100% quota
		CPUSetCPUs:    "",                                    // No affinity by default
		MemoryMax:     memoryMB * 1024 * 1024,               // Convert MB to bytes
		MemoryHigh:    int64(float64(memoryMB) * 0.9 * 1024 * 1024), // 90% soft limit
		MemorySwapMax: 0,                                     // No swap by default
		IOWeight:      100,                                   // Default I/O weight
		PIDsMax:       4096,                                  // Reasonable process limit
	}
}

// DefaultLimitsForContainer returns default cgroup limits for a container.
func DefaultLimitsForContainer(cpus int, memoryMB int64) *CgroupLimits {
	return &CgroupLimits{
		CPUWeight:     100,
		CPUMax:        fmt.Sprintf("%d 100000", cpus*100000),
		MemoryMax:     memoryMB * 1024 * 1024,
		MemoryHigh:    int64(float64(memoryMB) * 0.9 * 1024 * 1024),
		MemorySwapMax: memoryMB * 1024 * 1024 / 2, // Allow 50% swap
		IOWeight:      100,
		PIDsMax:       1024, // Lower limit for containers
	}
}

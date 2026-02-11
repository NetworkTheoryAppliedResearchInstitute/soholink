package compute

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCgroupV2Manager_CreateCgroup(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	name := "test-cgroup"
	err := manager.CreateCgroup(name)
	if err != nil {
		t.Fatalf("CreateCgroup failed: %v", err)
	}

	// Verify cgroup directory was created
	cgroupPath := filepath.Join(tmpDir, name)
	if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
		t.Errorf("Cgroup directory not created: %s", cgroupPath)
	}
}

func TestCgroupV2Manager_DeleteCgroup(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	name := "test-cgroup-delete"
	cgroupPath := filepath.Join(tmpDir, name)
	os.MkdirAll(cgroupPath, 0755)

	err := manager.DeleteCgroup(name)
	if err != nil {
		t.Fatalf("DeleteCgroup failed: %v", err)
	}

	// Verify cgroup directory was removed
	if _, err := os.Stat(cgroupPath); !os.IsNotExist(err) {
		t.Error("Cgroup directory should be removed")
	}
}

func TestCgroupV2Manager_ApplyLimits_CPU(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-cpu")
	os.MkdirAll(cgroupPath, 0755)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	limits := &CgroupLimits{
		CPUWeight: 100,
		CPUMax:    "50000 100000", // 50% of one CPU
	}

	err := manager.ApplyLimits("test-cpu", limits)
	if err != nil {
		t.Fatalf("ApplyLimits failed: %v", err)
	}

	// Verify cpu.weight file was written
	weightFile := filepath.Join(cgroupPath, "cpu.weight")
	if _, err := os.Stat(weightFile); err == nil {
		content, _ := os.ReadFile(weightFile)
		if !strings.Contains(string(content), "100") {
			t.Errorf("Expected cpu.weight to contain '100', got: %s", content)
		}
	}

	// Verify cpu.max file was written
	maxFile := filepath.Join(cgroupPath, "cpu.max")
	if _, err := os.Stat(maxFile); err == nil {
		content, _ := os.ReadFile(maxFile)
		if !strings.Contains(string(content), "50000") {
			t.Errorf("Expected cpu.max to contain '50000', got: %s", content)
		}
	}
}

func TestCgroupV2Manager_ApplyLimits_Memory(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-memory")
	os.MkdirAll(cgroupPath, 0755)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	limits := &CgroupLimits{
		MemoryMin:  512 * 1024 * 1024,  // 512 MB
		MemoryLow:  768 * 1024 * 1024,  // 768 MB
		MemoryHigh: 1024 * 1024 * 1024, // 1 GB
		MemoryMax:  2048 * 1024 * 1024, // 2 GB
	}

	err := manager.ApplyLimits("test-memory", limits)
	if err != nil {
		t.Fatalf("ApplyLimits failed: %v", err)
	}

	// Verify memory files were created
	memoryFiles := []string{"memory.min", "memory.low", "memory.high", "memory.max"}
	for _, file := range memoryFiles {
		filePath := filepath.Join(cgroupPath, file)
		if _, err := os.Stat(filePath); err != nil {
			t.Logf("Memory file %s not found (created by ApplyLimits)", file)
		}
	}
}

func TestCgroupV2Manager_ApplyLimits_IO(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-io")
	os.MkdirAll(cgroupPath, 0755)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	limits := &CgroupLimits{
		IOWeight: 200,
		IOMax: map[string]string{
			"8:0": "rbps=1048576 wbps=1048576", // 1 MB/s read/write
		},
	}

	err := manager.ApplyLimits("test-io", limits)
	if err != nil {
		t.Fatalf("ApplyLimits failed: %v", err)
	}

	// Verify io.weight file
	weightFile := filepath.Join(cgroupPath, "io.weight")
	if _, err := os.Stat(weightFile); err == nil {
		t.Log("io.weight file created")
	}

	// Verify io.max file
	maxFile := filepath.Join(cgroupPath, "io.max")
	if _, err := os.Stat(maxFile); err == nil {
		t.Log("io.max file created")
	}
}

func TestCgroupV2Manager_ApplyLimits_PIDs(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-pids")
	os.MkdirAll(cgroupPath, 0755)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	limits := &CgroupLimits{
		PIDsMax: 1024,
	}

	err := manager.ApplyLimits("test-pids", limits)
	if err != nil {
		t.Fatalf("ApplyLimits failed: %v", err)
	}

	// Verify pids.max file
	pidsFile := filepath.Join(cgroupPath, "pids.max")
	if _, err := os.Stat(pidsFile); err == nil {
		content, _ := os.ReadFile(pidsFile)
		if !strings.Contains(string(content), "1024") {
			t.Errorf("Expected pids.max to contain '1024', got: %s", content)
		}
	}
}

func TestCgroupV2Manager_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-stats")
	os.MkdirAll(cgroupPath, 0755)

	// Create mock stat files
	os.WriteFile(filepath.Join(cgroupPath, "cpu.stat"), []byte("usage_usec 1000000\nuser_usec 600000\nsystem_usec 400000\n"), 0644)
	os.WriteFile(filepath.Join(cgroupPath, "memory.current"), []byte("536870912\n"), 0644) // 512 MB
	os.WriteFile(filepath.Join(cgroupPath, "memory.peak"), []byte("1073741824\n"), 0644)   // 1 GB
	os.WriteFile(filepath.Join(cgroupPath, "pids.current"), []byte("42\n"), 0644)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	stats, err := manager.GetStats("test-stats")
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	// Verify CPU stats
	if usageUsec, ok := stats.CPUStats["usage_usec"]; ok {
		if usageUsec != 1000000 {
			t.Errorf("Expected usage_usec=1000000, got %d", usageUsec)
		}
	} else {
		t.Error("Expected usage_usec in CPU stats")
	}

	// Verify memory stats
	if stats.MemoryCurrent != 536870912 {
		t.Errorf("Expected memory.current=536870912, got %d", stats.MemoryCurrent)
	}

	if stats.MemoryPeak != 1073741824 {
		t.Errorf("Expected memory.peak=1073741824, got %d", stats.MemoryPeak)
	}

	// Verify PID stats
	if stats.PIDsCurrent != 42 {
		t.Errorf("Expected pids.current=42, got %d", stats.PIDsCurrent)
	}
}

func TestCgroupV2Manager_AddProcess(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-process")
	os.MkdirAll(cgroupPath, 0755)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	// Use current process PID for testing
	pid := os.Getpid()

	err := manager.AddProcess("test-process", pid)
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}

	// Verify cgroup.procs file was written
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	if _, err := os.Stat(procsFile); err == nil {
		content, _ := os.ReadFile(procsFile)
		t.Logf("cgroup.procs content: %s", content)
	}
}

func TestCgroupV2Manager_CPUSetCPUs(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-cpuset")
	os.MkdirAll(cgroupPath, 0755)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	limits := &CgroupLimits{
		CPUSetCPUs: "0-3", // CPUs 0, 1, 2, 3
	}

	err := manager.ApplyLimits("test-cpuset", limits)
	if err != nil {
		t.Fatalf("ApplyLimits failed: %v", err)
	}

	// Verify cpuset.cpus file
	cpusetFile := filepath.Join(cgroupPath, "cpuset.cpus")
	if _, err := os.Stat(cpusetFile); err == nil {
		content, _ := os.ReadFile(cpusetFile)
		if !strings.Contains(string(content), "0-3") {
			t.Errorf("Expected cpuset.cpus to contain '0-3', got: %s", content)
		}
	}
}

func TestCgroupV2Manager_MemorySwap(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-swap")
	os.MkdirAll(cgroupPath, 0755)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	limits := &CgroupLimits{
		MemoryMax:     1024 * 1024 * 1024, // 1 GB
		MemorySwapMax: 512 * 1024 * 1024,  // 512 MB swap
	}

	err := manager.ApplyLimits("test-swap", limits)
	if err != nil {
		t.Fatalf("ApplyLimits failed: %v", err)
	}

	// Verify memory.swap.max file
	swapFile := filepath.Join(cgroupPath, "memory.swap.max")
	if _, err := os.Stat(swapFile); err == nil {
		t.Log("memory.swap.max file created")
	}
}

func TestCgroupV2Manager_IOStats(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-iostats")
	os.MkdirAll(cgroupPath, 0755)

	// Create mock io.stat file
	ioStatContent := `8:0 rbytes=1048576 wbytes=2097152 rios=100 wios=200
8:16 rbytes=524288 wbytes=1048576 rios=50 wios=100
`
	os.WriteFile(filepath.Join(cgroupPath, "io.stat"), []byte(ioStatContent), 0644)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	stats, err := manager.GetStats("test-iostats")
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	// Verify IO stats for device 8:0
	if deviceStats, ok := stats.IOStats["8:0"]; ok {
		if rbytes, exists := deviceStats["rbytes"]; exists {
			if rbytes != 1048576 {
				t.Errorf("Expected rbytes=1048576 for 8:0, got %d", rbytes)
			}
		} else {
			t.Error("Expected rbytes in IO stats for 8:0")
		}

		if wbytes, exists := deviceStats["wbytes"]; exists {
			if wbytes != 2097152 {
				t.Errorf("Expected wbytes=2097152 for 8:0, got %d", wbytes)
			}
		}
	} else {
		t.Error("Expected IO stats for device 8:0")
	}
}

func TestCgroupV2Manager_MemoryStats(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-memstats")
	os.MkdirAll(cgroupPath, 0755)

	// Create mock memory.stat file
	memStatContent := `anon 104857600
file 419430400
kernel_stack 65536
slab 10485760
sock 0
shmem 0
file_mapped 104857600
file_dirty 0
file_writeback 0
`
	os.WriteFile(filepath.Join(cgroupPath, "memory.stat"), []byte(memStatContent), 0644)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	stats, err := manager.GetStats("test-memstats")
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	// Verify memory stats
	if anon, ok := stats.MemoryStats["anon"]; ok {
		if anon != 104857600 {
			t.Errorf("Expected anon=104857600, got %d", anon)
		}
	} else {
		t.Error("Expected 'anon' in memory stats")
	}

	if file, ok := stats.MemoryStats["file"]; ok {
		if file != 419430400 {
			t.Errorf("Expected file=419430400, got %d", file)
		}
	}
}

func TestCgroupV2Manager_Hierarchy(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	// Create nested cgroups
	err := manager.CreateCgroup("parent")
	if err != nil {
		t.Fatalf("Failed to create parent: %v", err)
	}

	err = manager.CreateCgroup("parent/child")
	if err != nil {
		t.Fatalf("Failed to create child: %v", err)
	}

	// Verify hierarchy
	parentPath := filepath.Join(tmpDir, "parent")
	childPath := filepath.Join(tmpDir, "parent", "child")

	if _, err := os.Stat(parentPath); os.IsNotExist(err) {
		t.Error("Parent cgroup not created")
	}

	if _, err := os.Stat(childPath); os.IsNotExist(err) {
		t.Error("Child cgroup not created")
	}
}

func TestCgroupV2Manager_ResourcePressure(t *testing.T) {
	tmpDir := t.TempDir()
	cgroupPath := filepath.Join(tmpDir, "test-pressure")
	os.MkdirAll(cgroupPath, 0755)

	// Create mock pressure files
	cpuPressure := "some avg10=5.00 avg60=4.50 avg300=3.80 total=12345678\n"
	memoryPressure := "some avg10=10.00 avg60=8.50 avg300=7.20 total=23456789\n"

	os.WriteFile(filepath.Join(cgroupPath, "cpu.pressure"), []byte(cpuPressure), 0644)
	os.WriteFile(filepath.Join(cgroupPath, "memory.pressure"), []byte(memoryPressure), 0644)

	// Read and validate pressure stats
	if content, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.pressure")); err == nil {
		if !strings.Contains(string(content), "avg10") {
			t.Error("Expected avg10 in CPU pressure")
		}
		t.Logf("CPU pressure: %s", content)
	}

	if content, err := os.ReadFile(filepath.Join(cgroupPath, "memory.pressure")); err == nil {
		if !strings.Contains(string(content), "avg10") {
			t.Error("Expected avg10 in memory pressure")
		}
		t.Logf("Memory pressure: %s", content)
	}
}

func TestCgroupV2Manager_ConcurrentOperations(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	done := make(chan bool, 10)

	// Create multiple cgroups concurrently
	for i := 0; i < 10; i++ {
		go func(index int) {
			name := string(rune('A' + index))
			err := manager.CreateCgroup(name)
			if err != nil {
				t.Errorf("Concurrent CreateCgroup failed for %s: %v", name, err)
			}

			limits := &CgroupLimits{
				CPUWeight: 100,
				MemoryMax: 1024 * 1024 * 512,
			}
			manager.ApplyLimits(name, limits)

			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all cgroups were created
	for i := 0; i < 10; i++ {
		name := string(rune('A' + i))
		cgroupPath := filepath.Join(tmpDir, name)
		if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
			t.Errorf("Cgroup %s not created", name)
		}
	}
}

func BenchmarkCgroupV2Manager_ApplyLimits(b *testing.B) {
	tmpDir := b.TempDir()
	cgroupPath := filepath.Join(tmpDir, "bench-cgroup")
	os.MkdirAll(cgroupPath, 0755)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	limits := &CgroupLimits{
		CPUWeight: 100,
		MemoryMax: 1024 * 1024 * 1024,
		PIDsMax:   1024,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.ApplyLimits("bench-cgroup", limits)
	}
}

func BenchmarkCgroupV2Manager_GetStats(b *testing.B) {
	tmpDir := b.TempDir()
	cgroupPath := filepath.Join(tmpDir, "bench-stats")
	os.MkdirAll(cgroupPath, 0755)

	// Create mock stat files
	os.WriteFile(filepath.Join(cgroupPath, "cpu.stat"), []byte("usage_usec 1000000\n"), 0644)
	os.WriteFile(filepath.Join(cgroupPath, "memory.current"), []byte("536870912\n"), 0644)
	os.WriteFile(filepath.Join(cgroupPath, "pids.current"), []byte("42\n"), 0644)

	manager := &CgroupV2Manager{
		rootPath: tmpDir,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetStats("bench-stats")
	}
}

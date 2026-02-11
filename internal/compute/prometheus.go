package compute

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ComputePrometheusExporter exposes compute resource metrics in Prometheus format.
type ComputePrometheusExporter struct {
	// Hypervisor for VM metrics
	hypervisor Hypervisor

	// Cgroups manager for resource metrics
	cgroupManager *CgroupV2Manager

	// GPU manager for GPU metrics
	gpuManager *GPUManager

	// Metrics cache
	mu           sync.RWMutex
	vmMetrics    map[string]*VMMetrics
	cgroupStats  map[string]*CgroupStats
	gpuMetrics   map[string]*GPUMetrics
	lastUpdate   time.Time

	// Update interval
	updateInterval time.Duration
}

// VMMetrics represents VM performance metrics.
type VMMetrics struct {
	VMID            string
	State           string
	CPUUsagePercent float64
	MemoryUsedMB    int64
	MemoryTotalMB   int64
	DiskReadBytes   int64
	DiskWriteBytes  int64
	NetworkRxBytes  int64
	NetworkTxBytes  int64
	Uptime          time.Duration
}

// GPUMetrics represents GPU utilization metrics.
type GPUMetrics struct {
	PCIAddress      string
	Name            string
	Utilization     float64 // Percentage
	MemoryUsedMB    int64
	MemoryTotalMB   int64
	Temperature     int // Celsius
	PowerUsageWatts int
	Attached        bool
	VMID            string
}

// NewComputePrometheusExporter creates a new compute metrics exporter.
func NewComputePrometheusExporter(hypervisor Hypervisor, cgroupManager *CgroupV2Manager, gpuManager *GPUManager) *ComputePrometheusExporter {
	return &ComputePrometheusExporter{
		hypervisor:     hypervisor,
		cgroupManager:  cgroupManager,
		gpuManager:     gpuManager,
		vmMetrics:      make(map[string]*VMMetrics),
		cgroupStats:    make(map[string]*CgroupStats),
		gpuMetrics:     make(map[string]*GPUMetrics),
		updateInterval: 15 * time.Second,
	}
}

// Start begins the metrics collection loop.
func (cpe *ComputePrometheusExporter) Start(ctx context.Context) {
	ticker := time.NewTicker(cpe.updateInterval)
	defer ticker.Stop()

	// Initial collection
	cpe.collectMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cpe.collectMetrics(ctx)
		}
	}
}

// collectMetrics collects metrics from all compute resources.
func (cpe *ComputePrometheusExporter) collectMetrics(ctx context.Context) {
	cpe.mu.Lock()
	defer cpe.mu.Unlock()

	// Collect VM metrics
	vms := cpe.hypervisor.ListVMs()
	for _, vmID := range vms {
		metrics := cpe.collectVMMetrics(ctx, vmID)
		if metrics != nil {
			cpe.vmMetrics[vmID] = metrics
		}
	}

	// Collect cgroup metrics
	if cpe.cgroupManager != nil {
		// List all cgroups under soholink hierarchy
		// For now, collect from known VMs/containers
		for vmID := range cpe.vmMetrics {
			stats, err := cpe.cgroupManager.GetStats(vmID)
			if err == nil {
				cpe.cgroupStats[vmID] = stats
			}
		}
	}

	// Collect GPU metrics
	if cpe.gpuManager != nil {
		gpus := cpe.gpuManager.ListGPUs()
		for _, gpu := range gpus {
			metrics := cpe.collectGPUMetrics(gpu)
			if metrics != nil {
				cpe.gpuMetrics[gpu.PCIAddress] = metrics
			}
		}
	}

	cpe.lastUpdate = time.Now()
}

// collectVMMetrics collects metrics for a single VM.
func (cpe *ComputePrometheusExporter) collectVMMetrics(ctx context.Context, vmID string) *VMMetrics {
	// Get VM info
	info, err := cpe.hypervisor.GetVMInfo(vmID)
	if err != nil {
		return nil
	}

	metrics := &VMMetrics{
		VMID:          vmID,
		State:         info.State,
		MemoryTotalMB: info.MemoryMB,
	}

	// TODO: Collect actual runtime metrics from hypervisor
	// For QEMU: query via QMP
	// For Firecracker: query via metrics endpoint
	// For Hyper-V: query via WMI/PowerShell

	return metrics
}

// collectGPUMetrics collects metrics for a GPU.
func (cpe *ComputePrometheusExporter) collectGPUMetrics(gpu *GPUDevice) *GPUMetrics {
	metrics := &GPUMetrics{
		PCIAddress: gpu.PCIAddress,
		Name:       gpu.Name,
		Attached:   gpu.VFIOBound,
	}

	// TODO: Query nvidia-smi for NVIDIA GPUs
	// nvidia-smi --query-gpu=utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw --format=csv,noheader
	// For AMD: use rocm-smi
	// For Intel: use intel_gpu_top

	return metrics
}

// ServeHTTP handles Prometheus /metrics requests.
func (cpe *ComputePrometheusExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cpe.mu.RLock()
	defer cpe.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// VM metrics
	fmt.Fprintf(w, "# HELP soholink_vm_state VM state (0=stopped, 1=running, 2=paused)\n")
	fmt.Fprintf(w, "# TYPE soholink_vm_state gauge\n")

	fmt.Fprintf(w, "# HELP soholink_vm_cpu_usage_percent VM CPU usage percentage\n")
	fmt.Fprintf(w, "# TYPE soholink_vm_cpu_usage_percent gauge\n")

	fmt.Fprintf(w, "# HELP soholink_vm_memory_used_bytes VM memory used in bytes\n")
	fmt.Fprintf(w, "# TYPE soholink_vm_memory_used_bytes gauge\n")

	fmt.Fprintf(w, "# HELP soholink_vm_memory_total_bytes VM total memory in bytes\n")
	fmt.Fprintf(w, "# TYPE soholink_vm_memory_total_bytes gauge\n")

	for vmID, metrics := range cpe.vmMetrics {
		labels := fmt.Sprintf(`vm_id="%s",state="%s"`, vmID, metrics.State)

		stateValue := 0
		if metrics.State == "running" {
			stateValue = 1
		} else if metrics.State == "paused" {
			stateValue = 2
		}
		fmt.Fprintf(w, "soholink_vm_state{%s} %d\n", labels, stateValue)

		fmt.Fprintf(w, "soholink_vm_cpu_usage_percent{%s} %.2f\n", labels, metrics.CPUUsagePercent)
		fmt.Fprintf(w, "soholink_vm_memory_used_bytes{%s} %d\n", labels, metrics.MemoryUsedMB*1024*1024)
		fmt.Fprintf(w, "soholink_vm_memory_total_bytes{%s} %d\n", labels, metrics.MemoryTotalMB*1024*1024)

		// Network I/O
		fmt.Fprintf(w, "# HELP soholink_vm_network_rx_bytes VM network received bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_vm_network_rx_bytes counter\n")
		fmt.Fprintf(w, "soholink_vm_network_rx_bytes{%s} %d\n", labels, metrics.NetworkRxBytes)

		fmt.Fprintf(w, "# HELP soholink_vm_network_tx_bytes VM network transmitted bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_vm_network_tx_bytes counter\n")
		fmt.Fprintf(w, "soholink_vm_network_tx_bytes{%s} %d\n", labels, metrics.NetworkTxBytes)

		// Disk I/O
		fmt.Fprintf(w, "# HELP soholink_vm_disk_read_bytes VM disk read bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_vm_disk_read_bytes counter\n")
		fmt.Fprintf(w, "soholink_vm_disk_read_bytes{%s} %d\n", labels, metrics.DiskReadBytes)

		fmt.Fprintf(w, "# HELP soholink_vm_disk_write_bytes VM disk write bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_vm_disk_write_bytes counter\n")
		fmt.Fprintf(w, "soholink_vm_disk_write_bytes{%s} %d\n", labels, metrics.DiskWriteBytes)
	}

	// Cgroup metrics
	if len(cpe.cgroupStats) > 0 {
		fmt.Fprintf(w, "# HELP soholink_cgroup_cpu_usage_usec Cgroup CPU usage in microseconds\n")
		fmt.Fprintf(w, "# TYPE soholink_cgroup_cpu_usage_usec counter\n")

		fmt.Fprintf(w, "# HELP soholink_cgroup_memory_current_bytes Cgroup current memory usage\n")
		fmt.Fprintf(w, "# TYPE soholink_cgroup_memory_current_bytes gauge\n")

		fmt.Fprintf(w, "# HELP soholink_cgroup_memory_peak_bytes Cgroup peak memory usage\n")
		fmt.Fprintf(w, "# TYPE soholink_cgroup_memory_peak_bytes gauge\n")

		fmt.Fprintf(w, "# HELP soholink_cgroup_pids_current Current number of processes in cgroup\n")
		fmt.Fprintf(w, "# TYPE soholink_cgroup_pids_current gauge\n")

		for cgroupName, stats := range cpe.cgroupStats {
			labels := fmt.Sprintf(`cgroup="%s"`, cgroupName)

			if usageUsec, ok := stats.CPUStats["usage_usec"]; ok {
				fmt.Fprintf(w, "soholink_cgroup_cpu_usage_usec{%s} %d\n", labels, usageUsec)
			}

			fmt.Fprintf(w, "soholink_cgroup_memory_current_bytes{%s} %d\n", labels, stats.MemoryCurrent)
			fmt.Fprintf(w, "soholink_cgroup_memory_peak_bytes{%s} %d\n", labels, stats.MemoryPeak)
			fmt.Fprintf(w, "soholink_cgroup_pids_current{%s} %d\n", labels, stats.PIDsCurrent)

			// I/O stats per device
			for device, deviceStats := range stats.IOStats {
				deviceLabels := fmt.Sprintf(`cgroup="%s",device="%s"`, cgroupName, device)

				if rbytes, ok := deviceStats["rbytes"]; ok {
					fmt.Fprintf(w, "# HELP soholink_cgroup_io_read_bytes Cgroup I/O read bytes\n")
					fmt.Fprintf(w, "# TYPE soholink_cgroup_io_read_bytes counter\n")
					fmt.Fprintf(w, "soholink_cgroup_io_read_bytes{%s} %d\n", deviceLabels, rbytes)
				}

				if wbytes, ok := deviceStats["wbytes"]; ok {
					fmt.Fprintf(w, "# HELP soholink_cgroup_io_write_bytes Cgroup I/O write bytes\n")
					fmt.Fprintf(w, "# TYPE soholink_cgroup_io_write_bytes counter\n")
					fmt.Fprintf(w, "soholink_cgroup_io_write_bytes{%s} %d\n", deviceLabels, wbytes)
				}
			}
		}
	}

	// GPU metrics
	if len(cpe.gpuMetrics) > 0 {
		fmt.Fprintf(w, "# HELP soholink_gpu_utilization_percent GPU utilization percentage\n")
		fmt.Fprintf(w, "# TYPE soholink_gpu_utilization_percent gauge\n")

		fmt.Fprintf(w, "# HELP soholink_gpu_memory_used_bytes GPU memory used in bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_gpu_memory_used_bytes gauge\n")

		fmt.Fprintf(w, "# HELP soholink_gpu_memory_total_bytes GPU total memory in bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_gpu_memory_total_bytes gauge\n")

		fmt.Fprintf(w, "# HELP soholink_gpu_temperature_celsius GPU temperature in Celsius\n")
		fmt.Fprintf(w, "# TYPE soholink_gpu_temperature_celsius gauge\n")

		fmt.Fprintf(w, "# HELP soholink_gpu_power_usage_watts GPU power usage in watts\n")
		fmt.Fprintf(w, "# TYPE soholink_gpu_power_usage_watts gauge\n")

		fmt.Fprintf(w, "# HELP soholink_gpu_attached GPU attached to VM (0=no, 1=yes)\n")
		fmt.Fprintf(w, "# TYPE soholink_gpu_attached gauge\n")

		for pciAddr, metrics := range cpe.gpuMetrics {
			labels := fmt.Sprintf(`pci_address="%s",name="%s"`, pciAddr, metrics.Name)

			fmt.Fprintf(w, "soholink_gpu_utilization_percent{%s} %.2f\n", labels, metrics.Utilization)
			fmt.Fprintf(w, "soholink_gpu_memory_used_bytes{%s} %d\n", labels, metrics.MemoryUsedMB*1024*1024)
			fmt.Fprintf(w, "soholink_gpu_memory_total_bytes{%s} %d\n", labels, metrics.MemoryTotalMB*1024*1024)
			fmt.Fprintf(w, "soholink_gpu_temperature_celsius{%s} %d\n", labels, metrics.Temperature)
			fmt.Fprintf(w, "soholink_gpu_power_usage_watts{%s} %d\n", labels, metrics.PowerUsageWatts)

			attached := 0
			if metrics.Attached {
				attached = 1
			}
			fmt.Fprintf(w, "soholink_gpu_attached{%s} %d\n", labels, attached)
		}
	}

	// Summary metrics
	fmt.Fprintf(w, "# HELP soholink_vms_total Total number of VMs\n")
	fmt.Fprintf(w, "# TYPE soholink_vms_total gauge\n")
	fmt.Fprintf(w, "soholink_vms_total %d\n", len(cpe.vmMetrics))

	fmt.Fprintf(w, "# HELP soholink_gpus_total Total number of GPUs\n")
	fmt.Fprintf(w, "# TYPE soholink_gpus_total gauge\n")
	fmt.Fprintf(w, "soholink_gpus_total %d\n", len(cpe.gpuMetrics))

	// Exporter metadata
	fmt.Fprintf(w, "# HELP soholink_compute_exporter_last_update_timestamp Last metrics update timestamp\n")
	fmt.Fprintf(w, "# TYPE soholink_compute_exporter_last_update_timestamp gauge\n")
	fmt.Fprintf(w, "soholink_compute_exporter_last_update_timestamp %d\n", cpe.lastUpdate.Unix())
}

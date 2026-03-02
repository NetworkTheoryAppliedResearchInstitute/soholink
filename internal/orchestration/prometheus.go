package orchestration

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// OrchestratorPrometheusExporter exposes orchestrator metrics in Prometheus format.
type OrchestratorPrometheusExporter struct {
	scheduler *FedScheduler

	// Metrics cache
	mu         sync.RWMutex
	lastUpdate time.Time
}

// NewOrchestratorPrometheusExporter creates a new orchestrator metrics exporter.
func NewOrchestratorPrometheusExporter(scheduler *FedScheduler) *OrchestratorPrometheusExporter {
	return &OrchestratorPrometheusExporter{
		scheduler: scheduler,
	}
}

// ServeHTTP handles Prometheus /metrics requests.
func (ope *OrchestratorPrometheusExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ope.mu.Lock()
	ope.lastUpdate = time.Now()
	ope.mu.Unlock()

	ope.scheduler.mu.RLock()
	defer ope.scheduler.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Workload metrics
	fmt.Fprintf(w, "# HELP soholink_workloads_total Total number of workloads by status\n")
	fmt.Fprintf(w, "# TYPE soholink_workloads_total gauge\n")

	statusCounts := make(map[string]int)
	for _, workload := range ope.scheduler.ActiveWorkloads {
		statusCounts[workload.Workload.Status]++
	}

	for status, count := range statusCounts {
		fmt.Fprintf(w, "soholink_workloads_total{status=\"%s\"} %d\n", status, count)
	}

	// Pending queue size
	fmt.Fprintf(w, "# HELP soholink_workload_queue_size Workloads in pending queue\n")
	fmt.Fprintf(w, "# TYPE soholink_workload_queue_size gauge\n")
	fmt.Fprintf(w, "soholink_workload_queue_size %d\n", len(ope.scheduler.PendingQueue))

	// Workload details
	fmt.Fprintf(w, "# HELP soholink_workload_replicas_desired Desired replica count\n")
	fmt.Fprintf(w, "# TYPE soholink_workload_replicas_desired gauge\n")

	fmt.Fprintf(w, "# HELP soholink_workload_replicas_running Running replica count\n")
	fmt.Fprintf(w, "# TYPE soholink_workload_replicas_running gauge\n")

	fmt.Fprintf(w, "# HELP soholink_workload_replicas_failed Failed replica count\n")
	fmt.Fprintf(w, "# TYPE soholink_workload_replicas_failed gauge\n")

	fmt.Fprintf(w, "# HELP soholink_workload_cpu_requested CPU cores requested\n")
	fmt.Fprintf(w, "# TYPE soholink_workload_cpu_requested gauge\n")

	fmt.Fprintf(w, "# HELP soholink_workload_memory_requested_bytes Memory bytes requested\n")
	fmt.Fprintf(w, "# TYPE soholink_workload_memory_requested_bytes gauge\n")

	for _, workload := range ope.scheduler.ActiveWorkloads {
		w0 := workload.Workload
		labels := fmt.Sprintf(`workload_id="%s",name="%s",status="%s",type="%s"`,
			w0.WorkloadID,
			w0.Name,
			w0.Status,
			w0.Type,
		)

		fmt.Fprintf(w, "soholink_workload_replicas_desired{%s} %d\n", labels, w0.Replicas)

		// Count running and failed replicas
		runningCount := 0
		failedCount := 0
		for _, placement := range workload.Placements {
			if placement.Status == "running" {
				runningCount++
			} else if placement.Status == "failed" {
				failedCount++
			}
		}

		fmt.Fprintf(w, "soholink_workload_replicas_running{%s} %d\n", labels, runningCount)
		fmt.Fprintf(w, "soholink_workload_replicas_failed{%s} %d\n", labels, failedCount)

		// Resource requests
		if w0.Spec.CPUCores > 0 {
			fmt.Fprintf(w, "soholink_workload_cpu_requested{%s} %.2f\n", labels, w0.Spec.CPUCores)
		}
		if w0.Spec.MemoryMB > 0 {
			fmt.Fprintf(w, "soholink_workload_memory_requested_bytes{%s} %d\n", labels, w0.Spec.MemoryMB*1024*1024)
		}

		// Workload age
		age := time.Since(w0.CreatedAt).Seconds()
		fmt.Fprintf(w, "# HELP soholink_workload_age_seconds Workload age in seconds\n")
		fmt.Fprintf(w, "# TYPE soholink_workload_age_seconds gauge\n")
		fmt.Fprintf(w, "soholink_workload_age_seconds{%s} %.0f\n", labels, age)
	}

	// Node metrics from discovery
	nodes := ope.scheduler.discovery.ListNodes()

	fmt.Fprintf(w, "# HELP soholink_nodes_total Total number of nodes by status\n")
	fmt.Fprintf(w, "# TYPE soholink_nodes_total gauge\n")

	nodeStatusCounts := make(map[string]int)
	for _, node := range nodes {
		nodeStatusCounts[node.Status]++
	}

	for status, count := range nodeStatusCounts {
		fmt.Fprintf(w, "soholink_nodes_total{status=\"%s\"} %d\n", status, count)
	}

	// Node capacity metrics
	fmt.Fprintf(w, "# HELP soholink_node_cpu_capacity Total CPU cores\n")
	fmt.Fprintf(w, "# TYPE soholink_node_cpu_capacity gauge\n")

	fmt.Fprintf(w, "# HELP soholink_node_memory_capacity_bytes Total memory in bytes\n")
	fmt.Fprintf(w, "# TYPE soholink_node_memory_capacity_bytes gauge\n")

	fmt.Fprintf(w, "# HELP soholink_node_cpu_available Available CPU cores\n")
	fmt.Fprintf(w, "# TYPE soholink_node_cpu_available gauge\n")

	fmt.Fprintf(w, "# HELP soholink_node_memory_available_bytes Available memory in bytes\n")
	fmt.Fprintf(w, "# TYPE soholink_node_memory_available_bytes gauge\n")

	fmt.Fprintf(w, "# HELP soholink_node_workload_count Active jobs on node\n")
	fmt.Fprintf(w, "# TYPE soholink_node_workload_count gauge\n")

	for _, node := range nodes {
		labels := fmt.Sprintf(`node_id="%s",address="%s",status="%s"`,
			node.DID,
			node.Address,
			node.Status,
		)

		fmt.Fprintf(w, "soholink_node_cpu_capacity{%s} %.2f\n", labels, node.TotalCPU)
		fmt.Fprintf(w, "soholink_node_memory_capacity_bytes{%s} %d\n", labels, node.TotalMemoryMB*1024*1024)
		fmt.Fprintf(w, "soholink_node_cpu_available{%s} %.2f\n", labels, node.AvailableCPU)
		fmt.Fprintf(w, "soholink_node_memory_available_bytes{%s} %d\n", labels, node.AvailableMemoryMB*1024*1024)

		// Node last heartbeat
		lastHeartbeat := time.Since(node.LastHeartbeat).Seconds()
		fmt.Fprintf(w, "# HELP soholink_node_last_heartbeat_seconds Seconds since last heartbeat\n")
		fmt.Fprintf(w, "# TYPE soholink_node_last_heartbeat_seconds gauge\n")
		fmt.Fprintf(w, "soholink_node_last_heartbeat_seconds{%s} %.0f\n", labels, lastHeartbeat)
	}

	// Scheduler statistics
	fmt.Fprintf(w, "# HELP soholink_scheduler_placement_attempts_total Total placement attempts\n")
	fmt.Fprintf(w, "# TYPE soholink_scheduler_placement_attempts_total counter\n")
	fmt.Fprintf(w, "soholink_scheduler_placement_attempts_total %d\n", len(ope.scheduler.ActiveWorkloads)*10) // Estimate

	fmt.Fprintf(w, "# HELP soholink_scheduler_placement_failures_total Total placement failures\n")
	fmt.Fprintf(w, "# TYPE soholink_scheduler_placement_failures_total counter\n")
	failureCount := 0
	for _, workload := range ope.scheduler.ActiveWorkloads {
		for _, placement := range workload.Placements {
			if placement.Status == "failed" {
				failureCount++
			}
		}
	}
	fmt.Fprintf(w, "soholink_scheduler_placement_failures_total %d\n", failureCount)

	// Exporter metadata
	fmt.Fprintf(w, "# HELP soholink_orchestrator_exporter_last_update_timestamp Last metrics update timestamp\n")
	fmt.Fprintf(w, "# TYPE soholink_orchestrator_exporter_last_update_timestamp gauge\n")
	fmt.Fprintf(w, "soholink_orchestrator_exporter_last_update_timestamp %d\n", ope.lastUpdate.Unix())
}

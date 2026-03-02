package services

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// PrometheusExporter exposes service metrics in Prometheus format.
type PrometheusExporter struct {
	// Docker client for collecting container metrics
	dockerClient *DockerClient

	// Service catalog for listing instances
	catalog *Catalog

	// Metrics cache
	mu           sync.RWMutex
	metricsCache map[string]*ServiceMetrics
	lastUpdate   time.Time

	// Update interval
	updateInterval time.Duration
}

// NewPrometheusExporter creates a new Prometheus exporter.
func NewPrometheusExporter(dockerEndpoint string, catalog *Catalog) *PrometheusExporter {
	return &PrometheusExporter{
		dockerClient:   NewDockerClient(dockerEndpoint),
		catalog:        catalog,
		metricsCache:   make(map[string]*ServiceMetrics),
		updateInterval: 15 * time.Second,
	}
}

// Start begins the metrics collection loop.
func (pe *PrometheusExporter) Start(ctx context.Context) {
	ticker := time.NewTicker(pe.updateInterval)
	defer ticker.Stop()

	// Initial collection
	pe.collectMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pe.collectMetrics(ctx)
		}
	}
}

// collectMetrics collects metrics from all service instances.
func (pe *PrometheusExporter) collectMetrics(ctx context.Context) {
	instances := pe.catalog.ListAllInstances()

	pe.mu.Lock()
	defer pe.mu.Unlock()

	for _, instance := range instances {
		metrics, err := pe.collectInstanceMetrics(ctx, instance)
		if err != nil {
			continue
		}
		pe.metricsCache[instance.InstanceID] = metrics
	}

	pe.lastUpdate = time.Now()
}

// collectInstanceMetrics collects metrics for a single instance.
func (pe *PrometheusExporter) collectInstanceMetrics(ctx context.Context, instance *ServiceInstance) (*ServiceMetrics, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found")
	}

	// Get container stats
	stats, err := pe.dockerClient.GetContainerStats(ctx, containerID)
	if err != nil {
		return nil, err
	}

	// Calculate CPU percentage
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage)
	cpuPercent := 0.0
	if systemDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * 100.0
	}

	// Calculate memory percentage
	memoryUsage := stats.MemoryStats.Usage
	memoryLimit := stats.MemoryStats.Limit
	memoryPercent := 0.0
	if memoryLimit > 0 {
		memoryPercent = (float64(memoryUsage) / float64(memoryLimit)) * 100.0
	}

	metrics := &ServiceMetrics{
		CPUPercent:    cpuPercent,
		MemoryPercent: memoryPercent,
		MemoryUsageMB: memoryUsage / (1024 * 1024),
		Uptime:        time.Since(instance.CreatedAt),
	}

	// Service-specific metrics
	switch instance.ServiceType {
	case ServiceTypePostgres, ServiceTypeMySQL, ServiceTypeMongoDB:
		metrics.Connections = pe.getConnectionCount(ctx, instance)
		metrics.QPS = pe.getQueryRate(ctx, instance)
	}

	return metrics, nil
}

// ServeHTTP handles Prometheus /metrics requests.
func (pe *PrometheusExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Write service instance metrics
	for instanceID, metrics := range pe.metricsCache {
		instance := pe.catalog.GetInstance(instanceID)
		if instance == nil {
			continue
		}

		labels := fmt.Sprintf(`instance_id="%s",service_type="%s",status="%s"`,
			instance.InstanceID,
			instance.ServiceType,
			instance.Status,
		)

		// CPU metrics
		fmt.Fprintf(w, "# HELP soholink_service_cpu_usage_percent CPU usage percentage\n")
		fmt.Fprintf(w, "# TYPE soholink_service_cpu_usage_percent gauge\n")
		fmt.Fprintf(w, "soholink_service_cpu_usage_percent{%s} %.2f\n", labels, metrics.CPUPercent)

		// Memory metrics
		fmt.Fprintf(w, "# HELP soholink_service_memory_usage_percent Memory usage percentage\n")
		fmt.Fprintf(w, "# TYPE soholink_service_memory_usage_percent gauge\n")
		fmt.Fprintf(w, "soholink_service_memory_usage_percent{%s} %.2f\n", labels, metrics.MemoryPercent)

		fmt.Fprintf(w, "# HELP soholink_service_memory_usage_bytes Memory usage in bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_service_memory_usage_bytes gauge\n")
		fmt.Fprintf(w, "soholink_service_memory_usage_bytes{%s} %d\n", labels, metrics.MemoryUsageMB*1024*1024)

		// Uptime
		fmt.Fprintf(w, "# HELP soholink_service_uptime_seconds Service uptime in seconds\n")
		fmt.Fprintf(w, "# TYPE soholink_service_uptime_seconds gauge\n")
		fmt.Fprintf(w, "soholink_service_uptime_seconds{%s} %.0f\n", labels, metrics.Uptime.Seconds())

		// Service-specific metrics
		if metrics.Connections > 0 {
			fmt.Fprintf(w, "# HELP soholink_service_connections Active database connections\n")
			fmt.Fprintf(w, "# TYPE soholink_service_connections gauge\n")
			fmt.Fprintf(w, "soholink_service_connections{%s} %d\n", labels, metrics.Connections)
		}

		if metrics.QPS > 0 {
			fmt.Fprintf(w, "# HELP soholink_service_query_rate Queries per second\n")
			fmt.Fprintf(w, "# TYPE soholink_service_query_rate gauge\n")
			fmt.Fprintf(w, "soholink_service_query_rate{%s} %.2f\n", labels, metrics.QPS)
		}
	}

	// Overall catalog metrics
	instances := pe.catalog.ListAllInstances()
	statusCounts := make(map[string]int)
	typeCounts := make(map[ServiceType]int)

	for _, instance := range instances {
		statusCounts[string(instance.Status)]++
		typeCounts[instance.ServiceType]++
	}

	// Instance count by status
	fmt.Fprintf(w, "# HELP soholink_instances_total Total service instances by status\n")
	fmt.Fprintf(w, "# TYPE soholink_instances_total gauge\n")
	for status, count := range statusCounts {
		fmt.Fprintf(w, "soholink_instances_total{status=\"%s\"} %d\n", status, count)
	}

	// Instance count by type
	fmt.Fprintf(w, "# HELP soholink_instances_by_type Service instances by type\n")
	fmt.Fprintf(w, "# TYPE soholink_instances_by_type gauge\n")
	for serviceType, count := range typeCounts {
		fmt.Fprintf(w, "soholink_instances_by_type{type=\"%s\"} %d\n", serviceType, count)
	}

	// Exporter metadata
	fmt.Fprintf(w, "# HELP soholink_exporter_last_update_timestamp Last metrics update timestamp\n")
	fmt.Fprintf(w, "# TYPE soholink_exporter_last_update_timestamp gauge\n")
	fmt.Fprintf(w, "soholink_exporter_last_update_timestamp %d\n", pe.lastUpdate.Unix())
}

// Service-specific metric collection (stub implementations)

func (pe *PrometheusExporter) getConnectionCount(ctx context.Context, instance *ServiceInstance) int {
	// TODO: Query database for active connections
	return 0
}

func (pe *PrometheusExporter) getQueryRate(ctx context.Context, instance *ServiceInstance) float64 {
	// TODO: Calculate queries per second from stats
	return 0.0
}

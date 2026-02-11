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
	catalog *ServiceCatalog

	// Metrics cache
	mu           sync.RWMutex
	metricsCache map[string]*ServiceMetrics
	lastUpdate   time.Time

	// Update interval
	updateInterval time.Duration
}

// NewPrometheusExporter creates a new Prometheus exporter.
func NewPrometheusExporter(dockerEndpoint string, catalog *ServiceCatalog) *PrometheusExporter {
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
	instances := pe.catalog.ListInstances()

	pe.mu.Lock()
	defer pe.mu.Unlock()

	for _, instance := range instances {
		metrics, err := pe.collectInstanceMetrics(ctx, instance)
		if err != nil {
			// Log error but continue
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

	// Calculate network I/O
	var networkRx, networkTx int64
	for _, netStats := range stats.Networks {
		networkRx += netStats.RxBytes
		networkTx += netStats.TxBytes
	}

	// Calculate disk I/O
	var diskRead, diskWrite int64
	for _, ioEntry := range stats.BlkioStats.IoServiceBytesRecursive {
		if ioEntry.Op == "Read" {
			diskRead += ioEntry.Value
		} else if ioEntry.Op == "Write" {
			diskWrite += ioEntry.Value
		}
	}

	metrics := &ServiceMetrics{
		CPUUsagePercent: cpuPercent,
		MemoryUsagePercent: memoryPercent,
		MemoryUsageMB: memoryUsage / (1024 * 1024),
		NetworkInBytes: networkRx,
		NetworkOutBytes: networkTx,
		DiskReadBytes: diskRead,
		DiskWriteBytes: diskWrite,
		Uptime: time.Since(instance.CreatedAt),
		Timestamp: time.Now(),
	}

	// Service-specific metrics
	switch instance.ServiceType {
	case ServiceTypePostgres, ServiceTypeMySQL, ServiceTypeMongoDB:
		metrics.ConnectionCount = pe.getConnectionCount(ctx, instance)
		metrics.QueryRate = pe.getQueryRate(ctx, instance)
	case ServiceTypeRedis:
		metrics.CacheHitRate = pe.getCacheHitRate(ctx, instance)
	case ServiceTypeObjectStore:
		metrics.RequestRate = pe.getRequestRate(ctx, instance)
	case ServiceTypeQueue:
		metrics.MessageRate = pe.getMessageRate(ctx, instance)
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
		fmt.Fprintf(w, "soholink_service_cpu_usage_percent{%s} %.2f\n", labels, metrics.CPUUsagePercent)

		// Memory metrics
		fmt.Fprintf(w, "# HELP soholink_service_memory_usage_percent Memory usage percentage\n")
		fmt.Fprintf(w, "# TYPE soholink_service_memory_usage_percent gauge\n")
		fmt.Fprintf(w, "soholink_service_memory_usage_percent{%s} %.2f\n", labels, metrics.MemoryUsagePercent)

		fmt.Fprintf(w, "# HELP soholink_service_memory_usage_bytes Memory usage in bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_service_memory_usage_bytes gauge\n")
		fmt.Fprintf(w, "soholink_service_memory_usage_bytes{%s} %d\n", labels, metrics.MemoryUsageMB*1024*1024)

		// Network metrics
		fmt.Fprintf(w, "# HELP soholink_service_network_rx_bytes Network received bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_service_network_rx_bytes counter\n")
		fmt.Fprintf(w, "soholink_service_network_rx_bytes{%s} %d\n", labels, metrics.NetworkInBytes)

		fmt.Fprintf(w, "# HELP soholink_service_network_tx_bytes Network transmitted bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_service_network_tx_bytes counter\n")
		fmt.Fprintf(w, "soholink_service_network_tx_bytes{%s} %d\n", labels, metrics.NetworkOutBytes)

		// Disk I/O metrics
		fmt.Fprintf(w, "# HELP soholink_service_disk_read_bytes Disk read bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_service_disk_read_bytes counter\n")
		fmt.Fprintf(w, "soholink_service_disk_read_bytes{%s} %d\n", labels, metrics.DiskReadBytes)

		fmt.Fprintf(w, "# HELP soholink_service_disk_write_bytes Disk write bytes\n")
		fmt.Fprintf(w, "# TYPE soholink_service_disk_write_bytes counter\n")
		fmt.Fprintf(w, "soholink_service_disk_write_bytes{%s} %d\n", labels, metrics.DiskWriteBytes)

		// Uptime
		fmt.Fprintf(w, "# HELP soholink_service_uptime_seconds Service uptime in seconds\n")
		fmt.Fprintf(w, "# TYPE soholink_service_uptime_seconds gauge\n")
		fmt.Fprintf(w, "soholink_service_uptime_seconds{%s} %.0f\n", labels, metrics.Uptime.Seconds())

		// Service-specific metrics
		if metrics.ConnectionCount > 0 {
			fmt.Fprintf(w, "# HELP soholink_service_connections Active database connections\n")
			fmt.Fprintf(w, "# TYPE soholink_service_connections gauge\n")
			fmt.Fprintf(w, "soholink_service_connections{%s} %d\n", labels, metrics.ConnectionCount)
		}

		if metrics.QueryRate > 0 {
			fmt.Fprintf(w, "# HELP soholink_service_query_rate Queries per second\n")
			fmt.Fprintf(w, "# TYPE soholink_service_query_rate gauge\n")
			fmt.Fprintf(w, "soholink_service_query_rate{%s} %.2f\n", labels, metrics.QueryRate)
		}

		if metrics.CacheHitRate > 0 {
			fmt.Fprintf(w, "# HELP soholink_service_cache_hit_rate Cache hit rate percentage\n")
			fmt.Fprintf(w, "# TYPE soholink_service_cache_hit_rate gauge\n")
			fmt.Fprintf(w, "soholink_service_cache_hit_rate{%s} %.2f\n", labels, metrics.CacheHitRate)
		}

		if metrics.RequestRate > 0 {
			fmt.Fprintf(w, "# HELP soholink_service_request_rate Requests per second\n")
			fmt.Fprintf(w, "# TYPE soholink_service_request_rate gauge\n")
			fmt.Fprintf(w, "soholink_service_request_rate{%s} %.2f\n", labels, metrics.RequestRate)
		}

		if metrics.MessageRate > 0 {
			fmt.Fprintf(w, "# HELP soholink_service_message_rate Messages per second\n")
			fmt.Fprintf(w, "# TYPE soholink_service_message_rate gauge\n")
			fmt.Fprintf(w, "soholink_service_message_rate{%s} %.2f\n", labels, metrics.MessageRate)
		}
	}

	// Overall catalog metrics
	instances := pe.catalog.ListInstances()
	statusCounts := make(map[string]int)
	typeCounts := make(map[ServiceType]int)

	for _, instance := range instances {
		statusCounts[instance.Status]++
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
	// PostgreSQL: SELECT count(*) FROM pg_stat_activity WHERE state = 'active'
	// MySQL: SHOW STATUS LIKE 'Threads_connected'
	// MongoDB: db.serverStatus().connections.current
	return 0
}

func (pe *PrometheusExporter) getQueryRate(ctx context.Context, instance *ServiceInstance) float64 {
	// TODO: Calculate queries per second from stats
	return 0.0
}

func (pe *PrometheusExporter) getCacheHitRate(ctx context.Context, instance *ServiceInstance) float64 {
	// TODO: Redis INFO stats - keyspace_hits / (keyspace_hits + keyspace_misses)
	return 0.0
}

func (pe *PrometheusExporter) getRequestRate(ctx context.Context, instance *ServiceInstance) float64 {
	// TODO: MinIO metrics endpoint
	return 0.0
}

func (pe *PrometheusExporter) getMessageRate(ctx context.Context, instance *ServiceInstance) float64 {
	// TODO: RabbitMQ management API - message rates
	return 0.0
}

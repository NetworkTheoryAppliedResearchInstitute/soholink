package central

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// CapacityMetrics holds a point-in-time snapshot of federated resource capacity.
type CapacityMetrics struct {
	Timestamp time.Time

	// Tenant counts
	TotalTenants  int
	ActiveTenants int // Active in last 24h

	// CPU resources
	TotalCPUCores     int
	AvailableCPUCores int
	UsedCPUCores      int

	// Storage resources
	TotalStorageGB     int64
	AvailableStorageGB int64
	UsedStorageGB      int64

	// Memory resources
	TotalMemoryGB     int64
	AvailableMemoryGB int64
	UsedMemoryGB      int64

	// GPU resources
	TotalGPUs     int
	AvailableGPUs int
	GPUModels     map[string]int // e.g. "RTX 4090": 8, "A100": 4

	// Network
	TotalBandwidthMbps int
	UsedBandwidthMbps  int

	// Job queue
	PendingJobs int
	RunningJobs int

	// Trends (compared to 24h ago)
	TenantGrowth   float64 // Percentage
	StorageGrowth  float64
	CPUUtilization float64 // Percentage (0.0–1.0)

	// Projections (based on growth rate)
	DaysUntilCPUFull     int
	DaysUntilStorageFull int
}

// CapacityAlert represents a threshold-triggered capacity warning.
type CapacityAlert struct {
	Severity string // "warning", "critical"
	Resource string // "cpu", "storage", "memory", "gpu"
	Message  string
	Metrics  CapacityMetrics
}

// CapacityMonitor periodically collects capacity metrics from all tenants
// and fires alerts when utilisation thresholds are exceeded.
type CapacityMonitor struct {
	store    *store.Store
	interval time.Duration

	// Alert thresholds (0.0–1.0)
	cpuAlertThreshold     float64
	storageAlertThreshold float64

	alertChan chan CapacityAlert
	latest    *CapacityMetrics
}

// NewCapacityMonitor creates a new monitor that checks every interval.
func NewCapacityMonitor(s *store.Store, interval time.Duration) *CapacityMonitor {
	return &CapacityMonitor{
		store:                 s,
		interval:              interval,
		cpuAlertThreshold:     0.80,
		storageAlertThreshold: 0.80,
		alertChan:             make(chan CapacityAlert, 100),
	}
}

// AlertChan returns the channel that receives capacity alerts.
func (m *CapacityMonitor) AlertChan() <-chan CapacityAlert {
	return m.alertChan
}

// GetLatestMetrics returns the most recent collected metrics snapshot.
func (m *CapacityMonitor) GetLatestMetrics() *CapacityMetrics {
	return m.latest
}

// Run starts the periodic capacity collection loop.
func (m *CapacityMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Collect initial snapshot
	metrics := m.collectMetrics(ctx)
	m.latest = &metrics
	m.checkAlerts(metrics)

	for {
		select {
		case <-ticker.C:
			metrics := m.collectMetrics(ctx)
			m.latest = &metrics
			m.checkAlerts(metrics)
		case <-ctx.Done():
			return
		}
	}
}

// collectMetrics gathers current capacity data from the store.
func (m *CapacityMonitor) collectMetrics(ctx context.Context) CapacityMetrics {
	metrics := CapacityMetrics{
		Timestamp: time.Now(),
		GPUModels: make(map[string]int),
	}

	// Query tenant counts
	total, _ := m.store.CountTenants(ctx)
	active, _ := m.store.CountActiveTenants(ctx, 24*time.Hour)
	metrics.TotalTenants = total
	metrics.ActiveTenants = active

	// Query aggregated resources
	agg, err := m.store.GetAggregateResources(ctx)
	if err == nil {
		metrics.TotalCPUCores = agg.TotalCPU
		metrics.TotalStorageGB = agg.TotalStorageGB
		metrics.TotalMemoryGB = agg.TotalMemoryGB
		metrics.UsedCPUCores = agg.UsedCPU
		metrics.UsedStorageGB = agg.UsedStorageGB
		metrics.UsedMemoryGB = agg.UsedMemoryGB
	}

	metrics.AvailableCPUCores = metrics.TotalCPUCores - metrics.UsedCPUCores
	metrics.AvailableStorageGB = metrics.TotalStorageGB - metrics.UsedStorageGB
	metrics.AvailableMemoryGB = metrics.TotalMemoryGB - metrics.UsedMemoryGB

	// CPU utilisation
	if metrics.TotalCPUCores > 0 {
		metrics.CPUUtilization = float64(metrics.UsedCPUCores) / float64(metrics.TotalCPUCores)
	}

	// Storage growth projection
	if metrics.UsedStorageGB > 0 && metrics.StorageGrowth > 0 {
		remainingGB := metrics.TotalStorageGB - metrics.UsedStorageGB
		dailyGrowthGB := float64(metrics.UsedStorageGB) * metrics.StorageGrowth
		if dailyGrowthGB > 0 {
			metrics.DaysUntilStorageFull = int(float64(remainingGB) / dailyGrowthGB)
		}
	}

	// Job queue stats
	metrics.PendingJobs, _ = m.store.CountJobsByState(ctx, "pending")
	metrics.RunningJobs, _ = m.store.CountJobsByState(ctx, "running")

	return metrics
}

// checkAlerts fires alerts when capacity thresholds are exceeded.
func (m *CapacityMonitor) checkAlerts(metrics CapacityMetrics) {
	// CPU
	if metrics.CPUUtilization > m.cpuAlertThreshold {
		select {
		case m.alertChan <- CapacityAlert{
			Severity: "warning",
			Resource: "cpu",
			Message: fmt.Sprintf("CPU utilization at %.1f%% (threshold: %.1f%%)",
				metrics.CPUUtilization*100, m.cpuAlertThreshold*100),
			Metrics: metrics,
		}:
		default:
			log.Printf("[capacity] alert channel full, dropping CPU alert")
		}
	}

	// Storage
	if metrics.TotalStorageGB > 0 {
		storageUtil := float64(metrics.UsedStorageGB) / float64(metrics.TotalStorageGB)
		if storageUtil > m.storageAlertThreshold {
			select {
			case m.alertChan <- CapacityAlert{
				Severity: "warning",
				Resource: "storage",
				Message: fmt.Sprintf("Storage utilization at %.1f%% (threshold: %.1f%%)",
					storageUtil*100, m.storageAlertThreshold*100),
				Metrics: metrics,
			}:
			default:
			}
		}
	}

	// Storage exhaustion projection
	if metrics.DaysUntilStorageFull > 0 && metrics.DaysUntilStorageFull < 30 {
		select {
		case m.alertChan <- CapacityAlert{
			Severity: "warning",
			Resource: "storage",
			Message:  fmt.Sprintf("Storage projected to be full in %d days", metrics.DaysUntilStorageFull),
			Metrics:  metrics,
		}:
		default:
		}
	}
}

// Package orchestration implements the FedScheduler — a Kubernetes-inspired
// elastic orchestrator that distributes workloads across federated SOHO nodes.
package orchestration

import (
	"time"
)

// Workload represents a user-submitted unit of work that the FedScheduler
// places onto one or more federation nodes.
type Workload struct {
	WorkloadID  string
	Name        string
	OwnerDID    string
	Type        string // "container", "vm", "function", "service"

	Spec        WorkloadSpec
	Constraints PlacementConstraints

	Replicas    int
	AutoScale   *AutoScalePolicy

	Status       string // "pending", "scheduling", "running", "scaling", "failed", "stopped"
	DesiredState string // "running", "stopped"

	HealthCheck HealthCheckConfig

	SLA *SLARef // Optional SLA contract reference

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SLARef links a workload to an SLA contract.
type SLARef struct {
	ContractID string
}

// WorkloadSpec defines the resource and runtime requirements.
type WorkloadSpec struct {
	// Compute
	CPUCores    float64 // 0.5 = 500m, 2.0 = 2 cores
	MemoryMB    int64
	GPURequired bool
	GPUModel    string // "RTX 4090", "A100", etc.

	// Storage
	DiskGB      int64
	StorageType string // "ephemeral", "persistent"

	// Network
	NetworkMbps int
	PublicIP    bool
	Ports       []PortMapping

	// Image / code
	Image      string
	Entrypoint []string
	Environment map[string]string

	// Runtime
	Timeout    time.Duration
	MaxRetries int
}

// PortMapping maps a container port to a host protocol.
type PortMapping struct {
	ContainerPort int
	HostPort      int
	Protocol      string // "tcp", "udp"
}

// PlacementConstraints control where workloads are scheduled.
type PlacementConstraints struct {
	// Geographic
	Regions        []string
	ExcludeRegions []string
	MaxLatencyMs   int

	// Resource quality
	MinProviderScore   int
	PreferredProviders []string
	ExcludedProviders  []string

	// Cost
	MaxCostPerHour int64 // Cents

	// Affinity / anti-affinity
	Affinity     *Affinity
	AntiAffinity *Affinity
}

// Affinity expresses co-location or separation preferences.
type Affinity struct {
	WorkloadLabels map[string]string
	Strength       string // "required", "preferred"
}

// AutoScalePolicy governs horizontal scaling of workload replicas.
type AutoScalePolicy struct {
	Enabled     bool
	MinReplicas int
	MaxReplicas int

	// Triggers
	TargetCPU     float64 // Scale when avg CPU > this (0.0–1.0)
	TargetMemory  float64
	TargetLatency int // Scale when p95 latency > Xms

	// Cool-down
	ScaleUpCooldown   time.Duration
	ScaleDownCooldown time.Duration
}

// HealthCheckConfig defines how the scheduler monitors replica health.
type HealthCheckConfig struct {
	Type             string // "http", "tcp", "exec"
	Endpoint         string // "/health" or port number
	IntervalSeconds  int
	TimeoutSeconds   int
	FailureThreshold int
}

// WorkloadState tracks runtime state for an active workload.
type WorkloadState struct {
	Workload   *Workload
	Placements []Placement
	Health     HealthStatus
	Metrics    WorkloadMetrics
}

// Placement records where a single replica is running.
type Placement struct {
	PlacementID     string
	WorkloadID      string
	ReplicaNum      int
	NodeDID         string
	NodeAddress     string
	Status          string // "pending", "running", "failed"
	Performance     float64
	StartedAt       time.Time
	LastHealthCheck time.Time
}

// HealthStatus is the aggregate health of a workload.
type HealthStatus struct {
	Status  string // "healthy", "degraded", "unhealthy"
	Details string
}

// WorkloadMetrics holds aggregated runtime metrics.
type WorkloadMetrics struct {
	AvgCPUPercent    float64
	AvgMemoryPercent float64
	P95LatencyMs     int
	RequestsPerSec   float64
}

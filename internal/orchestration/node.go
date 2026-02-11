package orchestration

import (
	"time"
)

// Node represents a federation node that can accept workloads.
type Node struct {
	DID     string
	Address string
	Region  string

	// Capacity
	TotalCPU          float64
	AvailableCPU      float64
	TotalMemoryMB     int64
	AvailableMemoryMB int64
	TotalDiskGB       int64
	AvailableDiskGB   int64

	// GPU
	HasGPU      bool
	GPUModel    string
	GPUMemoryMB int64

	// Network
	BandwidthMbps int
	LatencyMs     int // Measured from central SOHO

	// Pricing
	PricePerCPUHour int64 // Cents
	PricePerGBMonth int64

	// Reputation
	ReputationScore int // 0-100
	UptimePercent   float64
	FailureRate     float64 // Fraction of failed jobs

	// Status
	Status        string // "online", "busy", "offline"
	LastHeartbeat time.Time
}

// NodeQuery describes filter criteria for node discovery.
type NodeQuery struct {
	MinCPU         float64
	MinMemory      int64
	MinDisk        int64
	GPURequired    bool
	GPUModel       string
	Regions        []string
	MinReputation  int
	MaxCostPerHour int64
}

// NodeCapacity is a snapshot of a node's available resources.
type NodeCapacity struct {
	NodeDID       string
	AvailableCPU  float64
	AvailableMem  int64
	AvailableDisk int64
	ActiveJobs    int
}

// DeployRequest is sent to a node's worker API to deploy a workload replica.
type DeployRequest struct {
	PlacementID string
	WorkloadID  string
	Spec        WorkloadSpec
	HealthCheck HealthCheckConfig
}

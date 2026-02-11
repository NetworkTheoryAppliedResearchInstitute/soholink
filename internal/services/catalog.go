package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// ServiceType identifies the kind of managed service.
type ServiceType string

const (
	ServiceTypePostgres     ServiceType = "postgres"
	ServiceTypeMySQL        ServiceType = "mysql"
	ServiceTypeMongoDB      ServiceType = "mongodb"
	ServiceTypeRedis        ServiceType = "redis"
	ServiceTypeObjectStore  ServiceType = "object_storage"
	ServiceTypeMessageQueue ServiceType = "message_queue"
)

// ServiceStatus tracks the lifecycle of a provisioned service.
type ServiceStatus string

const (
	StatusProvisioning ServiceStatus = "provisioning"
	StatusRunning      ServiceStatus = "running"
	StatusStopped      ServiceStatus = "stopped"
	StatusFailed       ServiceStatus = "failed"
	StatusTerminated   ServiceStatus = "terminated"
)

// ServiceInstance represents a provisioned managed service.
type ServiceInstance struct {
	InstanceID   string
	OwnerDID     string
	ServiceType  ServiceType
	Name         string
	Plan         string
	Status       ServiceStatus
	NodeDID      string
	Endpoint     string
	Port         int
	Credentials  ServiceCredentials
	Config       map[string]string
	Metrics      ServiceMetrics
	SLARef       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ServiceCredentials holds connection information for a service instance.
type ServiceCredentials struct {
	Username string
	Password string
	Database string
	Token    string
	Region   string
	Bucket   string
	QueueURL string
}

// ServiceMetrics tracks resource utilization for a service instance.
type ServiceMetrics struct {
	CPUPercent     float64
	MemoryPercent  float64
	MemoryUsageMB  uint64
	StorageUsedGB  float64
	StorageTotalGB float64
	Connections    int
	QPS            float64
	AvgLatencyMs   float64
	Uptime         time.Duration
}

// ServicePlan defines resource limits and pricing for a service tier.
type ServicePlan struct {
	PlanID      string
	ServiceType ServiceType
	Name        string
	CPUCores    float64
	MemoryMB    int64
	StorageGB   int64
	MaxConns    int
	PricePerDay int64
	HA          bool
	Replicas    int
}

// ProvisionRequest describes a request to create a new managed service.
type ProvisionRequest struct {
	OwnerDID    string
	ServiceType ServiceType
	Name        string
	Plan        string
	Region      string
	Config      map[string]string
}

// Catalog manages the lifecycle of managed service instances.
type Catalog struct {
	store *store.Store

	mu        sync.RWMutex
	instances map[string]*ServiceInstance
	plans     map[string]ServicePlan

	// Provisioners for each service type
	provisioners map[ServiceType]Provisioner
}

// Provisioner creates and manages a specific type of managed service.
type Provisioner interface {
	Provision(ctx context.Context, req ProvisionRequest, plan ServicePlan) (*ServiceInstance, error)
	Deprovision(ctx context.Context, instance *ServiceInstance) error
	GetMetrics(ctx context.Context, instance *ServiceInstance) (*ServiceMetrics, error)
	HealthCheck(ctx context.Context, instance *ServiceInstance) (bool, error)
}

// NewCatalog creates a new service catalog with default plans.
func NewCatalog(s *store.Store) *Catalog {
	c := &Catalog{
		store:        s,
		instances:    make(map[string]*ServiceInstance),
		plans:        make(map[string]ServicePlan),
		provisioners: make(map[ServiceType]Provisioner),
	}
	c.registerDefaultPlans()
	return c
}

// RegisterProvisioner registers a provisioner for a service type.
func (c *Catalog) RegisterProvisioner(stype ServiceType, p Provisioner) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.provisioners[stype] = p
}

// Provision creates a new managed service instance.
func (c *Catalog) Provision(ctx context.Context, req ProvisionRequest) (*ServiceInstance, error) {
	plan, ok := c.plans[req.Plan]
	if !ok {
		return nil, fmt.Errorf("unknown plan: %s", req.Plan)
	}

	c.mu.RLock()
	prov, ok := c.provisioners[req.ServiceType]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no provisioner for service type: %s", req.ServiceType)
	}

	instance, err := prov.Provision(ctx, req, plan)
	if err != nil {
		return nil, fmt.Errorf("provision failed: %w", err)
	}

	c.mu.Lock()
	c.instances[instance.InstanceID] = instance
	c.mu.Unlock()

	// Persist to store
	_ = c.store.CreateServiceInstance(ctx, &store.ServiceInstanceRow{
		InstanceID:  instance.InstanceID,
		OwnerDID:    instance.OwnerDID,
		ServiceType: string(instance.ServiceType),
		Name:        instance.Name,
		Plan:        instance.Plan,
		Status:      string(instance.Status),
		NodeDID:     instance.NodeDID,
		Endpoint:    instance.Endpoint,
		Port:        instance.Port,
		CreatedAt:   instance.CreatedAt,
		UpdatedAt:   instance.UpdatedAt,
	})

	log.Printf("[services] provisioned %s instance %s for %s", req.ServiceType, instance.InstanceID, req.OwnerDID)
	return instance, nil
}

// Deprovision terminates a managed service instance.
func (c *Catalog) Deprovision(ctx context.Context, instanceID string) error {
	c.mu.Lock()
	instance, ok := c.instances[instanceID]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	prov, provOK := c.provisioners[instance.ServiceType]
	c.mu.Unlock()

	if provOK {
		if err := prov.Deprovision(ctx, instance); err != nil {
			return fmt.Errorf("deprovision failed: %w", err)
		}
	}

	c.mu.Lock()
	instance.Status = StatusTerminated
	instance.UpdatedAt = time.Now()
	c.mu.Unlock()

	_ = c.store.UpdateServiceInstanceStatus(ctx, instanceID, string(StatusTerminated))
	log.Printf("[services] deprovisioned instance %s", instanceID)
	return nil
}

// GetInstance returns a service instance by ID.
func (c *Catalog) GetInstance(instanceID string) *ServiceInstance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instances[instanceID]
}

// ListInstances returns all instances for a given owner.
func (c *Catalog) ListInstances(ownerDID string) []*ServiceInstance {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*ServiceInstance
	for _, inst := range c.instances {
		if inst.OwnerDID == ownerDID {
			result = append(result, inst)
		}
	}
	return result
}

// GetPlans returns all available plans for a service type.
func (c *Catalog) GetPlans(stype ServiceType) []ServicePlan {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []ServicePlan
	for _, plan := range c.plans {
		if plan.ServiceType == stype {
			result = append(result, plan)
		}
	}
	return result
}

// HealthCheckLoop periodically checks the health of all running instances.
func (c *Catalog) HealthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkAllInstances(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Catalog) checkAllInstances(ctx context.Context) {
	c.mu.RLock()
	instances := make([]*ServiceInstance, 0, len(c.instances))
	for _, inst := range c.instances {
		if inst.Status == StatusRunning {
			instances = append(instances, inst)
		}
	}
	c.mu.RUnlock()

	for _, inst := range instances {
		c.mu.RLock()
		prov, ok := c.provisioners[inst.ServiceType]
		c.mu.RUnlock()
		if !ok {
			continue
		}

		healthy, err := prov.HealthCheck(ctx, inst)
		if err != nil || !healthy {
			log.Printf("[services] instance %s unhealthy: %v", inst.InstanceID, err)
			c.mu.Lock()
			inst.Status = StatusFailed
			inst.UpdatedAt = time.Now()
			c.mu.Unlock()
		}
	}
}

func (c *Catalog) registerDefaultPlans() {
	// PostgreSQL plans
	c.plans["pg-starter"] = ServicePlan{
		PlanID: "pg-starter", ServiceType: ServiceTypePostgres,
		Name: "Starter", CPUCores: 1, MemoryMB: 1024, StorageGB: 10,
		MaxConns: 25, PricePerDay: 100, HA: false, Replicas: 1,
	}
	c.plans["pg-standard"] = ServicePlan{
		PlanID: "pg-standard", ServiceType: ServiceTypePostgres,
		Name: "Standard", CPUCores: 2, MemoryMB: 4096, StorageGB: 50,
		MaxConns: 100, PricePerDay: 500, HA: true, Replicas: 2,
	}
	c.plans["pg-premium"] = ServicePlan{
		PlanID: "pg-premium", ServiceType: ServiceTypePostgres,
		Name: "Premium", CPUCores: 4, MemoryMB: 16384, StorageGB: 200,
		MaxConns: 500, PricePerDay: 2000, HA: true, Replicas: 3,
	}

	// Object Storage plans
	c.plans["s3-starter"] = ServicePlan{
		PlanID: "s3-starter", ServiceType: ServiceTypeObjectStore,
		Name: "Starter", StorageGB: 50, PricePerDay: 50,
	}
	c.plans["s3-standard"] = ServicePlan{
		PlanID: "s3-standard", ServiceType: ServiceTypeObjectStore,
		Name: "Standard", StorageGB: 500, PricePerDay: 300,
	}

	// Message Queue plans
	c.plans["mq-starter"] = ServicePlan{
		PlanID: "mq-starter", ServiceType: ServiceTypeMessageQueue,
		Name: "Starter", MemoryMB: 256, PricePerDay: 50,
	}
	c.plans["mq-standard"] = ServicePlan{
		PlanID: "mq-standard", ServiceType: ServiceTypeMessageQueue,
		Name: "Standard", MemoryMB: 1024, PricePerDay: 200,
	}
}

package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"
)

// PostgresProvisioner manages PostgreSQL-as-a-Service instances via Docker.
// It provisions isolated PostgreSQL containers on federation nodes.
type PostgresProvisioner struct {
	dockerClient *DockerClient
}

// NewPostgresProvisioner creates a new PostgreSQL provisioner.
func NewPostgresProvisioner(dockerEndpoint string) *PostgresProvisioner {
	return &PostgresProvisioner{
		dockerClient: NewDockerClient(dockerEndpoint),
	}
}

// Provision creates a new PostgreSQL service instance via Docker.
func (p *PostgresProvisioner) Provision(ctx context.Context, req ProvisionRequest, plan ServicePlan) (*ServiceInstance, error) {
	instanceID := generateServiceID("pg")
	dbName := fmt.Sprintf("db_%s", instanceID[:16])
	username := fmt.Sprintf("user_%s", instanceID[:12])
	password := generatePassword(24)
	containerName := fmt.Sprintf("postgres-%s", instanceID)

	// Create Docker container with PostgreSQL
	containerConfig := map[string]interface{}{
		"Image": "postgres:15-alpine",
		"Env": []string{
			fmt.Sprintf("POSTGRES_DB=%s", dbName),
			fmt.Sprintf("POSTGRES_USER=%s", username),
			fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			fmt.Sprintf("POSTGRES_INITDB_ARGS=--encoding=UTF8 --lc-collate=C --lc-ctype=C"),
		},
		"HostConfig": map[string]interface{}{
			"Memory":     plan.MemoryMB * 1024 * 1024,
			"NanoCpus":   plan.CPUCores * 1e9,
			"PortBindings": map[string][]map[string]string{
				"5432/tcp": {{
					"HostIp":   "0.0.0.0",
					"HostPort": "0", // Auto-assign port
				}},
			},
			"Binds": []string{
				fmt.Sprintf("postgres-data-%s:/var/lib/postgresql/data", instanceID),
			},
		},
		"Labels": map[string]string{
			"soholink.service.type":       "postgres",
			"soholink.service.id":         instanceID,
			"soholink.service.owner":      req.OwnerDID,
			"soholink.service.plan":       req.Plan,
		},
	}

	// Create and start container
	containerID, err := p.dockerClient.CreateContainer(ctx, containerName, containerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := p.dockerClient.StartContainer(ctx, containerID); err != nil {
		p.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get assigned port
	inspect, err := p.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		p.dockerClient.StopContainer(ctx, containerID, 10)
		p.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	hostPort := extractHostPort(inspect, "5432/tcp")
	if hostPort == 0 {
		hostPort = 5432
	}

	instance := &ServiceInstance{
		InstanceID:  instanceID,
		OwnerDID:    req.OwnerDID,
		ServiceType: ServiceTypePostgres,
		Name:        req.Name,
		Plan:        req.Plan,
		Status:      StatusRunning,
		Port:        hostPort,
		Credentials: ServiceCredentials{
			Username: username,
			Password: password,
			Database: dbName,
		},
		Config: map[string]string{
			"container_id":   containerID,
			"container_name": containerName,
			"image":          "postgres:15-alpine",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if plan.HA && plan.Replicas > 1 {
		instance.Config["replication"] = "streaming"
		instance.Config["replicas"] = fmt.Sprintf("%d", plan.Replicas)
	}

	log.Printf("[postgres] provisioned instance %s (container=%s, db=%s, port=%d, plan=%s)",
		instanceID, containerID[:12], dbName, hostPort, req.Plan)

	return instance, nil
}

// Deprovision terminates a PostgreSQL service instance.
func (p *PostgresProvisioner) Deprovision(ctx context.Context, instance *ServiceInstance) error {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return fmt.Errorf("container_id not found in instance config")
	}

	// Stop container gracefully (30 second timeout for connections to drain)
	if err := p.dockerClient.StopContainer(ctx, containerID, 30); err != nil {
		log.Printf("[postgres] warning: failed to stop container %s: %v", containerID[:12], err)
	}

	// Remove container and optionally its volumes
	if err := p.dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	log.Printf("[postgres] deprovisioned instance %s (container=%s)", instance.InstanceID, containerID[:12])
	return nil
}

// GetMetrics retrieves current metrics for a PostgreSQL instance.
func (p *PostgresProvisioner) GetMetrics(ctx context.Context, instance *ServiceInstance) (*ServiceMetrics, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found in instance config")
	}

	stats, err := p.dockerClient.GetContainerStats(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}

	// Calculate CPU percentage
	cpuDelta := stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage
	systemDelta := stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (float64(cpuDelta) / float64(systemDelta)) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// Calculate memory percentage
	memPercent := 0.0
	if stats.MemoryStats.Limit > 0 {
		memPercent = (float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit)) * 100.0
	}

	return &ServiceMetrics{
		CPUPercent:    cpuPercent,
		MemoryPercent: memPercent,
		MemoryUsageMB: stats.MemoryStats.Usage / (1024 * 1024),
		Connections:   0, // Would need to query PostgreSQL directly
		QPS:           0, // Would need to query pg_stat_database
		AvgLatencyMs:  0, // Would need to query PostgreSQL directly
		Uptime:        time.Since(instance.CreatedAt),
	}, nil
}

// HealthCheck verifies that a PostgreSQL instance is responsive.
func (p *PostgresProvisioner) HealthCheck(ctx context.Context, instance *ServiceInstance) (bool, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return false, fmt.Errorf("container_id not found in instance config")
	}

	// Check if container is running
	inspect, err := p.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	running, _ := inspect["State"].(map[string]interface{})["Running"].(bool)
	if !running {
		return false, nil
	}

	// Execute health check inside container
	execConfig := map[string]interface{}{
		"Cmd": []string{"pg_isready", "-U", instance.Credentials.Username},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := p.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create exec: %w", err)
	}

	output, err := p.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return false, fmt.Errorf("failed to execute health check: %w", err)
	}

	// pg_isready returns "accepting connections" when healthy
	healthy := strings.Contains(output, "accepting connections")
	return healthy, nil
}

func generateServiceID(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func generatePassword(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

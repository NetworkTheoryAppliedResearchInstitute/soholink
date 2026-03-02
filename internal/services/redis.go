package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// RedisProvisioner manages Redis-as-a-Service instances via Docker.
// It provisions isolated Redis containers on federation nodes.
type RedisProvisioner struct {
	dockerClient *DockerClient
}

// NewRedisProvisioner creates a new Redis provisioner.
func NewRedisProvisioner(dockerEndpoint string) *RedisProvisioner {
	return &RedisProvisioner{
		dockerClient: NewDockerClient(dockerEndpoint),
	}
}

// Provision creates a new Redis service instance via Docker.
func (r *RedisProvisioner) Provision(ctx context.Context, req ProvisionRequest, plan ServicePlan) (*ServiceInstance, error) {
	instanceID := generateServiceID("redis")
	password := generatePassword(32)
	containerName := fmt.Sprintf("redis-%s", instanceID)

	// Build Redis command with authentication
	redisCmd := []string{
		"redis-server",
		"--requirepass", password,
		"--appendonly", "yes", // Enable AOF persistence
		"--appendfsync", "everysec",
	}

	// Add maxmemory policy for cache use cases
	if config, ok := req.Config["maxmemory_policy"]; ok {
		redisCmd = append(redisCmd, "--maxmemory-policy", config)
	} else {
		redisCmd = append(redisCmd, "--maxmemory-policy", "allkeys-lru")
	}

	// Set maxmemory to 80% of container memory limit
	maxMemoryMB := uint64(float64(plan.MemoryMB) * 0.8)
	redisCmd = append(redisCmd, "--maxmemory", fmt.Sprintf("%dmb", maxMemoryMB))

	// Create Docker container with Redis
	containerConfig := map[string]interface{}{
		"Image": "redis:7-alpine",
		"Cmd":   redisCmd,
		"HostConfig": map[string]interface{}{
			"Memory":   plan.MemoryMB * 1024 * 1024,
			"NanoCpus": plan.CPUCores * 1e9,
			"PortBindings": map[string][]map[string]string{
				"6379/tcp": {{
					"HostIp":   "0.0.0.0",
					"HostPort": "0", // Auto-assign port
				}},
			},
			"Binds": []string{
				fmt.Sprintf("redis-data-%s:/data", instanceID),
			},
		},
		"Labels": map[string]string{
			"soholink.service.type":  "redis",
			"soholink.service.id":    instanceID,
			"soholink.service.owner": req.OwnerDID,
			"soholink.service.plan":  req.Plan,
		},
	}

	// Create and start container
	containerID, err := r.dockerClient.CreateContainer(ctx, containerName, containerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := r.dockerClient.StartContainer(ctx, containerID); err != nil {
		r.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get assigned port
	inspect, err := r.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		r.dockerClient.StopContainer(ctx, containerID, 10)
		r.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	hostPort := extractHostPort(inspect, "6379/tcp")
	if hostPort == 0 {
		hostPort = 6379
	}

	instance := &ServiceInstance{
		InstanceID:  instanceID,
		OwnerDID:    req.OwnerDID,
		ServiceType: ServiceTypeRedis,
		Name:        req.Name,
		Plan:        req.Plan,
		Status:      StatusRunning,
		Port:        hostPort,
		Credentials: ServiceCredentials{
			Password: password,
		},
		Config: map[string]string{
			"container_id":      containerID,
			"container_name":    containerName,
			"image":             "redis:7-alpine",
			"maxmemory_policy":  req.Config["maxmemory_policy"],
			"persistence":       "aof",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	log.Printf("[redis] provisioned instance %s (container=%s, port=%d, plan=%s)",
		instanceID, containerID[:12], hostPort, req.Plan)

	return instance, nil
}

// Deprovision terminates a Redis service instance.
func (r *RedisProvisioner) Deprovision(ctx context.Context, instance *ServiceInstance) error {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return fmt.Errorf("container_id not found in instance config")
	}

	// Stop container gracefully (10 second timeout for Redis to save)
	if err := r.dockerClient.StopContainer(ctx, containerID, 10); err != nil {
		log.Printf("[redis] warning: failed to stop container %s: %v", containerID[:12], err)
	}

	// Remove container and volumes
	if err := r.dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	log.Printf("[redis] deprovisioned instance %s (container=%s)", instance.InstanceID, containerID[:12])
	return nil
}

// GetMetrics retrieves current metrics for a Redis instance.
func (r *RedisProvisioner) GetMetrics(ctx context.Context, instance *ServiceInstance) (*ServiceMetrics, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found in instance config")
	}

	stats, err := r.dockerClient.GetContainerStats(ctx, containerID)
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

	// Get Redis-specific metrics via INFO command
	connections, qps := r.getRedisInfo(ctx, instance)

	return &ServiceMetrics{
		CPUPercent:    cpuPercent,
		MemoryPercent: memPercent,
		MemoryUsageMB: stats.MemoryStats.Usage / (1024 * 1024),
		Connections:   connections,
		QPS:           qps,
		AvgLatencyMs:  0, // Would need Redis latency monitoring
		Uptime:        time.Since(instance.CreatedAt),
	}, nil
}

// HealthCheck verifies that a Redis instance is responsive.
func (r *RedisProvisioner) HealthCheck(ctx context.Context, instance *ServiceInstance) (bool, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return false, fmt.Errorf("container_id not found in instance config")
	}

	// Check if container is running
	inspect, err := r.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	running, _ := inspect["State"].(map[string]interface{})["Running"].(bool)
	if !running {
		return false, nil
	}

	// Execute PING command to verify Redis is responsive
	execConfig := map[string]interface{}{
		"Cmd": []string{
			"redis-cli",
			"-a", instance.Credentials.Password,
			"PING",
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := r.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create exec: %w", err)
	}

	output, err := r.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return false, fmt.Errorf("failed to execute health check: %w", err)
	}

	// Redis responds with "PONG" when healthy
	healthy := strings.Contains(output, "PONG")
	return healthy, nil
}

// getRedisInfo retrieves Redis INFO statistics.
func (r *RedisProvisioner) getRedisInfo(ctx context.Context, instance *ServiceInstance) (int, float64) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return 0, 0
	}

	// Execute INFO command
	execConfig := map[string]interface{}{
		"Cmd": []string{
			"redis-cli",
			"-a", instance.Credentials.Password,
			"INFO", "stats",
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := r.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return 0, 0
	}

	output, err := r.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return 0, 0
	}

	// Parse INFO output for connections and ops/sec
	connections := 0
	qps := 0.0

	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "connected_clients:") {
			fmt.Sscanf(line, "connected_clients:%d", &connections)
		}
		if strings.HasPrefix(line, "instantaneous_ops_per_sec:") {
			fmt.Sscanf(line, "instantaneous_ops_per_sec:%f", &qps)
		}
	}

	return connections, qps
}

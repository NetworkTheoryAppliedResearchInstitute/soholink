package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// MongoDBProvisioner manages MongoDB-as-a-Service instances via Docker.
// It provisions isolated MongoDB containers on federation nodes.
type MongoDBProvisioner struct {
	dockerClient *DockerClient
}

// NewMongoDBProvisioner creates a new MongoDB provisioner.
func NewMongoDBProvisioner(dockerEndpoint string) *MongoDBProvisioner {
	return &MongoDBProvisioner{
		dockerClient: NewDockerClient(dockerEndpoint),
	}
}

// Provision creates a new MongoDB service instance via Docker.
func (mo *MongoDBProvisioner) Provision(ctx context.Context, req ProvisionRequest, plan ServicePlan) (*ServiceInstance, error) {
	instanceID := generateServiceID("mongo")
	dbName := fmt.Sprintf("db_%s", instanceID[:16])
	username := fmt.Sprintf("user_%s", instanceID[:12])
	password := generatePassword(24)
	rootPassword := generatePassword(32)
	containerName := fmt.Sprintf("mongodb-%s", instanceID)

	// Create Docker container with MongoDB
	containerConfig := map[string]interface{}{
		"Image": "mongo:7.0",
		"Env": []string{
			fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=root"),
			fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", rootPassword),
			fmt.Sprintf("MONGO_INITDB_DATABASE=%s", dbName),
		},
		"Cmd": []string{
			"--auth",
			"--wiredTigerCacheSizeGB", fmt.Sprintf("%.1f", float64(plan.MemoryMB)*0.6/1024), // 60% of memory for cache
		},
		"HostConfig": map[string]interface{}{
			"Memory":   plan.MemoryMB * 1024 * 1024,
			"NanoCpus": plan.CPUCores * 1e9,
			"PortBindings": map[string][]map[string]string{
				"27017/tcp": {{
					"HostIp":   "0.0.0.0",
					"HostPort": "0", // Auto-assign port
				}},
			},
			"Binds": []string{
				fmt.Sprintf("mongodb-data-%s:/data/db", instanceID),
				fmt.Sprintf("mongodb-config-%s:/data/configdb", instanceID),
			},
		},
		"Labels": map[string]string{
			"soholink.service.type":  "mongodb",
			"soholink.service.id":    instanceID,
			"soholink.service.owner": req.OwnerDID,
			"soholink.service.plan":  req.Plan,
		},
	}

	// Create and start container
	containerID, err := mo.dockerClient.CreateContainer(ctx, containerName, containerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := mo.dockerClient.StartContainer(ctx, containerID); err != nil {
		mo.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get assigned port
	inspect, err := mo.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		mo.dockerClient.StopContainer(ctx, containerID, 10)
		mo.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	hostPort := extractHostPort(inspect, "27017/tcp")
	if hostPort == 0 {
		hostPort = 27017
	}

	// Wait for MongoDB to be ready before creating user
	time.Sleep(5 * time.Second)

	// Create application user with database-specific permissions
	createUserCmd := fmt.Sprintf(
		`db.getSiblingDB('%s').createUser({user: '%s', pwd: '%s', roles: [{role: 'readWrite', db: '%s'}]})`,
		dbName, username, password, dbName,
	)

	execConfig := map[string]interface{}{
		"Cmd": []string{
			"mongosh",
			fmt.Sprintf("mongodb://root:%s@localhost:27017/admin", rootPassword),
			"--quiet",
			"--eval", createUserCmd,
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, _ := mo.dockerClient.CreateExec(ctx, containerID, execConfig)
	if execID != "" {
		mo.dockerClient.StartExec(ctx, execID)
	}

	instance := &ServiceInstance{
		InstanceID:  instanceID,
		OwnerDID:    req.OwnerDID,
		ServiceType: ServiceTypeMongoDB,
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
			"image":          "mongo:7.0",
			"root_password":  rootPassword,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if plan.HA && plan.Replicas > 1 {
		instance.Config["replica_set"] = fmt.Sprintf("rs-%s", instanceID[:8])
		instance.Config["replicas"] = fmt.Sprintf("%d", plan.Replicas)
	}

	log.Printf("[mongodb] provisioned instance %s (container=%s, db=%s, port=%d, plan=%s)",
		instanceID, containerID[:12], dbName, hostPort, req.Plan)

	return instance, nil
}

// Deprovision terminates a MongoDB service instance.
func (mo *MongoDBProvisioner) Deprovision(ctx context.Context, instance *ServiceInstance) error {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return fmt.Errorf("container_id not found in instance config")
	}

	// Stop container gracefully (30 second timeout for connections to drain)
	if err := mo.dockerClient.StopContainer(ctx, containerID, 30); err != nil {
		log.Printf("[mongodb] warning: failed to stop container %s: %v", containerID[:12], err)
	}

	// Remove container and volumes
	if err := mo.dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	log.Printf("[mongodb] deprovisioned instance %s (container=%s)", instance.InstanceID, containerID[:12])
	return nil
}

// GetMetrics retrieves current metrics for a MongoDB instance.
func (mo *MongoDBProvisioner) GetMetrics(ctx context.Context, instance *ServiceInstance) (*ServiceMetrics, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found in instance config")
	}

	stats, err := mo.dockerClient.GetContainerStats(ctx, containerID)
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

	// Get MongoDB-specific metrics
	connections, qps := mo.getMongoDBServerStatus(ctx, instance)

	return &ServiceMetrics{
		CPUPercent:    cpuPercent,
		MemoryPercent: memPercent,
		MemoryUsageMB: stats.MemoryStats.Usage / (1024 * 1024),
		Connections:   connections,
		QPS:           qps,
		AvgLatencyMs:  0, // Would need to query serverStatus.opcounters
		Uptime:        time.Since(instance.CreatedAt),
	}, nil
}

// HealthCheck verifies that a MongoDB instance is responsive.
func (mo *MongoDBProvisioner) HealthCheck(ctx context.Context, instance *ServiceInstance) (bool, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return false, fmt.Errorf("container_id not found in instance config")
	}

	// Check if container is running
	inspect, err := mo.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	running, _ := inspect["State"].(map[string]interface{})["Running"].(bool)
	if !running {
		return false, nil
	}

	// Execute ping command to verify MongoDB is responsive
	rootPassword := instance.Config["root_password"]
	execConfig := map[string]interface{}{
		"Cmd": []string{
			"mongosh",
			fmt.Sprintf("mongodb://root:%s@localhost:27017/admin", rootPassword),
			"--quiet",
			"--eval", "db.adminCommand('ping')",
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := mo.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create exec: %w", err)
	}

	output, err := mo.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return false, fmt.Errorf("failed to execute health check: %w", err)
	}

	// MongoDB ping responds with ok: 1 when healthy
	healthy := strings.Contains(output, "ok: 1")
	return healthy, nil
}

// getMongoDBServerStatus retrieves MongoDB serverStatus metrics.
func (mo *MongoDBProvisioner) getMongoDBServerStatus(ctx context.Context, instance *ServiceInstance) (int, float64) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return 0, 0
	}

	rootPassword := instance.Config["root_password"]

	// Execute serverStatus command
	execConfig := map[string]interface{}{
		"Cmd": []string{
			"mongosh",
			fmt.Sprintf("mongodb://root:%s@localhost:27017/admin", rootPassword),
			"--quiet",
			"--eval", "JSON.stringify(db.serverStatus())",
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := mo.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return 0, 0
	}

	output, err := mo.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return 0, 0
	}

	// Parse serverStatus output
	connections := 0
	qps := 0.0

	// Simple parsing for connections (would use JSON in production)
	if strings.Contains(output, "\"current\":") {
		parts := strings.Split(output, "\"current\":")
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &connections)
		}
	}

	// Would calculate QPS from opcounters in production
	return connections, qps
}

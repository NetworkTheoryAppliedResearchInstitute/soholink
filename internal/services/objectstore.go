package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// ObjectStoreProvisioner manages S3-compatible Object Storage instances via Docker.
// It provisions isolated MinIO containers on federation nodes.
type ObjectStoreProvisioner struct {
	dockerClient *DockerClient
}

// NewObjectStoreProvisioner creates a new object storage provisioner.
func NewObjectStoreProvisioner(dockerEndpoint string) *ObjectStoreProvisioner {
	return &ObjectStoreProvisioner{
		dockerClient: NewDockerClient(dockerEndpoint),
	}
}

// Provision creates a new S3-compatible object storage instance via Docker.
func (o *ObjectStoreProvisioner) Provision(ctx context.Context, req ProvisionRequest, plan ServicePlan) (*ServiceInstance, error) {
	instanceID := generateServiceID("s3")
	accessKey := generatePassword(20)
	secretKey := generatePassword(40)
	bucket := fmt.Sprintf("shl-%s", instanceID[:16])
	containerName := fmt.Sprintf("minio-%s", instanceID)

	region := req.Region
	if region == "" {
		region = "us-east-1"
	}

	// Create Docker container with MinIO
	containerConfig := map[string]interface{}{
		"Image": "minio/minio:latest",
		"Env": []string{
			fmt.Sprintf("MINIO_ROOT_USER=%s", accessKey),
			fmt.Sprintf("MINIO_ROOT_PASSWORD=%s", secretKey),
			fmt.Sprintf("MINIO_REGION=%s", region),
		},
		"Cmd": []string{
			"server",
			"/data",
			"--console-address", ":9001",
		},
		"HostConfig": map[string]interface{}{
			"Memory":   plan.MemoryMB * 1024 * 1024,
			"NanoCpus": plan.CPU * 1000000000,
			"PortBindings": map[string][]map[string]string{
				"9000/tcp": {{
					"HostIp":   "0.0.0.0",
					"HostPort": "0", // Auto-assign port
				}},
				"9001/tcp": {{
					"HostIp":   "0.0.0.0",
					"HostPort": "0", // Auto-assign console port
				}},
			},
			"Binds": []string{
				fmt.Sprintf("minio-data-%s:/data", instanceID),
			},
		},
		"Labels": map[string]string{
			"soholink.service.type":  "objectstore",
			"soholink.service.id":    instanceID,
			"soholink.service.owner": req.OwnerDID,
			"soholink.service.plan":  req.Plan,
		},
	}

	// Create and start container
	containerID, err := o.dockerClient.CreateContainer(ctx, containerName, containerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := o.dockerClient.StartContainer(ctx, containerID); err != nil {
		o.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get assigned ports
	inspect, err := o.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		o.dockerClient.StopContainer(ctx, containerID, 10)
		o.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	hostPort := extractHostPort(inspect, "9000/tcp")
	if hostPort == 0 {
		hostPort = 9000
	}

	consolePort := extractHostPort(inspect, "9001/tcp")

	// Wait for MinIO to be ready
	time.Sleep(3 * time.Second)

	// Create default bucket using mc (MinIO Client)
	createBucketCmd := map[string]interface{}{
		"Cmd": []string{
			"sh", "-c",
			fmt.Sprintf("mc alias set local http://localhost:9000 %s %s && mc mb local/%s", accessKey, secretKey, bucket),
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, _ := o.dockerClient.CreateExec(ctx, containerID, createBucketCmd)
	if execID != "" {
		o.dockerClient.StartExec(ctx, execID)
	}

	instance := &ServiceInstance{
		InstanceID:  instanceID,
		OwnerDID:    req.OwnerDID,
		ServiceType: ServiceTypeObjectStore,
		Name:        req.Name,
		Plan:        req.Plan,
		Status:      StatusRunning,
		Port:        hostPort,
		Credentials: ServiceCredentials{
			Username: accessKey,
			Password: secretKey,
			Bucket:   bucket,
			Region:   region,
		},
		Config: map[string]string{
			"container_id":     containerID,
			"container_name":   containerName,
			"image":            "minio/minio:latest",
			"storage_quota_gb": fmt.Sprintf("%d", plan.StorageGB),
			"console_port":     fmt.Sprintf("%d", consolePort),
			"versioning":       "disabled",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if req.Config != nil {
		for k, v := range req.Config {
			instance.Config[k] = v
		}
	}

	log.Printf("[objectstore] provisioned instance %s (container=%s, bucket=%s, port=%d, quota=%dGB)",
		instanceID, containerID[:12], bucket, hostPort, plan.StorageGB)

	return instance, nil
}

// Deprovision terminates an object storage instance.
func (o *ObjectStoreProvisioner) Deprovision(ctx context.Context, instance *ServiceInstance) error {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return fmt.Errorf("container_id not found in instance config")
	}

	// Stop container gracefully
	if err := o.dockerClient.StopContainer(ctx, containerID, 10); err != nil {
		log.Printf("[objectstore] warning: failed to stop container %s: %v", containerID[:12], err)
	}

	// Remove container and volumes
	if err := o.dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	log.Printf("[objectstore] deprovisioned instance %s (container=%s)", instance.InstanceID, containerID[:12])
	return nil
}

// GetMetrics retrieves current metrics for an object storage instance.
func (o *ObjectStoreProvisioner) GetMetrics(ctx context.Context, instance *ServiceInstance) (*ServiceMetrics, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found in instance config")
	}

	stats, err := o.dockerClient.GetContainerStats(ctx, containerID)
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
		CPUPercent:     cpuPercent,
		MemoryPercent:  memPercent,
		MemoryUsageMB:  stats.MemoryStats.Usage / (1024 * 1024),
		StorageUsedGB:  0, // Would need to query MinIO admin API
		StorageTotalGB: 0, // Would need to query MinIO admin API
		QPS:            0, // Would need to query MinIO metrics endpoint
		Uptime:         time.Since(instance.CreatedAt),
	}, nil
}

// HealthCheck verifies that an object storage instance is responsive.
func (o *ObjectStoreProvisioner) HealthCheck(ctx context.Context, instance *ServiceInstance) (bool, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return false, fmt.Errorf("container_id not found in instance config")
	}

	// Check if container is running
	inspect, err := o.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	running, _ := inspect["State"].(map[string]interface{})["Running"].(bool)
	if !running {
		return false, nil
	}

	// Execute MinIO health check
	execConfig := map[string]interface{}{
		"Cmd": []string{
			"sh", "-c",
			"wget -q -O- http://localhost:9000/minio/health/live",
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := o.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create exec: %w", err)
	}

	output, err := o.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return false, fmt.Errorf("failed to execute health check: %w", err)
	}

	// MinIO health endpoint returns empty body with 200 OK when healthy
	healthy := !strings.Contains(output, "error") && !strings.Contains(output, "failed")
	return healthy, nil
}

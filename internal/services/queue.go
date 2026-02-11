package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// QueueProvisioner manages RabbitMQ Message Queue instances via Docker.
// It provisions isolated RabbitMQ containers on federation nodes.
type QueueProvisioner struct {
	dockerClient *DockerClient
}

// NewQueueProvisioner creates a new message queue provisioner.
func NewQueueProvisioner(dockerEndpoint string) *QueueProvisioner {
	return &QueueProvisioner{
		dockerClient: NewDockerClient(dockerEndpoint),
	}
}

// Provision creates a new RabbitMQ message queue instance via Docker.
func (q *QueueProvisioner) Provision(ctx context.Context, req ProvisionRequest, plan ServicePlan) (*ServiceInstance, error) {
	instanceID := generateServiceID("mq")
	username := fmt.Sprintf("user_%s", instanceID[:12])
	password := generatePassword(24)
	vhost := fmt.Sprintf("vhost_%s", instanceID[:12])
	containerName := fmt.Sprintf("rabbitmq-%s", instanceID)

	// Create Docker container with RabbitMQ
	containerConfig := map[string]interface{}{
		"Image": "rabbitmq:3-management-alpine",
		"Env": []string{
			fmt.Sprintf("RABBITMQ_DEFAULT_USER=%s", username),
			fmt.Sprintf("RABBITMQ_DEFAULT_PASS=%s", password),
			fmt.Sprintf("RABBITMQ_DEFAULT_VHOST=%s", vhost),
		},
		"HostConfig": map[string]interface{}{
			"Memory":   plan.MemoryMB * 1024 * 1024,
			"NanoCpus": plan.CPU * 1000000000,
			"PortBindings": map[string][]map[string]string{
				"5672/tcp": {{ // AMQP port
					"HostIp":   "0.0.0.0",
					"HostPort": "0", // Auto-assign port
				}},
				"15672/tcp": {{ // Management UI
					"HostIp":   "0.0.0.0",
					"HostPort": "0", // Auto-assign port
				}},
			},
			"Binds": []string{
				fmt.Sprintf("rabbitmq-data-%s:/var/lib/rabbitmq", instanceID),
			},
		},
		"Labels": map[string]string{
			"soholink.service.type":  "messagequeue",
			"soholink.service.id":    instanceID,
			"soholink.service.owner": req.OwnerDID,
			"soholink.service.plan":  req.Plan,
		},
	}

	// Create and start container
	containerID, err := q.dockerClient.CreateContainer(ctx, containerName, containerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := q.dockerClient.StartContainer(ctx, containerID); err != nil {
		q.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get assigned ports
	inspect, err := q.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		q.dockerClient.StopContainer(ctx, containerID, 10)
		q.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	hostPort := extractHostPort(inspect, "5672/tcp")
	if hostPort == 0 {
		hostPort = 5672
	}

	mgmtPort := extractHostPort(inspect, "15672/tcp")

	queueURL := fmt.Sprintf("amqp://%s:%s@localhost:%d/%s", username, password, hostPort, vhost)

	instance := &ServiceInstance{
		InstanceID:  instanceID,
		OwnerDID:    req.OwnerDID,
		ServiceType: ServiceTypeMessageQueue,
		Name:        req.Name,
		Plan:        req.Plan,
		Status:      StatusRunning,
		Port:        hostPort,
		Credentials: ServiceCredentials{
			Username: username,
			Password: password,
			QueueURL: queueURL,
		},
		Config: map[string]string{
			"container_id":    containerID,
			"container_name":  containerName,
			"image":           "rabbitmq:3-management-alpine",
			"vhost":           vhost,
			"management_port": fmt.Sprintf("%d", mgmtPort),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if req.Config != nil {
		for k, v := range req.Config {
			instance.Config[k] = v
		}
	}

	log.Printf("[queue] provisioned instance %s (container=%s, vhost=%s, port=%d, plan=%s)",
		instanceID, containerID[:12], vhost, hostPort, req.Plan)

	return instance, nil
}

// Deprovision terminates a message queue instance.
func (q *QueueProvisioner) Deprovision(ctx context.Context, instance *ServiceInstance) error {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return fmt.Errorf("container_id not found in instance config")
	}

	// Stop container gracefully (30 second timeout for messages to drain)
	if err := q.dockerClient.StopContainer(ctx, containerID, 30); err != nil {
		log.Printf("[queue] warning: failed to stop container %s: %v", containerID[:12], err)
	}

	// Remove container and volumes
	if err := q.dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	log.Printf("[queue] deprovisioned instance %s (container=%s)", instance.InstanceID, containerID[:12])
	return nil
}

// GetMetrics retrieves current metrics for a message queue instance.
func (q *QueueProvisioner) GetMetrics(ctx context.Context, instance *ServiceInstance) (*ServiceMetrics, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found in instance config")
	}

	stats, err := q.dockerClient.GetContainerStats(ctx, containerID)
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
		QPS:           0, // Would need to query RabbitMQ management API
		Uptime:        time.Since(instance.CreatedAt),
	}, nil
}

// HealthCheck verifies that a message queue instance is responsive.
func (q *QueueProvisioner) HealthCheck(ctx context.Context, instance *ServiceInstance) (bool, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return false, fmt.Errorf("container_id not found in instance config")
	}

	// Check if container is running
	inspect, err := q.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	running, _ := inspect["State"].(map[string]interface{})["Running"].(bool)
	if !running {
		return false, nil
	}

	// Execute rabbitmqctl status to verify RabbitMQ is responsive
	execConfig := map[string]interface{}{
		"Cmd": []string{
			"rabbitmqctl",
			"status",
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := q.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create exec: %w", err)
	}

	output, err := q.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return false, fmt.Errorf("failed to execute health check: %w", err)
	}

	// rabbitmqctl status returns node information when healthy
	healthy := strings.Contains(output, "Runtime") && !strings.Contains(output, "nodedown")
	return healthy, nil
}

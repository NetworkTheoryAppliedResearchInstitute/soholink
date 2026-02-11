package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// MySQLProvisioner manages MySQL-as-a-Service instances via Docker.
// It provisions isolated MySQL containers on federation nodes.
type MySQLProvisioner struct {
	dockerClient *DockerClient
}

// NewMySQLProvisioner creates a new MySQL provisioner.
func NewMySQLProvisioner(dockerEndpoint string) *MySQLProvisioner {
	return &MySQLProvisioner{
		dockerClient: NewDockerClient(dockerEndpoint),
	}
}

// Provision creates a new MySQL service instance via Docker.
func (m *MySQLProvisioner) Provision(ctx context.Context, req ProvisionRequest, plan ServicePlan) (*ServiceInstance, error) {
	instanceID := generateServiceID("mysql")
	dbName := fmt.Sprintf("db_%s", instanceID[:16])
	username := fmt.Sprintf("user_%s", instanceID[:12])
	password := generatePassword(24)
	rootPassword := generatePassword(32)
	containerName := fmt.Sprintf("mysql-%s", instanceID)

	// Create Docker container with MySQL
	containerConfig := map[string]interface{}{
		"Image": "mysql:8.0",
		"Env": []string{
			fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", rootPassword),
			fmt.Sprintf("MYSQL_DATABASE=%s", dbName),
			fmt.Sprintf("MYSQL_USER=%s", username),
			fmt.Sprintf("MYSQL_PASSWORD=%s", password),
		},
		"Cmd": []string{
			"--character-set-server=utf8mb4",
			"--collation-server=utf8mb4_unicode_ci",
			"--default-authentication-plugin=mysql_native_password",
		},
		"HostConfig": map[string]interface{}{
			"Memory":   plan.MemoryMB * 1024 * 1024,
			"NanoCpus": plan.CPU * 1000000000,
			"PortBindings": map[string][]map[string]string{
				"3306/tcp": {{
					"HostIp":   "0.0.0.0",
					"HostPort": "0", // Auto-assign port
				}},
			},
			"Binds": []string{
				fmt.Sprintf("mysql-data-%s:/var/lib/mysql", instanceID),
			},
		},
		"Labels": map[string]string{
			"soholink.service.type":  "mysql",
			"soholink.service.id":    instanceID,
			"soholink.service.owner": req.OwnerDID,
			"soholink.service.plan":  req.Plan,
		},
	}

	// Create and start container
	containerID, err := m.dockerClient.CreateContainer(ctx, containerName, containerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := m.dockerClient.StartContainer(ctx, containerID); err != nil {
		m.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get assigned port
	inspect, err := m.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		m.dockerClient.StopContainer(ctx, containerID, 10)
		m.dockerClient.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	hostPort := extractHostPort(inspect, "3306/tcp")
	if hostPort == 0 {
		hostPort = 3306
	}

	instance := &ServiceInstance{
		InstanceID:  instanceID,
		OwnerDID:    req.OwnerDID,
		ServiceType: ServiceTypeMySQL,
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
			"image":          "mysql:8.0",
			"root_password":  rootPassword,
			"charset":        "utf8mb4",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if plan.HA && plan.Replicas > 1 {
		instance.Config["replication"] = "async"
		instance.Config["replicas"] = fmt.Sprintf("%d", plan.Replicas)
	}

	log.Printf("[mysql] provisioned instance %s (container=%s, db=%s, port=%d, plan=%s)",
		instanceID, containerID[:12], dbName, hostPort, req.Plan)

	return instance, nil
}

// Deprovision terminates a MySQL service instance.
func (m *MySQLProvisioner) Deprovision(ctx context.Context, instance *ServiceInstance) error {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return fmt.Errorf("container_id not found in instance config")
	}

	// Stop container gracefully (30 second timeout for connections to drain)
	if err := m.dockerClient.StopContainer(ctx, containerID, 30); err != nil {
		log.Printf("[mysql] warning: failed to stop container %s: %v", containerID[:12], err)
	}

	// Remove container and volumes
	if err := m.dockerClient.RemoveContainer(ctx, containerID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	log.Printf("[mysql] deprovisioned instance %s (container=%s)", instance.InstanceID, containerID[:12])
	return nil
}

// GetMetrics retrieves current metrics for a MySQL instance.
func (m *MySQLProvisioner) GetMetrics(ctx context.Context, instance *ServiceInstance) (*ServiceMetrics, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found in instance config")
	}

	stats, err := m.dockerClient.GetContainerStats(ctx, containerID)
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

	// Get MySQL-specific metrics
	connections, qps := m.getMySQLStatus(ctx, instance)

	return &ServiceMetrics{
		CPUPercent:    cpuPercent,
		MemoryPercent: memPercent,
		MemoryUsageMB: stats.MemoryStats.Usage / (1024 * 1024),
		Connections:   connections,
		QPS:           qps,
		AvgLatencyMs:  0, // Would need performance_schema queries
		Uptime:        time.Since(instance.CreatedAt),
	}, nil
}

// HealthCheck verifies that a MySQL instance is responsive.
func (m *MySQLProvisioner) HealthCheck(ctx context.Context, instance *ServiceInstance) (bool, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return false, fmt.Errorf("container_id not found in instance config")
	}

	// Check if container is running
	inspect, err := m.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	running, _ := inspect["State"].(map[string]interface{})["Running"].(bool)
	if !running {
		return false, nil
	}

	// Execute mysqladmin ping to verify MySQL is responsive
	execConfig := map[string]interface{}{
		"Cmd": []string{
			"mysqladmin",
			fmt.Sprintf("--user=%s", instance.Credentials.Username),
			fmt.Sprintf("--password=%s", instance.Credentials.Password),
			"ping",
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := m.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create exec: %w", err)
	}

	output, err := m.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return false, fmt.Errorf("failed to execute health check: %w", err)
	}

	// mysqladmin ping responds with "mysqld is alive" when healthy
	healthy := strings.Contains(output, "mysqld is alive")
	return healthy, nil
}

// getMySQLStatus retrieves MySQL STATUS variables.
func (m *MySQLProvisioner) getMySQLStatus(ctx context.Context, instance *ServiceInstance) (int, float64) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return 0, 0
	}

	// Execute SHOW STATUS command
	execConfig := map[string]interface{}{
		"Cmd": []string{
			"mysql",
			fmt.Sprintf("--user=%s", instance.Credentials.Username),
			fmt.Sprintf("--password=%s", instance.Credentials.Password),
			"--batch",
			"--skip-column-names",
			"-e", "SHOW STATUS WHERE Variable_name IN ('Threads_connected', 'Questions', 'Uptime')",
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := m.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return 0, 0
	}

	output, err := m.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return 0, 0
	}

	// Parse output for connections and calculate QPS
	connections := 0
	questions := 0
	uptime := 1 // Avoid division by zero

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			switch fields[0] {
			case "Threads_connected":
				fmt.Sscanf(fields[1], "%d", &connections)
			case "Questions":
				fmt.Sscanf(fields[1], "%d", &questions)
			case "Uptime":
				fmt.Sscanf(fields[1], "%d", &uptime)
			}
		}
	}

	qps := 0.0
	if uptime > 0 {
		qps = float64(questions) / float64(uptime)
	}

	return connections, qps
}

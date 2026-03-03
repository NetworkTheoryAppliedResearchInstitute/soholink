package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DockerClient provides a minimal Docker Engine API client.
// This implementation uses the Docker HTTP API directly without external dependencies.
type DockerClient struct {
	endpoint string
	client   *http.Client
}

// NewDockerClient creates a new Docker API client.
// endpoint can be "unix:///var/run/docker.sock" or "tcp://host:port"
func NewDockerClient(endpoint string) *DockerClient {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Configure Unix socket transport if needed
	if strings.HasPrefix(endpoint, "unix://") {
		socketPath := strings.TrimPrefix(endpoint, "unix://")
		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		}
	}

	return &DockerClient{
		endpoint: endpoint,
		client:   client,
	}
}

// CreateContainer creates a new container.
func (d *DockerClient) CreateContainer(ctx context.Context, name string, config map[string]interface{}) (string, error) {
	url := d.buildURL(fmt.Sprintf("/containers/create?name=%s", name))

	body, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.ID, nil
}

// StartContainer starts a container.
func (d *DockerClient) StartContainer(ctx context.Context, containerID string) error {
	url := d.buildURL(fmt.Sprintf("/containers/%s/start", containerID))

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// StopContainer stops a container.
func (d *DockerClient) StopContainer(ctx context.Context, containerID string, timeout int) error {
	url := d.buildURL(fmt.Sprintf("/containers/%s/stop?t=%d", containerID, timeout))

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RemoveContainer removes a container.
func (d *DockerClient) RemoveContainer(ctx context.Context, containerID string, removeVolumes bool) error {
	url := d.buildURL(fmt.Sprintf("/containers/%s?v=%t&force=true", containerID, removeVolumes))

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// InspectContainer returns detailed information about a container.
func (d *DockerClient) InspectContainer(ctx context.Context, containerID string) (map[string]interface{}, error) {
	url := d.buildURL(fmt.Sprintf("/containers/%s/json", containerID))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// ContainerStats represents container resource usage statistics.
type ContainerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
}

// GetContainerStats retrieves resource usage statistics for a container.
func (d *DockerClient) GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	url := d.buildURL(fmt.Sprintf("/containers/%s/stats?stream=false", containerID))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var stats ContainerStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// CreateExec creates an exec instance inside a container.
func (d *DockerClient) CreateExec(ctx context.Context, containerID string, config map[string]interface{}) (string, error) {
	url := d.buildURL(fmt.Sprintf("/containers/%s/exec", containerID))

	body, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.ID, nil
}

// StartExec starts an exec instance and returns its output.
func (d *DockerClient) StartExec(ctx context.Context, execID string) (string, error) {
	url := d.buildURL(fmt.Sprintf("/exec/%s/start", execID))

	config := map[string]interface{}{
		"Detach": false,
		"Tty":    false,
	}

	body, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	output, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// ListContainers lists all containers matching the given filters.
func (d *DockerClient) ListContainers(ctx context.Context, all bool, filters map[string][]string) ([]map[string]interface{}, error) {
	params := ""
	if all {
		params = "?all=true"
	}

	if len(filters) > 0 {
		filterJSON, _ := json.Marshal(filters)
		if params == "" {
			params = "?"
		} else {
			params += "&"
		}
		params += "filters=" + string(filterJSON)
	}

	url := d.buildURL("/containers/json" + params)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// buildURL constructs the full API URL.
func (d *DockerClient) buildURL(path string) string {
	const apiVersion = "/v1.41"
	if strings.HasPrefix(d.endpoint, "unix://") {
		// For Unix sockets, use http://localhost as the host
		return "http://localhost" + apiVersion + path
	}
	return strings.TrimPrefix(d.endpoint, "tcp://") + apiVersion + path
}

// extractHostPort extracts the host port from container inspect data.
func extractHostPort(inspect map[string]interface{}, portKey string) int {
	networkSettings, ok := inspect["NetworkSettings"].(map[string]interface{})
	if !ok {
		return 0
	}

	ports, ok := networkSettings["Ports"].(map[string]interface{})
	if !ok {
		return 0
	}

	portBindings, ok := ports[portKey].([]interface{})
	if !ok || len(portBindings) == 0 {
		return 0
	}

	firstBinding, ok := portBindings[0].(map[string]interface{})
	if !ok {
		return 0
	}

	hostPortStr, ok := firstBinding["HostPort"].(string)
	if !ok {
		return 0
	}

	hostPort, _ := strconv.Atoi(hostPortStr)
	return hostPort
}

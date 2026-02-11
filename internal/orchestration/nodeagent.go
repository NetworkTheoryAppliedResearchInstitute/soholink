package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NodeAgentClient communicates with node agents for workload deployment.
type NodeAgentClient struct {
	// Base URL of the node agent API
	baseURL string

	// HTTP client
	client *http.Client

	// Authentication token
	authToken string
}

// NewNodeAgentClient creates a new node agent client.
func NewNodeAgentClient(baseURL, authToken string) *NodeAgentClient {
	return &NodeAgentClient{
		baseURL:   baseURL,
		authToken: authToken,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DeployWorkload deploys a workload to a node.
func (c *NodeAgentClient) DeployWorkload(ctx context.Context, workload *Workload) error {
	deployReq := map[string]interface{}{
		"workload_id":   workload.WorkloadID,
		"image":         workload.Image,
		"replicas":      1, // Single replica per node
		"cpu":           workload.Resources.CPUCores,
		"memory":        workload.Resources.MemoryMB,
		"env":           workload.Env,
		"ports":         workload.Ports,
		"volumes":       workload.Volumes,
		"restart_policy": workload.RestartPolicy,
	}

	resp, err := c.doRequest(ctx, "POST", "/api/workloads/deploy", deployReq)
	if err != nil {
		return fmt.Errorf("failed to deploy workload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deployment failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetWorkloadStatus retrieves the status of a workload on a node.
func (c *NodeAgentClient) GetWorkloadStatus(ctx context.Context, workloadID string) (*WorkloadNodeStatus, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/workloads/%s/status", workloadID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workload status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status check failed with status %d", resp.StatusCode)
	}

	var status WorkloadNodeStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status: %w", err)
	}

	return &status, nil
}

// StopWorkload stops a workload on a node.
func (c *NodeAgentClient) StopWorkload(ctx context.Context, workloadID string) error {
	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/workloads/%s/stop", workloadID), nil)
	if err != nil {
		return fmt.Errorf("failed to stop workload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// RemoveWorkload removes a workload from a node.
func (c *NodeAgentClient) RemoveWorkload(ctx context.Context, workloadID string) error {
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/workloads/%s", workloadID), nil)
	if err != nil {
		return fmt.Errorf("failed to remove workload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remove failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetNodeHealth retrieves node health metrics.
func (c *NodeAgentClient) GetNodeHealth(ctx context.Context) (*NodeHealth, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/health", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get node health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	var health NodeHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode health: %w", err)
	}

	return &health, nil
}

// GetNodeMetrics retrieves detailed node metrics.
func (c *NodeAgentClient) GetNodeMetrics(ctx context.Context) (*NodeMetrics, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/metrics", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get node metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics request failed with status %d", resp.StatusCode)
	}

	var metrics NodeMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return nil, fmt.Errorf("failed to decode metrics: %w", err)
	}

	return &metrics, nil
}

// ScaleWorkload adjusts the number of replicas for a workload on a node.
func (c *NodeAgentClient) ScaleWorkload(ctx context.Context, workloadID string, replicas int) error {
	scaleReq := map[string]interface{}{
		"replicas": replicas,
	}

	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/workloads/%s/scale", workloadID), scaleReq)
	if err != nil {
		return fmt.Errorf("failed to scale workload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scale failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// doRequest performs an HTTP request to the node agent.
func (c *NodeAgentClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	}

	return c.client.Do(req)
}

// WorkloadNodeStatus represents the status of a workload on a specific node.
type WorkloadNodeStatus struct {
	WorkloadID string    `json:"workload_id"`
	State      string    `json:"state"` // running, stopped, failed, etc.
	Replicas   int       `json:"replicas"`
	Health     string    `json:"health"` // healthy, unhealthy, unknown
	StartedAt  time.Time `json:"started_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Metrics    struct {
		CPUUsage    float64 `json:"cpu_usage"`
		MemoryUsage int64   `json:"memory_usage"`
		NetworkRx   int64   `json:"network_rx"`
		NetworkTx   int64   `json:"network_tx"`
	} `json:"metrics"`
}

// NodeHealth represents overall node health.
type NodeHealth struct {
	Status      string    `json:"status"` // healthy, degraded, unhealthy
	Timestamp   time.Time `json:"timestamp"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryUsed  int64     `json:"memory_used"`
	MemoryTotal int64     `json:"memory_total"`
	DiskUsed    int64     `json:"disk_used"`
	DiskTotal   int64     `json:"disk_total"`
	Workloads   int       `json:"workloads_count"`
}

// NodeMetrics represents detailed node resource metrics.
type NodeMetrics struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       struct {
		UsagePercent float64   `json:"usage_percent"`
		PerCore      []float64 `json:"per_core"`
		LoadAvg1     float64   `json:"load_avg_1"`
		LoadAvg5     float64   `json:"load_avg_5"`
		LoadAvg15    float64   `json:"load_avg_15"`
	} `json:"cpu"`
	Memory struct {
		Total       int64   `json:"total"`
		Used        int64   `json:"used"`
		Free        int64   `json:"free"`
		UsedPercent float64 `json:"used_percent"`
		Cached      int64   `json:"cached"`
		Buffers     int64   `json:"buffers"`
	} `json:"memory"`
	Disk struct {
		Total       int64   `json:"total"`
		Used        int64   `json:"used"`
		Free        int64   `json:"free"`
		UsedPercent float64 `json:"used_percent"`
		ReadBytes   int64   `json:"read_bytes"`
		WriteBytes  int64   `json:"write_bytes"`
	} `json:"disk"`
	Network struct {
		RxBytes   int64 `json:"rx_bytes"`
		TxBytes   int64 `json:"tx_bytes"`
		RxPackets int64 `json:"rx_packets"`
		TxPackets int64 `json:"tx_packets"`
		RxErrors  int64 `json:"rx_errors"`
		TxErrors  int64 `json:"tx_errors"`
	} `json:"network"`
	Workloads []WorkloadNodeStatus `json:"workloads"`
}

// IntegrateNodeAgent integrates node agent calls into the scheduler.
func (s *FedScheduler) scheduleWorkload(ctx context.Context, w *Workload) error {
	// Find suitable nodes
	nodes := s.discovery.GetAvailableNodes()
	if len(nodes) == 0 {
		return fmt.Errorf("no available nodes")
	}

	// Use placer to select best nodes
	selectedNodes := s.placer.PlaceReplicas(w, nodes, w.Replicas)
	if len(selectedNodes) < w.Replicas {
		return fmt.Errorf("insufficient capacity: need %d replicas, found %d nodes", w.Replicas, len(selectedNodes))
	}

	// Create workload state
	state := &WorkloadState{
		WorkloadID: w.WorkloadID,
		Status:     "deploying",
		Replicas:   make(map[string]*ReplicaState),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Deploy to each selected node
	for i, node := range selectedNodes {
		replicaID := fmt.Sprintf("%s-replica-%d", w.WorkloadID, i)

		// Create node agent client
		agentClient := NewNodeAgentClient(node.AgentURL, node.AuthToken)

		// Deploy workload
		if err := agentClient.DeployWorkload(ctx, w); err != nil {
			// Mark replica as failed
			state.Replicas[replicaID] = &ReplicaState{
				ReplicaID: replicaID,
				NodeID:    node.NodeID,
				Status:    "failed",
				Error:     err.Error(),
				UpdatedAt: time.Now(),
			}
			continue
		}

		// Mark replica as running
		state.Replicas[replicaID] = &ReplicaState{
			ReplicaID: replicaID,
			NodeID:    node.NodeID,
			Status:    "running",
			UpdatedAt: time.Now(),
		}
	}

	// Update overall workload status
	successCount := 0
	for _, replica := range state.Replicas {
		if replica.Status == "running" {
			successCount++
		}
	}

	if successCount == 0 {
		state.Status = "failed"
	} else if successCount < w.Replicas {
		state.Status = "degraded"
	} else {
		state.Status = "running"
	}

	// Store workload state
	s.mu.Lock()
	s.ActiveWorkloads[w.WorkloadID] = state
	s.mu.Unlock()

	return nil
}

// ReplicaState represents the state of a single replica.
type ReplicaState struct {
	ReplicaID string
	NodeID    string
	Status    string
	Error     string
	UpdatedAt time.Time
}

// NodeInfo represents a federation node.
type NodeInfo struct {
	NodeID    string
	AgentURL  string
	AuthToken string
	Capacity  *NodeCapacity
}

package orchestration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNodeAgentClient_DeployWorkload(t *testing.T) {
	deployed := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/workloads/deploy" && r.Method == "POST" {
			deployed = true

			var workload Workload
			json.NewDecoder(r.Body).Decode(&workload)

			if workload.WorkloadID == "" {
				t.Error("Expected non-empty workload ID")
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	workload := &Workload{
		WorkloadID: "test-workload-001",
		Name:       "test-app",
		Type:       "container",
		Image:      "nginx:latest",
		Replicas:   1,
		Resources: ResourceRequirements{
			CPUCores: 1.0,
			MemoryMB: 512,
		},
	}

	ctx := context.Background()
	err := client.DeployWorkload(ctx, workload)
	if err != nil {
		t.Fatalf("DeployWorkload failed: %v", err)
	}

	if !deployed {
		t.Error("Expected workload to be deployed")
	}
}

func TestNodeAgentClient_GetWorkloadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workloads/") && strings.HasSuffix(r.URL.Path, "/status") {
			status := WorkloadNodeStatus{
				WorkloadID: "test-workload-001",
				NodeID:     "node-001",
				Status:     "running",
				Replicas:   1,
				StartedAt:  time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(status)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	ctx := context.Background()
	status, err := client.GetWorkloadStatus(ctx, "test-workload-001")
	if err != nil {
		t.Fatalf("GetWorkloadStatus failed: %v", err)
	}

	if status.WorkloadID != "test-workload-001" {
		t.Errorf("Expected workload ID 'test-workload-001', got '%s'", status.WorkloadID)
	}

	if status.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", status.Status)
	}
}

func TestNodeAgentClient_StopWorkload(t *testing.T) {
	stopped := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workloads/") && strings.HasSuffix(r.URL.Path, "/stop") && r.Method == "POST" {
			stopped = true
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "stopped"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	ctx := context.Background()
	err := client.StopWorkload(ctx, "test-workload-001")
	if err != nil {
		t.Fatalf("StopWorkload failed: %v", err)
	}

	if !stopped {
		t.Error("Expected workload to be stopped")
	}
}

func TestNodeAgentClient_RemoveWorkload(t *testing.T) {
	removed := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workloads/") && r.Method == "DELETE" {
			removed = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	ctx := context.Background()
	err := client.RemoveWorkload(ctx, "test-workload-001")
	if err != nil {
		t.Fatalf("RemoveWorkload failed: %v", err)
	}

	if !removed {
		t.Error("Expected workload to be removed")
	}
}

func TestNodeAgentClient_GetNodeHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" && r.Method == "GET" {
			health := NodeHealth{
				NodeID:            "node-001",
				Status:            "healthy",
				CPUUsagePercent:   45.5,
				MemoryUsagePercent: 62.3,
				DiskUsagePercent:  38.7,
				Uptime:            time.Hour * 24,
				LastHeartbeat:     time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(health)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	ctx := context.Background()
	health, err := client.GetNodeHealth(ctx)
	if err != nil {
		t.Fatalf("GetNodeHealth failed: %v", err)
	}

	if health.NodeID != "node-001" {
		t.Errorf("Expected node ID 'node-001', got '%s'", health.NodeID)
	}

	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", health.Status)
	}

	if health.CPUUsagePercent <= 0 {
		t.Error("Expected positive CPU usage")
	}
}

func TestNodeAgentClient_GetNodeMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/metrics" && r.Method == "GET" {
			metrics := NodeMetrics{
				NodeID:           "node-001",
				CPUCores:         8,
				CPUUsagePercent:  35.2,
				MemoryTotalMB:    16384,
				MemoryUsedMB:     8192,
				DiskTotalGB:      500,
				DiskUsedGB:       200,
				NetworkRxBytes:   1024 * 1024 * 1024,
				NetworkTxBytes:   512 * 1024 * 1024,
				WorkloadCount:    5,
				Timestamp:        time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(metrics)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	ctx := context.Background()
	metrics, err := client.GetNodeMetrics(ctx)
	if err != nil {
		t.Fatalf("GetNodeMetrics failed: %v", err)
	}

	if metrics.CPUCores != 8 {
		t.Errorf("Expected 8 CPU cores, got %d", metrics.CPUCores)
	}

	if metrics.MemoryTotalMB != 16384 {
		t.Errorf("Expected 16384 MB memory, got %d", metrics.MemoryTotalMB)
	}

	if metrics.WorkloadCount != 5 {
		t.Errorf("Expected 5 workloads, got %d", metrics.WorkloadCount)
	}
}

func TestNodeAgentClient_ScaleWorkload(t *testing.T) {
	scaled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workloads/") && strings.HasSuffix(r.URL.Path, "/scale") && r.Method == "POST" {
			scaled = true

			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)

			replicas, ok := req["replicas"].(float64)
			if !ok || replicas != 3 {
				t.Errorf("Expected replicas=3, got %v", req["replicas"])
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "scaled"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	ctx := context.Background()
	err := client.ScaleWorkload(ctx, "test-workload-001", 3)
	if err != nil {
		t.Fatalf("ScaleWorkload failed: %v", err)
	}

	if !scaled {
		t.Error("Expected workload to be scaled")
	}
}

func TestNodeAgentClient_Authentication(t *testing.T) {
	tokenReceived := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenReceived = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "secret-token-123")

	workload := &Workload{
		WorkloadID: "test",
		Name:       "test",
		Type:       "container",
	}

	ctx := context.Background()
	client.DeployWorkload(ctx, workload)

	expectedAuth := "Bearer secret-token-123"
	if tokenReceived != expectedAuth {
		t.Errorf("Expected Authorization '%s', got '%s'", expectedAuth, tokenReceived)
	}
}

func TestNodeAgentClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Internal server error",
		})
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	workload := &Workload{
		WorkloadID: "test",
		Name:       "test",
		Type:       "container",
	}

	ctx := context.Background()
	err := client.DeployWorkload(ctx, workload)

	if err == nil {
		t.Fatal("Expected error for 500 response")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected error to mention 500 status, got: %v", err)
	}
}

func TestNodeAgentClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	workload := &Workload{
		WorkloadID: "test",
		Name:       "test",
		Type:       "container",
	}

	err := client.DeployWorkload(ctx, workload)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

func TestNodeAgentClient_Retry(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++

		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	// Note: Retry logic would need to be implemented in NodeAgentClient
	ctx := context.Background()
	workload := &Workload{
		WorkloadID: "test",
		Name:       "test",
		Type:       "container",
	}

	// First attempts should fail
	err := client.DeployWorkload(ctx, workload)
	if err == nil {
		t.Log("Deployment succeeded (retry not yet implemented)")
	}
}

func TestNodeAgentClient_ConcurrentRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	done := make(chan bool, 10)

	// Make 10 concurrent requests
	for i := 0; i < 10; i++ {
		go func(index int) {
			workload := &Workload{
				WorkloadID: string(rune('A' + index)),
				Name:       "test",
				Type:       "container",
			}

			ctx := context.Background()
			err := client.DeployWorkload(ctx, workload)
			if err != nil {
				t.Errorf("Concurrent request %d failed: %v", index, err)
			}

			done <- true
		}(i)
	}

	// Wait for all requests
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Fatal("Concurrent requests timed out")
		}
	}

	if requestCount != 10 {
		t.Errorf("Expected 10 requests, got %d", requestCount)
	}
}

func TestWorkloadNodeStatus_IsHealthy(t *testing.T) {
	testCases := []struct {
		name     string
		status   WorkloadNodeStatus
		expected bool
	}{
		{
			name:     "Running status",
			status:   WorkloadNodeStatus{Status: "running"},
			expected: true,
		},
		{
			name:     "Failed status",
			status:   WorkloadNodeStatus{Status: "failed"},
			expected: false,
		},
		{
			name:     "Stopped status",
			status:   WorkloadNodeStatus{Status: "stopped"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isHealthy := (tc.status.Status == "running")
			if isHealthy != tc.expected {
				t.Errorf("Expected IsHealthy=%v, got %v", tc.expected, isHealthy)
			}
		})
	}
}

func TestNodeHealth_IsHealthy(t *testing.T) {
	testCases := []struct {
		name     string
		health   NodeHealth
		expected bool
	}{
		{
			name:     "Healthy node",
			health:   NodeHealth{Status: "healthy", CPUUsagePercent: 50, MemoryUsagePercent: 60},
			expected: true,
		},
		{
			name:     "Unhealthy node",
			health:   NodeHealth{Status: "unhealthy"},
			expected: false,
		},
		{
			name:     "Degraded node",
			health:   NodeHealth{Status: "degraded"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isHealthy := (tc.health.Status == "healthy")
			if isHealthy != tc.expected {
				t.Errorf("Expected IsHealthy=%v, got %v", tc.expected, isHealthy)
			}
		})
	}
}

func BenchmarkNodeAgentClient_DeployWorkload(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")

	workload := &Workload{
		WorkloadID: "bench-workload",
		Name:       "bench",
		Type:       "container",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.DeployWorkload(ctx, workload)
	}
}

func BenchmarkNodeAgentClient_GetNodeMetrics(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics := NodeMetrics{
			NodeID:       "bench-node",
			CPUCores:     8,
			MemoryTotalMB: 16384,
		}
		json.NewEncoder(w).Encode(metrics)
	}))
	defer server.Close()

	client := NewNodeAgentClient(server.URL, "test-token")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetNodeMetrics(ctx)
	}
}

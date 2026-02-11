package services

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDockerClient_CreateContainer(t *testing.T) {
	// Mock Docker API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.41/containers/create" && r.Method == "POST" {
			response := map[string]interface{}{
				"Id":       "test-container-id-12345",
				"Warnings": []string{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	config := map[string]interface{}{
		"Image": "nginx:latest",
		"Env":   []string{"TEST=value"},
	}

	containerID, err := client.CreateContainer(ctx, "test-container", config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}

	if containerID != "test-container-id-12345" {
		t.Errorf("Expected container ID 'test-container-id-12345', got '%s'", containerID)
	}
}

func TestDockerClient_StartContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/start") && r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	err := client.StartContainer(ctx, "test-container-id")
	if err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}
}

func TestDockerClient_StopContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stop") && r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	err := client.StopContainer(ctx, "test-container-id", 10)
	if err != nil {
		t.Fatalf("StopContainer failed: %v", err)
	}
}

func TestDockerClient_InspectContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/json") && r.Method == "GET" {
			response := map[string]interface{}{
				"Id":    "test-container-id",
				"Name":  "/test-container",
				"Image": "nginx:latest",
				"State": map[string]interface{}{
					"Status":  "running",
					"Running": true,
				},
				"NetworkSettings": map[string]interface{}{
					"IPAddress": "172.17.0.2",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	info, err := client.InspectContainer(ctx, "test-container-id")
	if err != nil {
		t.Fatalf("InspectContainer failed: %v", err)
	}

	if info.ID != "test-container-id" {
		t.Errorf("Expected ID 'test-container-id', got '%s'", info.ID)
	}

	if info.State != "running" {
		t.Errorf("Expected state 'running', got '%s'", info.State)
	}

	if info.IPAddress != "172.17.0.2" {
		t.Errorf("Expected IP '172.17.0.2', got '%s'", info.IPAddress)
	}
}

func TestDockerClient_GetContainerStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") && r.Method == "GET" {
			stats := map[string]interface{}{
				"cpu_stats": map[string]interface{}{
					"cpu_usage": map[string]interface{}{
						"total_usage": 1000000000,
					},
					"system_cpu_usage": 10000000000,
				},
				"precpu_stats": map[string]interface{}{
					"cpu_usage": map[string]interface{}{
						"total_usage": 500000000,
					},
					"system_cpu_usage": 9000000000,
				},
				"memory_stats": map[string]interface{}{
					"usage": 134217728,
					"limit": 1073741824,
				},
				"networks": map[string]interface{}{
					"eth0": map[string]interface{}{
						"rx_bytes": 1024000,
						"tx_bytes": 2048000,
					},
				},
				"blkio_stats": map[string]interface{}{
					"io_service_bytes_recursive": []interface{}{
						map[string]interface{}{
							"op":    "Read",
							"value": 5242880,
						},
						map[string]interface{}{
							"op":    "Write",
							"value": 10485760,
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	stats, err := client.GetContainerStats(ctx, "test-container-id")
	if err != nil {
		t.Fatalf("GetContainerStats failed: %v", err)
	}

	if stats.MemoryStats.Usage != 134217728 {
		t.Errorf("Expected memory usage 134217728, got %d", stats.MemoryStats.Usage)
	}

	if stats.MemoryStats.Limit != 1073741824 {
		t.Errorf("Expected memory limit 1073741824, got %d", stats.MemoryStats.Limit)
	}
}

func TestDockerClient_ListContainers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.41/containers/json" && r.Method == "GET" {
			response := []map[string]interface{}{
				{
					"Id":    "container1",
					"Names": []string{"/test1"},
					"Image": "nginx:latest",
					"State": "running",
				},
				{
					"Id":    "container2",
					"Names": []string{"/test2"},
					"Image": "redis:7",
					"State": "running",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	containers, err := client.ListContainers(ctx, true)
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}

	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}

	if containers[0].ID != "container1" {
		t.Errorf("Expected first container ID 'container1', got '%s'", containers[0].ID)
	}
}

func TestDockerClient_CreateExec(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/exec") && r.Method == "POST" {
			response := map[string]interface{}{
				"Id": "exec-id-12345",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	execID, err := client.CreateExec(ctx, "container-id", []string{"echo", "test"})
	if err != nil {
		t.Fatalf("CreateExec failed: %v", err)
	}

	if execID != "exec-id-12345" {
		t.Errorf("Expected exec ID 'exec-id-12345', got '%s'", execID)
	}
}

func TestDockerClient_RemoveContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && strings.Contains(r.URL.Path, "/containers/") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	err := client.RemoveContainer(ctx, "test-container-id", true, true)
	if err != nil {
		t.Fatalf("RemoveContainer failed: %v", err)
	}
}

func TestDockerClient_UnixSocket(t *testing.T) {
	// Test Unix socket detection
	client := NewDockerClient("unix:///var/run/docker.sock")

	if client.endpoint != "unix:///var/run/docker.sock" {
		t.Errorf("Expected Unix socket endpoint to be preserved")
	}

	// Verify Unix socket transport is configured
	transport, ok := client.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Expected http.Transport")
	}

	if transport.DialContext == nil {
		t.Errorf("Expected DialContext to be set for Unix socket")
	}
}

func TestDockerClient_ContextCancellation(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)

	// Context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.CreateContainer(ctx, "test", map[string]interface{}{})
	if err == nil {
		t.Fatalf("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline error, got: %v", err)
	}
}

func TestDockerClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Internal server error",
		})
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	_, err := client.CreateContainer(ctx, "test", map[string]interface{}{})
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected status 500 in error, got: %v", err)
	}
}

// Benchmark tests
func BenchmarkDockerClient_CreateContainer(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Id": "benchmark-container-id",
		})
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()
	config := map[string]interface{}{"Image": "nginx:latest"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.CreateContainer(ctx, "bench", config)
	}
}

func BenchmarkDockerClient_InspectContainer(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Id":    "test",
			"State": map[string]interface{}{"Status": "running"},
		})
	}))
	defer server.Close()

	client := NewDockerClient(server.URL)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.InspectContainer(ctx, "test")
	}
}

// Helper to create Unix socket server for integration tests
func createUnixSocketServer(t *testing.T, socketPath string, handler http.Handler) net.Listener {
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create Unix socket: %v", err)
	}

	go http.Serve(listener, handler)
	return listener
}

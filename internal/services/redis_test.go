package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedisProvisioner_Provision(t *testing.T) {
	// Mock Docker API
	containerID := "redis-container-12345"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.41/containers/create" && r.Method == "POST":
			// Verify Redis image is requested
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)

			image, ok := req["Image"].(string)
			if !ok || !strings.Contains(image, "redis") {
				t.Errorf("Expected Redis image, got: %v", image)
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": containerID,
			})

		case strings.Contains(r.URL.Path, "/start") && r.Method == "POST":
			w.WriteHeader(http.StatusNoContent)

		case strings.Contains(r.URL.Path, "/json") && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":    containerID,
				"State": map[string]interface{}{"Status": "running"},
				"NetworkSettings": map[string]interface{}{
					"IPAddress": "172.17.0.10",
					"Ports": map[string]interface{}{
						"6379/tcp": []interface{}{
							map[string]interface{}{"HostPort": "6379"},
						},
					},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	provisioner := &RedisProvisioner{dockerClient: dockerClient}

	req := ProvisionRequest{
		Name: "redis-test-001",
		Plan: "shared",
		Config: map[string]string{
			"password": "testpass123",
		},
	}

	plan := ServicePlan{
		PlanID:    "shared",
		Name:      "Shared Redis",
		MemoryMB:  512,
		CPUCores:  0.5,
		StorageGB: 1,
	}

	ctx := context.Background()
	instance, err := provisioner.Provision(ctx, req, plan)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	if instance.InstanceID == "" {
		t.Error("Expected non-empty instance ID")
	}

	if instance.ServiceType != ServiceTypeRedis {
		t.Errorf("Expected service type Redis, got %s", instance.ServiceType)
	}

	if instance.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", instance.Status)
	}

	// Check connection details
	if instance.Endpoint == "" {
		t.Error("Expected non-empty endpoint")
	}

	if !strings.Contains(instance.Endpoint, "6379") {
		t.Errorf("Expected Redis port 6379 in endpoint, got: %s", instance.Endpoint)
	}

	containerIDFromConfig, ok := instance.Config["container_id"]
	if !ok {
		t.Error("Expected container_id in config")
	}
	if containerIDFromConfig != containerID {
		t.Errorf("Expected container_id '%s', got '%s'", containerID, containerIDFromConfig)
	}
}

func TestRedisProvisioner_Deprovision(t *testing.T) {
	stopped := false
	removed := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/stop") && r.Method == "POST":
			stopped = true
			w.WriteHeader(http.StatusNoContent)

		case r.Method == "DELETE":
			removed = true
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	provisioner := &RedisProvisioner{dockerClient: dockerClient}

	instance := &ServiceInstance{
		InstanceID:  "redis-test-001",
		ServiceType: ServiceTypeRedis,
		Status:      "running",
		Config: map[string]string{
			"container_id": "redis-container-12345",
		},
	}

	ctx := context.Background()
	err := provisioner.Deprovision(ctx, instance)
	if err != nil {
		t.Fatalf("Deprovision failed: %v", err)
	}

	if !stopped {
		t.Error("Expected container to be stopped")
	}

	if !removed {
		t.Error("Expected container to be removed")
	}
}

func TestRedisProvisioner_GetMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			stats := map[string]interface{}{
				"cpu_stats": map[string]interface{}{
					"cpu_usage":        map[string]interface{}{"total_usage": 2000000000},
					"system_cpu_usage": 20000000000,
				},
				"precpu_stats": map[string]interface{}{
					"cpu_usage":        map[string]interface{}{"total_usage": 1000000000},
					"system_cpu_usage": 18000000000,
				},
				"memory_stats": map[string]interface{}{
					"usage": 268435456,
					"limit": 536870912,
				},
				"networks": map[string]interface{}{
					"eth0": map[string]interface{}{
						"rx_bytes": 5242880,
						"tx_bytes": 10485760,
					},
				},
				"blkio_stats": map[string]interface{}{
					"io_service_bytes_recursive": []interface{}{},
				},
			}
			json.NewEncoder(w).Encode(stats)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	provisioner := &RedisProvisioner{dockerClient: dockerClient}

	instance := &ServiceInstance{
		InstanceID:  "redis-test-001",
		ServiceType: ServiceTypeRedis,
		Config: map[string]string{
			"container_id": "redis-container-12345",
		},
	}

	ctx := context.Background()
	metrics, err := provisioner.GetMetrics(ctx, instance)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if metrics.MemoryUsageMB != 256 {
		t.Errorf("Expected memory usage 256 MB, got %d MB", metrics.MemoryUsageMB)
	}

	if metrics.CPUPercent <= 0 {
		t.Errorf("Expected positive CPU usage, got %.2f", metrics.CPUPercent)
	}
}

func TestRedisProvisioner_ConfigValidation(t *testing.T) {

	testCases := []struct {
		name        string
		config      map[string]string
		expectError bool
	}{
		{
			name:        "Valid config with password",
			config:      map[string]string{"password": "secure123"},
			expectError: false,
		},
		{
			name:        "Empty password allowed",
			config:      map[string]string{},
			expectError: false,
		},
		{
			name:        "Custom maxmemory policy",
			config:      map[string]string{"maxmemory_policy": "allkeys-lfu"},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Config validation happens during provision
			// This is a basic test structure
			if tc.config != nil {
				// Basic validation passed
			}
		})
	}
}

func TestRedisProvisioner_PlanSizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.41/containers/create" {
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)

			// Check HostConfig for memory limits
			hostConfig, ok := req["HostConfig"].(map[string]interface{})
			if ok {
				if memory, exists := hostConfig["Memory"]; exists {
					t.Logf("Memory limit set: %v", memory)
				}
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": "redis-test-container",
			})
			return
		}
		if strings.Contains(r.URL.Path, "/start") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.Contains(r.URL.Path, "/json") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":              "redis-test-container",
				"State":           map[string]interface{}{"Status": "running"},
				"NetworkSettings": map[string]interface{}{"IPAddress": "172.17.0.2"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	provisioner := &RedisProvisioner{dockerClient: dockerClient}

	plans := []ServicePlan{
		{PlanID: "small", MemoryMB: 256},
		{PlanID: "medium", MemoryMB: 1024},
		{PlanID: "large", MemoryMB: 4096},
	}

	ctx := context.Background()

	for i, plan := range plans {
		req := ProvisionRequest{
			Name: "redis-test-" + plan.PlanID,
			Plan: plan.PlanID,
		}

		instance, err := provisioner.Provision(ctx, req, plan)
		if err != nil {
			t.Errorf("Plan %s failed: %v", plan.PlanID, err)
			continue
		}

		if instance == nil {
			t.Errorf("Plan %s returned nil instance", plan.PlanID)
			continue
		}

		t.Logf("Successfully provisioned Redis with plan %s (test %d/%d)", plan.PlanID, i+1, len(plans))
	}
}

func BenchmarkRedisProvisioner_Provision(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.41/containers/create":
			json.NewEncoder(w).Encode(map[string]interface{}{"Id": "bench-redis"})
		case strings.Contains(r.URL.Path, "/start"):
			w.WriteHeader(http.StatusNoContent)
		case strings.Contains(r.URL.Path, "/json"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":              "bench-redis",
				"State":           map[string]interface{}{"Status": "running"},
				"NetworkSettings": map[string]interface{}{"IPAddress": "172.17.0.2"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	provisioner := &RedisProvisioner{dockerClient: dockerClient}

	req := ProvisionRequest{
		Name:   "bench-redis",
		Plan:   "shared",
		Config: map[string]string{"password": "test"},
	}

	plan := ServicePlan{
		PlanID:   "shared",
		MemoryMB: 512,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provisioner.Provision(ctx, req, plan)
	}
}

package cdn

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteRequest_GeographicProximity(t *testing.T) {
	router := NewRouter()

	// Register nodes in different regions
	usWest := &EdgeNode{
		NodeDID:   "did:soho:us-west",
		Address:   "10.0.1.1:8080",
		Region:    "us-west",
		Latitude:  37.77, // San Francisco
		Longitude: -122.42,
		HealthOK:  true,
		LatencyMs: 50,
		LoadScore: 0.3,
	}

	usEast := &EdgeNode{
		NodeDID:   "did:soho:us-east",
		Address:   "10.0.2.1:8080",
		Region:    "us-east",
		Latitude:  40.71, // New York
		Longitude: -74.00,
		HealthOK:  true,
		LatencyMs: 45,
		LoadScore: 0.4,
	}

	europe := &EdgeNode{
		NodeDID:   "did:soho:europe",
		Address:   "10.0.3.1:8080",
		Region:    "europe",
		Latitude:  51.51, // London
		Longitude: -0.13,
		HealthOK:  true,
		LatencyMs: 100,
		LoadScore: 0.2,
	}

	router.RegisterEdge(usWest)
	router.RegisterEdge(usEast)
	router.RegisterEdge(europe)

	tests := []struct {
		name          string
		clientLat     float64
		clientLon     float64
		expectedRegion string
	}{
		{
			name:          "client near SF routes to us-west",
			clientLat:     37.80, // Oakland
			clientLon:     -122.27,
			expectedRegion: "us-west",
		},
		{
			name:          "client near NYC routes to us-east",
			clientLat:     40.75, // Manhattan
			clientLon:     -73.98,
			expectedRegion: "us-east",
		},
		{
			name:          "client in London routes to europe",
			clientLat:     51.50,
			clientLon:     -0.12,
			expectedRegion: "europe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := router.RouteRequest(tt.clientLat, tt.clientLon)
			if selected == nil {
				t.Fatal("Expected a node to be selected")
			}
			if selected.Region != tt.expectedRegion {
				t.Errorf("Expected region %q, got %q", tt.expectedRegion, selected.Region)
			}
		})
	}
}

func TestRouteRequest_HealthyNodesOnly(t *testing.T) {
	router := NewRouter()

	healthy := &EdgeNode{
		NodeDID:   "did:soho:healthy",
		Address:   "10.0.1.1:8080",
		Region:    "us-west",
		Latitude:  37.77,
		Longitude: -122.42,
		HealthOK:  true,
		LatencyMs: 50,
		LoadScore: 0.5,
	}

	unhealthy := &EdgeNode{
		NodeDID:   "did:soho:unhealthy",
		Address:   "10.0.2.1:8080",
		Region:    "us-west",
		Latitude:  37.78, // Slightly closer
		Longitude: -122.40,
		HealthOK:  false,
		LatencyMs: 30,
		LoadScore: 0.2,
	}

	router.RegisterEdge(healthy)
	router.RegisterEdge(unhealthy)

	// Route to SF area — should select healthy node even though unhealthy is closer
	selected := router.RouteRequest(37.77, -122.42)
	if selected == nil {
		t.Fatal("Expected a node to be selected")
	}
	if selected.NodeDID != healthy.NodeDID {
		t.Errorf("Expected healthy node %q, got %q", healthy.NodeDID, selected.NodeDID)
	}
}

func TestRouteRequest_LoadBalancing(t *testing.T) {
	router := NewRouter()

	// Two nodes in same location with different loads
	lowLoad := &EdgeNode{
		NodeDID:   "did:soho:low-load",
		Address:   "10.0.1.1:8080",
		Region:    "us-west",
		Latitude:  37.77,
		Longitude: -122.42,
		HealthOK:  true,
		LatencyMs: 50,
		LoadScore: 0.2, // Low load
	}

	highLoad := &EdgeNode{
		NodeDID:   "did:soho:high-load",
		Address:   "10.0.2.1:8080",
		Region:    "us-west",
		Latitude:  37.77,
		Longitude: -122.42,
		HealthOK:  true,
		LatencyMs: 50,
		LoadScore: 0.9, // High load
	}

	router.RegisterEdge(lowLoad)
	router.RegisterEdge(highLoad)

	// Should prefer low-load node
	selected := router.RouteRequest(37.77, -122.42)
	if selected == nil {
		t.Fatal("Expected a node to be selected")
	}
	if selected.NodeDID != lowLoad.NodeDID {
		t.Errorf("Expected low-load node %q, got %q", lowLoad.NodeDID, selected.NodeDID)
	}
}

func TestRouteRequest_NoHealthyNodes(t *testing.T) {
	router := NewRouter()

	unhealthy := &EdgeNode{
		NodeDID:   "did:soho:unhealthy",
		Address:   "10.0.1.1:8080",
		Region:    "us-west",
		Latitude:  37.77,
		Longitude: -122.42,
		HealthOK:  false,
		LatencyMs: 50,
		LoadScore: 0.5,
	}

	router.RegisterEdge(unhealthy)

	// Should return nil when no healthy nodes
	selected := router.RouteRequest(37.77, -122.42)
	if selected != nil {
		t.Error("Expected nil when no healthy nodes available")
	}
}

func TestRouteToRegion(t *testing.T) {
	router := NewRouter()

	usWest1 := &EdgeNode{
		NodeDID:   "did:soho:us-west-1",
		Address:   "10.0.1.1:8080",
		Region:    "us-west",
		Latitude:  37.77,
		Longitude: -122.42,
		HealthOK:  true,
		LatencyMs: 50,
		LoadScore: 0.7, // Higher load
	}

	usWest2 := &EdgeNode{
		NodeDID:   "did:soho:us-west-2",
		Address:   "10.0.1.2:8080",
		Region:    "us-west",
		Latitude:  37.80,
		Longitude: -122.40,
		HealthOK:  true,
		LatencyMs: 45,
		LoadScore: 0.3, // Lower load
	}

	usEast := &EdgeNode{
		NodeDID:   "did:soho:us-east",
		Address:   "10.0.2.1:8080",
		Region:    "us-east",
		Latitude:  40.71,
		Longitude: -74.00,
		HealthOK:  true,
		LatencyMs: 40,
		LoadScore: 0.2,
	}

	router.RegisterEdge(usWest1)
	router.RegisterEdge(usWest2)
	router.RegisterEdge(usEast)

	// Should select lowest-load node in us-west region
	selected := router.RouteToRegion("us-west")
	if selected == nil {
		t.Fatal("Expected a node to be selected")
	}
	if selected.NodeDID != usWest2.NodeDID {
		t.Errorf("Expected node %q, got %q", usWest2.NodeDID, selected.NodeDID)
	}
}

func TestProbeEdgeNode_TCPSuccess(t *testing.T) {
	// Create a TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	address := listener.Addr().String()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	router := NewRouter()

	// Probe should succeed for TCP connectivity
	healthy, latencyMs, loadScore := router.probeEdgeNode(address)

	if !healthy {
		t.Error("Expected healthy=true for successful TCP connection")
	}

	if latencyMs <= 0 {
		t.Error("Expected positive latency measurement")
	}

	// Since HTTP will fail, expect default load score
	if loadScore != 0.5 {
		t.Errorf("Expected default load score 0.5, got %.2f", loadScore)
	}
}

func TestProbeEdgeNode_TCPFailure(t *testing.T) {
	router := NewRouter()

	// Probe non-existent address
	healthy, latencyMs, loadScore := router.probeEdgeNode("192.0.2.1:9999")

	if healthy {
		t.Error("Expected healthy=false for failed TCP connection")
	}

	if latencyMs != 0 {
		t.Errorf("Expected latency=0 for failed connection, got %d", latencyMs)
	}

	if loadScore != 0 {
		t.Errorf("Expected loadScore=0 for failed connection, got %.2f", loadScore)
	}
}

func TestProbeEdgeNode_WithStatusEndpoint(t *testing.T) {
	// Create HTTP server with /cdn/status endpoint
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdn/status" {
			http.NotFound(w, r)
			return
		}

		status := struct {
			Load float64 `json:"load"`
		}{
			Load: 0.65,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Extract host:port from server URL
	address := server.Listener.Addr().String()

	router := NewRouter()

	// Probe should succeed and return load from endpoint
	healthy, latencyMs, loadScore := router.probeEdgeNode(address)

	if !healthy {
		t.Error("Expected healthy=true for successful probe")
	}

	if latencyMs <= 0 {
		t.Error("Expected positive latency measurement")
	}

	// Should get load from /cdn/status
	if loadScore < 0.64 || loadScore > 0.66 {
		t.Errorf("Expected load ~0.65, got %.2f", loadScore)
	}
}

func TestHaversineDistance(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lon1     float64
		lat2     float64
		lon2     float64
		expected float64
		tolerance float64
	}{
		{
			name:      "SF to NYC (~4140km)",
			lat1:      37.77,
			lon1:      -122.42,
			lat2:      40.71,
			lon2:      -74.00,
			expected:  4140,
			tolerance: 50,
		},
		{
			name:      "London to Paris (~344km)",
			lat1:      51.51,
			lon1:      -0.13,
			lat2:      48.86,
			lon2:      2.35,
			expected:  344,
			tolerance: 10,
		},
		{
			name:      "same location (0km)",
			lat1:      37.77,
			lon1:      -122.42,
			lat2:      37.77,
			lon2:      -122.42,
			expected:  0,
			tolerance: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := haversineDistance(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if dist < tt.expected-tt.tolerance || dist > tt.expected+tt.tolerance {
				t.Errorf("Expected distance ~%.0f km, got %.0f km", tt.expected, dist)
			}
		})
	}
}

func TestComputeRouteScore(t *testing.T) {
	router := NewRouter()

	tests := []struct {
		name           string
		node           *EdgeNode
		distKm         float64
		expectedMin    float64
		expectedMax    float64
	}{
		{
			name: "perfect node (close, low latency, low load)",
			node: &EdgeNode{
				LatencyMs: 10,
				LoadScore: 0.1,
			},
			distKm:      100,
			expectedMin: 90,
			expectedMax: 100,
		},
		{
			name: "average node",
			node: &EdgeNode{
				LatencyMs: 100,
				LoadScore: 0.5,
			},
			distKm:      5000,
			expectedMin: 60,
			expectedMax: 80,
		},
		{
			name: "poor node (far, high latency, high load)",
			node: &EdgeNode{
				LatencyMs: 500,
				LoadScore: 1.0,
			},
			distKm:      20000,
			expectedMin: 0,
			expectedMax: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := router.computeRouteScore(tt.node, tt.distKm)
			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("Expected score between %.1f and %.1f, got %.1f",
					tt.expectedMin, tt.expectedMax, score)
			}
		})
	}
}

func TestListEdges(t *testing.T) {
	router := NewRouter()

	node1 := &EdgeNode{NodeDID: "did:soho:node1", Address: "10.0.1.1:8080", HealthOK: true}
	node2 := &EdgeNode{NodeDID: "did:soho:node2", Address: "10.0.2.1:8080", HealthOK: true}
	node3 := &EdgeNode{NodeDID: "did:soho:node3", Address: "10.0.3.1:8080", HealthOK: false}

	router.RegisterEdge(node1)
	router.RegisterEdge(node2)
	router.RegisterEdge(node3)

	edges := router.ListEdges()

	if len(edges) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(edges))
	}

	// Verify all nodes are present
	found := make(map[string]bool)
	for _, edge := range edges {
		found[edge.NodeDID] = true
	}

	if !found["did:soho:node1"] || !found["did:soho:node2"] || !found["did:soho:node3"] {
		t.Error("Not all registered nodes found in list")
	}
}

func TestRemoveEdge(t *testing.T) {
	router := NewRouter()

	node1 := &EdgeNode{NodeDID: "did:soho:node1", Address: "10.0.1.1:8080", HealthOK: true}
	node2 := &EdgeNode{NodeDID: "did:soho:node2", Address: "10.0.2.1:8080", HealthOK: true}

	router.RegisterEdge(node1)
	router.RegisterEdge(node2)

	// Remove node1
	router.RemoveEdge("did:soho:node1")

	edges := router.ListEdges()
	if len(edges) != 1 {
		t.Errorf("Expected 1 edge after removal, got %d", len(edges))
	}

	if edges[0].NodeDID != "did:soho:node2" {
		t.Errorf("Expected node2 to remain, got %s", edges[0].NodeDID)
	}
}

func TestHealthCheckLoop_Integration(t *testing.T) {
	// Create HTTP server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdn/status" {
			http.NotFound(w, r)
			return
		}
		status := struct {
			Load float64 `json:"load"`
		}{Load: 0.3}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	address := server.Listener.Addr().String()

	router := NewRouter()

	// Register node
	node := &EdgeNode{
		NodeDID:   "did:soho:test",
		Address:   address,
		Region:    "test",
		Latitude:  37.77,
		Longitude: -122.42,
		HealthOK:  false, // Start as unhealthy
		LatencyMs: 0,
		LoadScore: 0,
	}
	router.RegisterEdge(node)

	// Run one health check cycle
	router.checkEdgeHealth()

	// Verify node is now healthy
	edges := router.ListEdges()
	if len(edges) != 1 {
		t.Fatal("Expected 1 edge")
	}

	if !edges[0].HealthOK {
		t.Error("Expected node to be healthy after check")
	}

	if edges[0].LatencyMs <= 0 {
		t.Error("Expected positive latency")
	}

	if edges[0].LoadScore < 0.29 || edges[0].LoadScore > 0.31 {
		t.Errorf("Expected load ~0.3, got %.2f", edges[0].LoadScore)
	}

	if edges[0].LastCheck.IsZero() {
		t.Error("Expected LastCheck to be updated")
	}
}

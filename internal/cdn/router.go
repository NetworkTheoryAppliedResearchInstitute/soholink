package cdn

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

// EdgeNode represents a CDN edge node in the federation.
type EdgeNode struct {
	NodeDID    string
	Address    string
	Region     string
	Latitude   float64
	Longitude  float64
	Cache      *EdgeCache
	HealthOK   bool
	LatencyMs  int
	LoadScore  float64
	LastCheck  time.Time
}

// Router performs intelligent geo-based routing to the nearest healthy edge node.
type Router struct {
	mu    sync.RWMutex
	nodes map[string]*EdgeNode
}

// NewRouter creates a new CDN router.
func NewRouter() *Router {
	return &Router{
		nodes: make(map[string]*EdgeNode),
	}
}

// RegisterEdge adds an edge node to the routing table.
func (r *Router) RegisterEdge(node *EdgeNode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[node.NodeDID] = node
	log.Printf("[cdn] registered edge node %s (%s, lat=%.2f, lon=%.2f)",
		node.NodeDID, node.Region, node.Latitude, node.Longitude)
}

// RemoveEdge removes an edge node from the routing table.
func (r *Router) RemoveEdge(nodeDID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.nodes, nodeDID)
}

// RouteRequest selects the best edge node for serving a request based on
// geographic proximity, health, and current load.
func (r *Router) RouteRequest(clientLat, clientLon float64) *EdgeNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []*edgeCandidate
	for _, node := range r.nodes {
		if !node.HealthOK {
			continue
		}
		dist := haversineDistance(clientLat, clientLon, node.Latitude, node.Longitude)
		score := r.computeRouteScore(node, dist)
		candidates = append(candidates, &edgeCandidate{node: node, score: score, distance: dist})
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by score (higher = better)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	return candidates[0].node
}

// RouteToRegion selects the best edge node in a specific region.
func (r *Router) RouteToRegion(region string) *EdgeNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *EdgeNode
	bestLoad := math.MaxFloat64

	for _, node := range r.nodes {
		if !node.HealthOK || node.Region != region {
			continue
		}
		if node.LoadScore < bestLoad {
			best = node
			bestLoad = node.LoadScore
		}
	}

	return best
}

// ListEdges returns all registered edge nodes.
func (r *Router) ListEdges() []*EdgeNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*EdgeNode, 0, len(r.nodes))
	for _, node := range r.nodes {
		result = append(result, node)
	}
	return result
}

// HealthCheckLoop periodically checks the health of all edge nodes.
func (r *Router) HealthCheckLoop(done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkEdgeHealth()
		case <-done:
			return
		}
	}
}

func (r *Router) checkEdgeHealth() {
	r.mu.RLock()
	nodes := make([]*EdgeNode, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	r.mu.RUnlock()

	for _, node := range nodes {
		healthy, latency, load := r.probeEdgeNode(node.Address)

		r.mu.Lock()
		node.HealthOK = healthy
		node.LastCheck = time.Now()
		if healthy {
			node.LatencyMs = latency
			node.LoadScore = load
		}
		r.mu.Unlock()
	}
}

// probeEdgeNode performs an active health check on an edge node.
// It makes a TCP connection to measure latency and optionally queries
// the /cdn/status endpoint for load information.
func (r *Router) probeEdgeNode(address string) (healthy bool, latencyMs int, loadScore float64) {
	start := time.Now()

	// TCP connectivity check to measure RTT
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return false, 0, 0
	}
	conn.Close()
	latencyMs = int(time.Since(start).Milliseconds())

	// Query /cdn/status for load information
	statusURL := fmt.Sprintf("http://%s/cdn/status", address)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(statusURL)
	if err != nil {
		// TCP worked but HTTP didn't — still mark as healthy with default load
		return true, latencyMs, 0.5
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return true, latencyMs, 0.5
	}

	var status struct {
		Load float64 `json:"load"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return true, latencyMs, 0.5
	}

	return true, latencyMs, status.Load
}

type edgeCandidate struct {
	node     *EdgeNode
	score    float64
	distance float64
}

// computeRouteScore combines proximity, latency, and load into a routing score.
func (r *Router) computeRouteScore(node *EdgeNode, distKm float64) float64 {
	// Distance score: closer is better (normalized to ~0-100 for distances up to 20000km)
	distScore := 100.0 * (1.0 - math.Min(distKm/20000.0, 1.0))

	// Latency score: lower is better
	latencyScore := 100.0
	if node.LatencyMs > 0 {
		latencyScore = 100.0 * (1.0 - math.Min(float64(node.LatencyMs)/500.0, 1.0))
	}

	// Load score: lower load is better
	loadScore := 100.0 * (1.0 - math.Min(node.LoadScore, 1.0))

	// Weighted combination
	return (distScore * 0.40) + (latencyScore * 0.35) + (loadScore * 0.25)
}

// haversineDistance calculates the great-circle distance between two
// points on Earth given their latitude and longitude in degrees.
// Returns distance in kilometers.
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)

	lat1Rad := degreesToRadians(lat1)
	lat2Rad := degreesToRadians(lat2)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1Rad)*math.Cos(lat2Rad)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}

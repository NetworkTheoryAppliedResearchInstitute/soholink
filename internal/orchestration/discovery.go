package orchestration

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// NodeDiscovery maintains an up-to-date view of available federation nodes
// and answers queries for workload placement.
type NodeDiscovery struct {
	store *store.Store

	mu    sync.RWMutex
	nodes map[string]*Node
}

// NewNodeDiscovery creates a new discovery service.
func NewNodeDiscovery(s *store.Store) *NodeDiscovery {
	return &NodeDiscovery{
		store: s,
		nodes: make(map[string]*Node),
	}
}

// FindNodes returns nodes matching the given query criteria.
func (d *NodeDiscovery) FindNodes(ctx context.Context, query NodeQuery) ([]*Node, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []*Node
	for _, node := range d.nodes {
		if !d.matchesQuery(node, query) {
			continue
		}
		result = append(result, node)
	}

	// Fallback: query the store if in-memory cache is empty
	if len(result) == 0 {
		rows, err := d.store.GetOnlineNodes(ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			node := nodeFromRow(&row)
			if d.matchesQuery(node, query) {
				result = append(result, node)
			}
		}
	}

	return result, nil
}

// DiscoverLoop periodically refreshes the in-memory node cache.
func (d *NodeDiscovery) DiscoverLoop(ctx context.Context) {
	// Initial load
	d.refresh(ctx)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.refresh(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (d *NodeDiscovery) refresh(ctx context.Context) {
	rows, err := d.store.GetOnlineNodes(ctx)
	if err != nil {
		log.Printf("[discovery] refresh error: %v", err)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fresh := make(map[string]*Node, len(rows))
	for _, row := range rows {
		fresh[row.NodeDID] = nodeFromRow(&row)
	}
	d.nodes = fresh
}

func (d *NodeDiscovery) matchesQuery(node *Node, q NodeQuery) bool {
	if node.Status != "online" {
		return false
	}
	if time.Since(node.LastHeartbeat) > 60*time.Second {
		return false
	}
	if node.AvailableCPU < q.MinCPU {
		return false
	}
	if node.AvailableMemoryMB < q.MinMemory {
		return false
	}
	if q.MinDisk > 0 && node.AvailableDiskGB < q.MinDisk {
		return false
	}
	if q.GPURequired && !node.HasGPU {
		return false
	}
	if q.GPUModel != "" && node.GPUModel != q.GPUModel {
		return false
	}
	if q.MinReputation > 0 && node.ReputationScore < q.MinReputation {
		return false
	}
	if q.MaxCostPerHour > 0 && node.PricePerCPUHour > q.MaxCostPerHour {
		return false
	}
	if len(q.Regions) > 0 {
		found := false
		for _, r := range q.Regions {
			if node.Region == r {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func nodeFromRow(row *store.FederationNodeRow) *Node {
	return &Node{
		DID:               row.NodeDID,
		Address:           row.Address,
		Region:            row.Region,
		TotalCPU:          row.TotalCPU,
		AvailableCPU:      row.AvailableCPU,
		TotalMemoryMB:     row.TotalMemoryMB,
		AvailableMemoryMB: row.AvailableMemoryMB,
		TotalDiskGB:       row.TotalDiskGB,
		AvailableDiskGB:   row.AvailableDiskGB,
		HasGPU:            row.GPUModel != "",
		GPUModel:          row.GPUModel,
		PricePerCPUHour:   row.PricePerCPUHour,
		ReputationScore:   row.ReputationScore,
		UptimePercent:     row.UptimePercent,
		FailureRate:       row.FailureRate,
		Status:            row.Status,
		LastHeartbeat:     row.LastHeartbeat,
	}
}

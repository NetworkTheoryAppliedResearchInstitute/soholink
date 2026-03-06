package store

// federation.go adds federation-coordinator-specific store methods on top of
// the core FederationNodeRow type and UpsertFederationNode method that already
// live in enterprise.go.  It does NOT redefine those; it only adds:
//   - UpdateFederationHeartbeat  — lightweight heartbeat / resource refresh
//   - SetFederationNodeOffline   — mark a node offline on clean deregister
//   - ListActiveFederationNodes  — recently-heartbeating online nodes
//   - GetFederationNode          — single-node lookup by DID

import (
	"context"
	"time"
)

// UpdateFederationHeartbeat refreshes a node's available resources and
// last_heartbeat timestamp. Called on every provider heartbeat.
func (s *Store) UpdateFederationHeartbeat(
	ctx context.Context,
	nodeDID string,
	availCPU float64,
	availMemMB, availDiskGB int64,
) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE federation_nodes
		SET available_cpu        = ?,
		    available_memory_mb  = ?,
		    available_disk_gb    = ?,
		    status               = 'online',
		    last_heartbeat       = ?
		WHERE node_did = ?`,
		availCPU, availMemMB, availDiskGB, time.Now().UTC(), nodeDID)
	return err
}

// SetFederationNodeOffline marks a node as offline (called on clean deregister
// or when a node misses too many heartbeats).
func (s *Store) SetFederationNodeOffline(ctx context.Context, nodeDID string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE federation_nodes SET status = 'offline' WHERE node_did = ?", nodeDID)
	return err
}

// ListActiveFederationNodes returns all nodes that sent a heartbeat within
// the last 2 minutes (considered online), ordered by reputation score.
func (s *Store) ListActiveFederationNodes(ctx context.Context) ([]FederationNodeRow, error) {
	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	rows, err := s.db.QueryContext(ctx, `
		SELECT node_did, address, region,
		       total_cpu, available_cpu,
		       total_memory_mb, available_memory_mb,
		       total_disk_gb, available_disk_gb,
		       gpu_model, price_per_cpu_hour,
		       reputation_score, uptime_percent, failure_rate,
		       status, last_heartbeat, public_key
		FROM   federation_nodes
		WHERE  last_heartbeat >= ? AND status = 'online'
		ORDER  BY reputation_score DESC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FederationNodeRow
	for rows.Next() {
		var n FederationNodeRow
		if err := rows.Scan(
			&n.NodeDID, &n.Address, &n.Region,
			&n.TotalCPU, &n.AvailableCPU,
			&n.TotalMemoryMB, &n.AvailableMemoryMB,
			&n.TotalDiskGB, &n.AvailableDiskGB,
			&n.GPUModel, &n.PricePerCPUHour,
			&n.ReputationScore, &n.UptimePercent, &n.FailureRate,
			&n.Status, &n.LastHeartbeat, &n.PublicKey,
		); err != nil {
			continue
		}
		result = append(result, n)
	}
	if result == nil {
		result = []FederationNodeRow{}
	}
	return result, rows.Err()
}

// GetFederationNode returns a single node by DID, or nil if not found.
func (s *Store) GetFederationNode(ctx context.Context, nodeDID string) (*FederationNodeRow, error) {
	var n FederationNodeRow
	err := s.db.QueryRowContext(ctx, `
		SELECT node_did, address, region,
		       total_cpu, available_cpu,
		       total_memory_mb, available_memory_mb,
		       total_disk_gb, available_disk_gb,
		       gpu_model, price_per_cpu_hour,
		       reputation_score, uptime_percent, failure_rate,
		       status, last_heartbeat, public_key
		FROM   federation_nodes WHERE node_did = ?`, nodeDID).Scan(
		&n.NodeDID, &n.Address, &n.Region,
		&n.TotalCPU, &n.AvailableCPU,
		&n.TotalMemoryMB, &n.AvailableMemoryMB,
		&n.TotalDiskGB, &n.AvailableDiskGB,
		&n.GPUModel, &n.PricePerCPUHour,
		&n.ReputationScore, &n.UptimePercent, &n.FailureRate,
		&n.Status, &n.LastHeartbeat, &n.PublicKey,
	)
	if err != nil {
		return nil, nil // not found or scan error
	}
	return &n, nil
}

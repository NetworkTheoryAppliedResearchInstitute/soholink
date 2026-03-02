package orchestration

import (
	"context"
	"fmt"
)

// ListWorkloads returns all active workload states.
func (s *FedScheduler) ListWorkloads() []*WorkloadState {
	return s.ListActiveWorkloads()
}

// GetWorkload returns the state for a single workload, or nil if not found.
func (s *FedScheduler) GetWorkload(workloadID string) *WorkloadState {
	return s.GetWorkloadState(workloadID)
}

// ScaleWorkload enqueues a scale event for the given workload.
func (s *FedScheduler) ScaleWorkload(ctx context.Context, workloadID string, replicas int) error {
	s.mu.RLock()
	_, ok := s.ActiveWorkloads[workloadID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("workload %s not found", workloadID)
	}
	s.scalingQueue <- ScaleEvent{WorkloadID: workloadID, TargetReplicas: replicas}
	return nil
}

// DeleteWorkload removes a workload from the active set.
func (s *FedScheduler) DeleteWorkload(ctx context.Context, workloadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.ActiveWorkloads[workloadID]; !ok {
		return fmt.Errorf("workload %s not found", workloadID)
	}
	delete(s.ActiveWorkloads, workloadID)
	return nil
}

// UpdateWorkload applies configuration updates to a workload.
func (s *FedScheduler) UpdateWorkload(ctx context.Context, workloadID string, updates map[string]interface{}) error {
	s.mu.RLock()
	_, ok := s.ActiveWorkloads[workloadID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("workload %s not found", workloadID)
	}
	// TODO: apply updates to workload spec
	return nil
}

// RestartWorkload re-queues a workload for rescheduling.
func (s *FedScheduler) RestartWorkload(ctx context.Context, workloadID string) error {
	s.mu.Lock()
	ws, ok := s.ActiveWorkloads[workloadID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("workload %s not found", workloadID)
	}
	w := ws.Workload
	delete(s.ActiveWorkloads, workloadID)
	s.mu.Unlock()

	s.SubmitWorkload(w)
	return nil
}

// GetWorkloadLogs returns recent log lines for a workload.
// Currently returns a stub message; real implementation requires node agent integration.
func (s *FedScheduler) GetWorkloadLogs(ctx context.Context, workloadID string, tailLines int, follow bool) (string, error) {
	s.mu.RLock()
	_, ok := s.ActiveWorkloads[workloadID]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("workload %s not found", workloadID)
	}
	return fmt.Sprintf("[%s] Log collection requires node agent integration\n", workloadID), nil
}

// GetWorkloadMetrics returns metrics for a workload.
// Currently returns the placement and health data; real metrics require node agent integration.
func (s *FedScheduler) GetWorkloadMetrics(ctx context.Context, workloadID string) (interface{}, error) {
	ws := s.GetWorkloadState(workloadID)
	if ws == nil {
		return nil, fmt.Errorf("workload %s not found", workloadID)
	}
	return map[string]interface{}{
		"workload_id": workloadID,
		"health":      ws.Health,
		"placements":  len(ws.Placements),
	}, nil
}

// GetWorkloadEvents returns recent events for a workload.
func (s *FedScheduler) GetWorkloadEvents(ctx context.Context, workloadID string) (interface{}, error) {
	ws := s.GetWorkloadState(workloadID)
	if ws == nil {
		return nil, fmt.Errorf("workload %s not found", workloadID)
	}
	return []map[string]interface{}{
		{
			"type":    "info",
			"message": fmt.Sprintf("Workload %s is %s", workloadID, ws.Workload.Status),
		},
	}, nil
}

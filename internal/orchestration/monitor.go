package orchestration

import (
	"context"
	"log"
	"time"
)

// WorkloadMonitor tracks the health and metrics of active workloads
// and triggers re-scheduling when placements fail.
type WorkloadMonitor struct {
	scheduler *FedScheduler
}

// NewWorkloadMonitor creates a new monitor.
func NewWorkloadMonitor(s *FedScheduler) *WorkloadMonitor {
	return &WorkloadMonitor{scheduler: s}
}

// MonitorLoop periodically checks the health of all active placements.
func (m *WorkloadMonitor) MonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAll(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (m *WorkloadMonitor) checkAll(_ context.Context) {
	m.scheduler.mu.RLock()
	defer m.scheduler.mu.RUnlock()

	for _, ws := range m.scheduler.ActiveWorkloads {
		healthy := 0
		for _, p := range ws.Placements {
			if p.Status == "running" {
				healthy++
			}
		}

		if healthy == 0 && len(ws.Placements) > 0 {
			ws.Health = HealthStatus{Status: "unhealthy", Details: "no healthy replicas"}
			log.Printf("[monitor] workload %s UNHEALTHY — no healthy replicas", ws.Workload.WorkloadID)
		} else if healthy < len(ws.Placements) {
			ws.Health = HealthStatus{Status: "degraded", Details: "some replicas unhealthy"}
		} else {
			ws.Health = HealthStatus{Status: "healthy"}
		}
	}
}

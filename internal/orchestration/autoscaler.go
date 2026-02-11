package orchestration

import (
	"context"
	"log"
	"sync"
	"time"
)

// AutoScaler evaluates workload metrics against auto-scale policies
// and emits scale events when thresholds are crossed.
type AutoScaler struct {
	scheduler *FedScheduler
	monitor   *WorkloadMonitor

	mu                sync.Mutex
	scaleUpCooldown   map[string]time.Time
	scaleDownCooldown map[string]time.Time
}

// NewAutoScaler creates a new auto-scaler.
func NewAutoScaler(s *FedScheduler, m *WorkloadMonitor) *AutoScaler {
	return &AutoScaler{
		scheduler:         s,
		monitor:           m,
		scaleUpCooldown:   make(map[string]time.Time),
		scaleDownCooldown: make(map[string]time.Time),
	}
}

// EvaluateLoop periodically checks workloads for scaling needs.
func (a *AutoScaler) EvaluateLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.evaluateAll(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (a *AutoScaler) evaluateAll(ctx context.Context) {
	a.scheduler.mu.RLock()
	workloads := make([]*WorkloadState, 0, len(a.scheduler.ActiveWorkloads))
	for _, ws := range a.scheduler.ActiveWorkloads {
		workloads = append(workloads, ws)
	}
	a.scheduler.mu.RUnlock()

	for _, ws := range workloads {
		_ = a.Evaluate(ctx, ws)
	}
}

// Evaluate checks a single workload's metrics and decides whether to scale.
func (a *AutoScaler) Evaluate(_ context.Context, ws *WorkloadState) error {
	w := ws.Workload
	if w.AutoScale == nil || !w.AutoScale.Enabled {
		return nil
	}

	metrics := ws.Metrics
	currentReplicas := len(ws.Placements)
	targetReplicas := currentReplicas

	a.mu.Lock()
	defer a.mu.Unlock()

	// Scale UP if CPU exceeds target
	if metrics.AvgCPUPercent > w.AutoScale.TargetCPU {
		target := int(float64(currentReplicas) * 1.5)
		if target < currentReplicas+1 {
			target = currentReplicas + 1
		}
		if target > w.AutoScale.MaxReplicas {
			target = w.AutoScale.MaxReplicas
		}

		if lastUp, ok := a.scaleUpCooldown[w.WorkloadID]; ok {
			if time.Since(lastUp) < w.AutoScale.ScaleUpCooldown {
				return nil
			}
		}

		targetReplicas = target
		a.scaleUpCooldown[w.WorkloadID] = time.Now()
		log.Printf("[autoscaler] scale UP %s: %d → %d (CPU=%.1f%%)", w.WorkloadID, currentReplicas, targetReplicas, metrics.AvgCPUPercent*100)

	} else if metrics.AvgCPUPercent < w.AutoScale.TargetCPU*0.5 && currentReplicas > w.AutoScale.MinReplicas {
		// Scale DOWN when significantly under target
		target := int(float64(currentReplicas) * 0.7)
		if target < w.AutoScale.MinReplicas {
			target = w.AutoScale.MinReplicas
		}

		if lastDown, ok := a.scaleDownCooldown[w.WorkloadID]; ok {
			if time.Since(lastDown) < w.AutoScale.ScaleDownCooldown {
				return nil
			}
		}

		targetReplicas = target
		a.scaleDownCooldown[w.WorkloadID] = time.Now()
		log.Printf("[autoscaler] scale DOWN %s: %d → %d (CPU=%.1f%%)", w.WorkloadID, currentReplicas, targetReplicas, metrics.AvgCPUPercent*100)
	}

	if targetReplicas != currentReplicas {
		select {
		case a.scheduler.scalingQueue <- ScaleEvent{WorkloadID: w.WorkloadID, TargetReplicas: targetReplicas}:
		default:
		}
	}

	return nil
}

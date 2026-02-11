package orchestration

import (
	"math"
)

// ScoreWeights controls the relative importance of each scoring dimension.
type ScoreWeights struct {
	Cost        float64 // default 0.30
	Latency     float64 // default 0.20
	Reputation  float64 // default 0.20
	Capacity    float64 // default 0.15
	Reliability float64 // default 0.15
}

// Placer scores and ranks federation nodes for workload placement.
type Placer struct {
	weights ScoreWeights
}

// NewPlacer creates a placer with default weights.
func NewPlacer() *Placer {
	return &Placer{
		weights: ScoreWeights{
			Cost:        0.30,
			Latency:     0.20,
			Reputation:  0.20,
			Capacity:    0.15,
			Reliability: 0.15,
		},
	}
}

// ScoreNodes returns a map of node DID → composite score (0–100).
func (p *Placer) ScoreNodes(nodes []*Node, w *Workload) map[string]float64 {
	scores := make(map[string]float64, len(nodes))

	for _, node := range nodes {
		costScore := p.scoreCost(node.PricePerCPUHour, w.Constraints.MaxCostPerHour)
		latencyScore := p.scoreLatency(node.LatencyMs, w.Constraints.MaxLatencyMs)
		reputationScore := float64(node.ReputationScore)
		capacityScore := p.scoreCapacity(node.AvailableCPU, node.AvailableMemoryMB, w.Spec)
		reliabilityScore := p.scoreReliability(node.UptimePercent, node.FailureRate)

		total := (costScore * p.weights.Cost) +
			(latencyScore * p.weights.Latency) +
			(reputationScore * p.weights.Reputation) +
			(capacityScore * p.weights.Capacity) +
			(reliabilityScore * p.weights.Reliability)

		scores[node.DID] = total
	}

	return scores
}

// scoreCost: lower price → higher score.
func (p *Placer) scoreCost(nodePrice, maxPrice int64) float64 {
	if maxPrice <= 0 {
		maxPrice = 1000 // default $10/h
	}
	if nodePrice > maxPrice {
		return 0
	}
	return 100.0 * (1.0 - float64(nodePrice)/float64(maxPrice))
}

// scoreLatency: lower latency → higher score.
func (p *Placer) scoreLatency(nodeLatency, maxLatency int) float64 {
	if maxLatency <= 0 {
		return 100.0
	}
	if nodeLatency > maxLatency {
		return 0
	}
	return 100.0 * (1.0 - float64(nodeLatency)/float64(maxLatency))
}

// scoreCapacity: more headroom → higher score (capped at 100).
func (p *Placer) scoreCapacity(availCPU float64, availMem int64, spec WorkloadSpec) float64 {
	if spec.CPUCores <= 0 {
		return 50
	}
	cpuHeadroom := (availCPU - spec.CPUCores) / spec.CPUCores
	memHeadroom := 0.0
	if spec.MemoryMB > 0 {
		memHeadroom = float64(availMem-spec.MemoryMB) / float64(spec.MemoryMB)
	}
	avg := (cpuHeadroom + memHeadroom) / 2.0
	score := 50.0 + (avg * 25.0)
	return math.Max(0, math.Min(100, score))
}

// scoreReliability: higher uptime + lower failure rate → higher score.
func (p *Placer) scoreReliability(uptimePercent, failureRate float64) float64 {
	uptimeScore := uptimePercent / 2.0
	failureScore := 50.0 * (1.0 - math.Min(failureRate/0.1, 1.0))
	return uptimeScore + failureScore
}

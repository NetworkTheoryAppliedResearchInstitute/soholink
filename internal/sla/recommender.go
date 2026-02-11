package sla

import (
	"log"
	"sort"
)

// Recommendation suggests an SLA tier based on workload requirements.
type Recommendation struct {
	Tier              TierLevel
	Reason            string
	EstimatedCostDay  int64
	UptimeGuarantee   float64
	LatencyGuarantee  int
	Confidence        float64
}

// WorkloadProfile describes the characteristics of a workload for SLA recommendation.
type WorkloadProfile struct {
	Type             string  // "web", "api", "batch", "database", "streaming"
	CriticalityLevel int     // 1-5 (1=low, 5=mission-critical)
	AvgRequestsPerSec float64
	PeakRequestsPerSec float64
	DataSensitivity  string  // "public", "internal", "confidential", "restricted"
	AvailabilityNeed string  // "best-effort", "business-hours", "always-on"
	LatencyTolerance int     // max acceptable latency in ms
	BudgetPerDay     int64   // max budget in cents per day
}

// Recommender analyzes workload profiles and suggests appropriate SLA tiers.
type Recommender struct {
	templates map[TierLevel]ContractTemplate
}

// NewRecommender creates a new SLA recommender with default templates.
func NewRecommender() *Recommender {
	return &Recommender{
		templates: DefaultContracts(),
	}
}

// Recommend returns a ranked list of SLA tier recommendations for a workload profile.
func (r *Recommender) Recommend(profile WorkloadProfile) []Recommendation {
	var recs []Recommendation

	for tier, tmpl := range r.templates {
		rec := r.evaluateTier(profile, tier, tmpl)
		if rec != nil {
			recs = append(recs, *rec)
		}
	}

	// Sort by confidence (descending)
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Confidence > recs[j].Confidence
	})

	if len(recs) > 0 {
		log.Printf("[sla] recommended tier %s for workload type=%s criticality=%d (confidence=%.2f)",
			recs[0].Tier, profile.Type, profile.CriticalityLevel, recs[0].Confidence)
	}

	return recs
}

// RecommendSingle returns the best single recommendation.
func (r *Recommender) RecommendSingle(profile WorkloadProfile) *Recommendation {
	recs := r.Recommend(profile)
	if len(recs) == 0 {
		return nil
	}
	return &recs[0]
}

func (r *Recommender) evaluateTier(profile WorkloadProfile, tier TierLevel, tmpl ContractTemplate) *Recommendation {
	confidence := 0.0
	reason := ""

	// Criticality alignment
	switch {
	case profile.CriticalityLevel >= 5 && tier == TierEnterprise:
		confidence += 0.35
		reason = "mission-critical workload requires enterprise SLA"
	case profile.CriticalityLevel >= 4 && tier == TierPremium:
		confidence += 0.30
		reason = "high-criticality workload benefits from premium SLA"
	case profile.CriticalityLevel >= 3 && tier == TierStandard:
		confidence += 0.30
		reason = "moderate-criticality workload suits standard SLA"
	case profile.CriticalityLevel <= 2 && tier == TierBasic:
		confidence += 0.30
		reason = "low-criticality workload matched to basic SLA"
	default:
		confidence += 0.10
	}

	// Latency alignment
	if profile.LatencyTolerance > 0 {
		if tmpl.LatencyTargetMs <= profile.LatencyTolerance {
			confidence += 0.20
		} else {
			confidence -= 0.10
		}
	}

	// Availability alignment
	switch profile.AvailabilityNeed {
	case "always-on":
		if tmpl.UptimeTarget >= 99.9 {
			confidence += 0.20
		}
	case "business-hours":
		if tmpl.UptimeTarget >= 99.0 {
			confidence += 0.15
		}
	default:
		confidence += 0.10
	}

	// Data sensitivity
	switch profile.DataSensitivity {
	case "restricted", "confidential":
		if tier == TierPremium || tier == TierEnterprise {
			confidence += 0.15
		}
	}

	// Budget check
	estimatedCost := estimateDailyCost(tier)
	if profile.BudgetPerDay > 0 && estimatedCost > profile.BudgetPerDay {
		confidence -= 0.30
		reason += " (over budget)"
	}

	if confidence <= 0 {
		return nil
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return &Recommendation{
		Tier:              tier,
		Reason:            reason,
		EstimatedCostDay:  estimatedCost,
		UptimeGuarantee:   tmpl.UptimeTarget,
		LatencyGuarantee:  tmpl.LatencyTargetMs,
		Confidence:        confidence,
	}
}

func estimateDailyCost(tier TierLevel) int64 {
	switch tier {
	case TierBasic:
		return 100 // $1/day
	case TierStandard:
		return 500 // $5/day
	case TierPremium:
		return 2000 // $20/day
	case TierEnterprise:
		return 10000 // $100/day
	default:
		return 100
	}
}

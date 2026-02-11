package sla

import (
	"fmt"
	"time"
)

// TierLevel defines the SLA tier.
type TierLevel string

const (
	TierBasic      TierLevel = "basic"
	TierStandard   TierLevel = "standard"
	TierPremium    TierLevel = "premium"
	TierEnterprise TierLevel = "enterprise"
)

// Contract represents a Service Level Agreement between a consumer and the federation.
type Contract struct {
	ContractID       string
	OwnerDID         string
	Tier             TierLevel
	Status           string // active, suspended, expired
	UptimeTarget     float64
	LatencyTargetMs  int
	SupportResponse  time.Duration
	MonthlyCost      int64 // monthly cost in cents for credit computation
	CreditPolicy     CreditPolicy
	PenaltyPolicy    PenaltyPolicy
	WorkloadIDs      []string
	ServiceIDs       []string
	StartDate        time.Time
	EndDate          time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CreditPolicy defines automatic compensation when SLA targets are missed.
type CreditPolicy struct {
	// Credits as percentage of monthly bill
	TierCredits map[string]float64 // e.g. "99.0-99.9" → 10% credit
	MaxCredit   float64            // max credit cap (e.g. 30% of monthly bill)
}

// PenaltyPolicy defines penalties for repeated SLA breaches.
type PenaltyPolicy struct {
	MaxBreachesPerMonth int
	AutoSuspendAfter    int
	NotifyThreshold     int
}

// Violation records a detected SLA breach.
type Violation struct {
	ViolationID   string
	ContractID    string
	Type          string // "uptime", "latency", "error_rate"
	Description   string
	Severity      string // "minor", "major", "critical"
	MeasuredValue float64
	TargetValue   float64
	Duration      time.Duration
	CreditAmount  int64 // computed credit in cents
	DetectedAt    time.Time
	ResolvedAt    time.Time
}

// MonthlyReport summarizes SLA performance for a billing period.
type MonthlyReport struct {
	ContractID     string
	Period         string // "2025-07"
	UptimeActual   float64
	UptimTarget    float64
	AvgLatencyMs   float64
	LatencyTarget  int
	Violations     int
	CreditEarned   int64 // total credit in cents
	CreditApplied  bool
	GeneratedAt    time.Time
}

// DefaultContracts returns the predefined SLA tier definitions.
func DefaultContracts() map[TierLevel]ContractTemplate {
	return map[TierLevel]ContractTemplate{
		TierBasic: {
			Tier:            TierBasic,
			UptimeTarget:    99.0,
			LatencyTargetMs: 200,
			SupportResponse: 24 * time.Hour,
			CreditPolicy: CreditPolicy{
				TierCredits: map[string]float64{
					"98.0-99.0": 5.0,
					"95.0-98.0": 10.0,
					"<95.0":     25.0,
				},
				MaxCredit: 25.0,
			},
		},
		TierStandard: {
			Tier:            TierStandard,
			UptimeTarget:    99.5,
			LatencyTargetMs: 100,
			SupportResponse: 4 * time.Hour,
			CreditPolicy: CreditPolicy{
				TierCredits: map[string]float64{
					"99.0-99.5": 10.0,
					"98.0-99.0": 20.0,
					"<98.0":     30.0,
				},
				MaxCredit: 30.0,
			},
		},
		TierPremium: {
			Tier:            TierPremium,
			UptimeTarget:    99.9,
			LatencyTargetMs: 50,
			SupportResponse: 1 * time.Hour,
			CreditPolicy: CreditPolicy{
				TierCredits: map[string]float64{
					"99.5-99.9": 10.0,
					"99.0-99.5": 25.0,
					"<99.0":     50.0,
				},
				MaxCredit: 50.0,
			},
		},
		TierEnterprise: {
			Tier:            TierEnterprise,
			UptimeTarget:    99.99,
			LatencyTargetMs: 20,
			SupportResponse: 15 * time.Minute,
			CreditPolicy: CreditPolicy{
				TierCredits: map[string]float64{
					"99.9-99.99": 10.0,
					"99.5-99.9":  30.0,
					"<99.5":      100.0,
				},
				MaxCredit: 100.0,
			},
		},
	}
}

// ContractTemplate is a reusable SLA tier definition.
type ContractTemplate struct {
	Tier            TierLevel
	UptimeTarget    float64
	LatencyTargetMs int
	SupportResponse time.Duration
	CreditPolicy    CreditPolicy
}

// NewContract creates a new SLA contract from a template.
func NewContract(contractID, ownerDID string, template ContractTemplate, duration time.Duration) *Contract {
	now := time.Now()
	return &Contract{
		ContractID:      contractID,
		OwnerDID:        ownerDID,
		Tier:            template.Tier,
		Status:          "active",
		UptimeTarget:    template.UptimeTarget,
		LatencyTargetMs: template.LatencyTargetMs,
		SupportResponse: template.SupportResponse,
		CreditPolicy:    template.CreditPolicy,
		PenaltyPolicy: PenaltyPolicy{
			MaxBreachesPerMonth: 10,
			AutoSuspendAfter:    5,
			NotifyThreshold:     3,
		},
		StartDate: now,
		EndDate:   now.Add(duration),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsActive returns true if the contract is currently active and not expired.
func (c *Contract) IsActive() bool {
	return c.Status == "active" && time.Now().Before(c.EndDate)
}

// Validate checks that a contract has sensible values.
func (c *Contract) Validate() error {
	if c.UptimeTarget < 90 || c.UptimeTarget > 100 {
		return fmt.Errorf("uptime target must be between 90%% and 100%%")
	}
	if c.LatencyTargetMs <= 0 {
		return fmt.Errorf("latency target must be positive")
	}
	if c.EndDate.Before(c.StartDate) {
		return fmt.Errorf("end date must be after start date")
	}
	return nil
}

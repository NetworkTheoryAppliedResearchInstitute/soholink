package sla

import (
	"testing"
	"time"
)

func TestComputeCredit_TieredSystem(t *testing.T) {
	monitor := &Monitor{
		contracts: make(map[string]*Contract),
		history:   make(map[string][]Violation),
	}

	tests := []struct {
		name           string
		tier           TierLevel
		monthlyCost    int64
		actualUptime   float64
		expectedCredit int64
	}{
		// Premium tier tests (99.9% target)
		{
			name:           "premium tier - 99.9%+ uptime (no credit)",
			tier:           TierPremium,
			monthlyCost:    10000, // $100.00
			actualUptime:   99.95,
			expectedCredit: 0,
		},
		{
			name:           "premium tier - 99.5-99.9% uptime (10% credit)",
			tier:           TierPremium,
			monthlyCost:    10000, // $100.00
			actualUptime:   99.7,
			expectedCredit: 1000, // $10.00
		},
		{
			name:           "premium tier - 99.0-99.5% uptime (25% credit)",
			tier:           TierPremium,
			monthlyCost:    10000, // $100.00
			actualUptime:   99.3,
			expectedCredit: 2500, // $25.00
		},
		{
			name:           "premium tier - <99.0% uptime (50% credit, maxed)",
			tier:           TierPremium,
			monthlyCost:    10000, // $100.00
			actualUptime:   98.5,
			expectedCredit: 5000, // $50.00 (capped at MaxCredit 50%)
		},
		// Standard tier tests (99.5% target)
		{
			name:           "standard tier - 99.5%+ uptime (no credit)",
			tier:           TierStandard,
			monthlyCost:    5000, // $50.00
			actualUptime:   99.6,
			expectedCredit: 0,
		},
		{
			name:           "standard tier - 99.0-99.5% uptime (10% credit)",
			tier:           TierStandard,
			monthlyCost:    5000, // $50.00
			actualUptime:   99.2,
			expectedCredit: 500, // $5.00
		},
		{
			name:           "standard tier - 98.0-99.0% uptime (20% credit)",
			tier:           TierStandard,
			monthlyCost:    5000, // $50.00
			actualUptime:   98.5,
			expectedCredit: 1000, // $10.00
		},
		{
			name:           "standard tier - <98.0% uptime (30% credit, maxed)",
			tier:           TierStandard,
			monthlyCost:    5000, // $50.00
			actualUptime:   95.0,
			expectedCredit: 1500, // $15.00 (capped at MaxCredit 30%)
		},
		// Basic tier tests (99.0% target)
		{
			name:           "basic tier - 99.0%+ uptime (no credit)",
			tier:           TierBasic,
			monthlyCost:    2000, // $20.00
			actualUptime:   99.5,
			expectedCredit: 0,
		},
		{
			name:           "basic tier - 98.0-99.0% uptime (5% credit)",
			tier:           TierBasic,
			monthlyCost:    2000, // $20.00
			actualUptime:   98.5,
			expectedCredit: 100, // $1.00
		},
		{
			name:           "basic tier - 95.0-98.0% uptime (10% credit)",
			tier:           TierBasic,
			monthlyCost:    2000, // $20.00
			actualUptime:   96.0,
			expectedCredit: 200, // $2.00
		},
		{
			name:           "basic tier - <95.0% uptime (25% credit, maxed)",
			tier:           TierBasic,
			monthlyCost:    2000, // $20.00
			actualUptime:   90.0,
			expectedCredit: 500, // $5.00 (capped at MaxCredit 25%)
		},
		// Enterprise tier tests (99.99% target)
		{
			name:           "enterprise tier - 99.99%+ uptime (no credit)",
			tier:           TierEnterprise,
			monthlyCost:    50000, // $500.00
			actualUptime:   99.995,
			expectedCredit: 0,
		},
		{
			name:           "enterprise tier - 99.9-99.99% uptime (10% credit)",
			tier:           TierEnterprise,
			monthlyCost:    50000, // $500.00
			actualUptime:   99.95,
			expectedCredit: 5000, // $50.00
		},
		{
			name:           "enterprise tier - 99.5-99.9% uptime (30% credit)",
			tier:           TierEnterprise,
			monthlyCost:    50000, // $500.00
			actualUptime:   99.7,
			expectedCredit: 15000, // $150.00
		},
		{
			name:           "enterprise tier - <99.5% uptime (100% refund)",
			tier:           TierEnterprise,
			monthlyCost:    50000, // $500.00
			actualUptime:   99.0,
			expectedCredit: 50000, // $500.00 (full refund, capped at MaxCredit 100%)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create contract with specified tier
			template := DefaultContracts()[tt.tier]
			contract := NewContract("test-contract", "did:soho:test", template, 30*24*time.Hour)
			contract.MonthlyCost = tt.monthlyCost

			// Compute credit
			actualCredit := monitor.computeCredit(contract, tt.actualUptime)

			// Verify credit amount
			if actualCredit != tt.expectedCredit {
				t.Errorf("Expected credit %d cents ($%.2f), got %d cents ($%.2f)",
					tt.expectedCredit, float64(tt.expectedCredit)/100.0,
					actualCredit, float64(actualCredit)/100.0)
			}
		})
	}
}

func TestComputeLatencyCredit(t *testing.T) {
	monitor := &Monitor{
		contracts: make(map[string]*Contract),
		history:   make(map[string][]Violation),
	}

	tests := []struct {
		name              string
		monthlyCost       int64
		latencyTargetMs   int
		actualLatencyMs   int
		maxCreditPercent  float64
		expectedCredit    int64
		expectedCreditMin int64 // Allow some tolerance
		expectedCreditMax int64
	}{
		{
			name:              "latency at target (no credit)",
			monthlyCost:       10000, // $100.00
			latencyTargetMs:   50,
			actualLatencyMs:   50,
			maxCreditPercent:  50.0,
			expectedCredit:    0,
			expectedCreditMin: 0,
			expectedCreditMax: 0,
		},
		{
			name:              "latency 2x target (10% overage → 10% credit)",
			monthlyCost:       10000, // $100.00
			latencyTargetMs:   100,
			actualLatencyMs:   200, // 100% overage → 10% credit
			maxCreditPercent:  50.0,
			expectedCredit:    1000, // $10.00
			expectedCreditMin: 1000,
			expectedCreditMax: 1000,
		},
		{
			name:              "latency 3x target (20% overage → 20% credit)",
			monthlyCost:       10000, // $100.00
			latencyTargetMs:   50,
			actualLatencyMs:   150, // 200% overage → 20% credit
			maxCreditPercent:  50.0,
			expectedCredit:    2000, // $20.00
			expectedCreditMin: 2000,
			expectedCreditMax: 2000,
		},
		{
			name:              "latency 10x target (capped at max credit)",
			monthlyCost:       10000, // $100.00
			latencyTargetMs:   20,
			actualLatencyMs:   200, // 900% overage → would be 90% credit, capped at 50%
			maxCreditPercent:  50.0,
			expectedCredit:    5000, // $50.00 (capped)
			expectedCreditMin: 5000,
			expectedCreditMax: 5000,
		},
		{
			name:              "small latency violation (10% overage → 1% credit)",
			monthlyCost:       5000, // $50.00
			latencyTargetMs:   100,
			actualLatencyMs:   110, // 10% overage → 1% credit
			maxCreditPercent:  30.0,
			expectedCredit:    50, // $0.50
			expectedCreditMin: 50,
			expectedCreditMax: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create contract
			contract := &Contract{
				MonthlyCost:     tt.monthlyCost,
				LatencyTargetMs: tt.latencyTargetMs,
				CreditPolicy: CreditPolicy{
					MaxCredit: tt.maxCreditPercent,
				},
			}

			// Compute credit
			actualCredit := monitor.computeLatencyCredit(contract, tt.actualLatencyMs)

			// Verify credit amount (with tolerance)
			if actualCredit < tt.expectedCreditMin || actualCredit > tt.expectedCreditMax {
				t.Errorf("Expected credit between %d and %d cents, got %d cents",
					tt.expectedCreditMin, tt.expectedCreditMax, actualCredit)
			}
		})
	}
}

func TestMatchesRange(t *testing.T) {
	tests := []struct {
		name      string
		rangeStr  string
		value     float64
		shouldMatch bool
	}{
		// Range tests
		{"range 99.0-99.9 with 99.5", "99.0-99.9", 99.5, true},
		{"range 99.0-99.9 with 99.0 (inclusive)", "99.0-99.9", 99.0, true},
		{"range 99.0-99.9 with 99.9 (exclusive)", "99.0-99.9", 99.9, false},
		{"range 99.0-99.9 with 98.5", "99.0-99.9", 98.5, false},
		{"range 99.0-99.9 with 100", "99.0-99.9", 100.0, false},

		// Less-than tests
		{"<99.0 with 98.5", "<99.0", 98.5, true},
		{"<99.0 with 99.0 (exclusive)", "<99.0", 99.0, false},
		{"<99.0 with 99.5", "<99.0", 99.5, false},
		{"<95.0 with 90.0", "<95.0", 90.0, true},

		// Edge cases
		{"range 95.0-98.0 with 95.0", "95.0-98.0", 95.0, true},
		{"range 95.0-98.0 with 97.999", "95.0-98.0", 97.999, true},
		{"range 95.0-98.0 with 98.0", "95.0-98.0", 98.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesRange(tt.rangeStr, tt.value)
			if result != tt.shouldMatch {
				t.Errorf("matchesRange(%q, %.3f) = %v, want %v",
					tt.rangeStr, tt.value, result, tt.shouldMatch)
			}
		})
	}
}

func TestCheckUptime_ViolationDetection(t *testing.T) {
	monitor := NewMonitor(nil)

	// Create a premium contract (99.9% target)
	template := DefaultContracts()[TierPremium]
	contract := NewContract("contract-123", "did:soho:owner", template, 30*24*time.Hour)
	contract.MonthlyCost = 10000 // $100/month
	monitor.RegisterContract(contract)

	tests := []struct {
		name               string
		actualUptime       float64
		expectViolation    bool
		expectedSeverity   string
		expectedCreditCents int64
	}{
		{
			name:            "uptime above target (no violation)",
			actualUptime:    99.95,
			expectViolation: false,
		},
		{
			name:                "uptime slightly below target (minor violation)",
			actualUptime:        99.7,
			expectViolation:     true,
			expectedSeverity:    "minor",
			expectedCreditCents: 1000, // 10% of $100
		},
		{
			name:                "uptime well below target (major violation)",
			actualUptime:        98.5,
			expectViolation:     true,
			expectedSeverity:    "major",
			expectedCreditCents: 2500, // 25% of $100
		},
		{
			name:                "uptime far below target (critical violation)",
			actualUptime:        93.0,
			expectViolation:     true,
			expectedSeverity:    "critical",
			expectedCreditCents: 5000, // 50% of $100 (maxed)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear history
			monitor.mu.Lock()
			monitor.history = make(map[string][]Violation)
			monitor.mu.Unlock()

			// Check uptime
			monitor.CheckUptime("contract-123", tt.actualUptime)

			// Get violations
			violations := monitor.GetViolations("contract-123")

			if tt.expectViolation {
				if len(violations) == 0 {
					t.Fatal("Expected violation but got none")
				}

				violation := violations[0]

				if violation.Severity != tt.expectedSeverity {
					t.Errorf("Expected severity %q, got %q", tt.expectedSeverity, violation.Severity)
				}

				if violation.CreditAmount != tt.expectedCreditCents {
					t.Errorf("Expected credit %d cents, got %d cents",
						tt.expectedCreditCents, violation.CreditAmount)
				}

				if violation.Type != "uptime" {
					t.Errorf("Expected type 'uptime', got %q", violation.Type)
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violation but got %d", len(violations))
				}
			}
		})
	}
}

func TestCheckLatency_ViolationDetection(t *testing.T) {
	monitor := NewMonitor(nil)

	// Create a premium contract (50ms latency target)
	template := DefaultContracts()[TierPremium]
	contract := NewContract("contract-123", "did:soho:owner", template, 30*24*time.Hour)
	contract.MonthlyCost = 10000 // $100/month
	monitor.RegisterContract(contract)

	tests := []struct {
		name             string
		actualLatencyMs  int
		expectViolation  bool
		expectedSeverity string
	}{
		{
			name:            "latency at target (no violation)",
			actualLatencyMs: 50,
			expectViolation: false,
		},
		{
			name:             "latency slightly above target (minor)",
			actualLatencyMs:  75,
			expectViolation:  true,
			expectedSeverity: "minor",
		},
		{
			name:             "latency 2x target (major)",
			actualLatencyMs:  100,
			expectViolation:  true,
			expectedSeverity: "major",
		},
		{
			name:             "latency 3x+ target (critical)",
			actualLatencyMs:  200,
			expectViolation:  true,
			expectedSeverity: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear history
			monitor.mu.Lock()
			monitor.history = make(map[string][]Violation)
			monitor.mu.Unlock()

			// Check latency
			monitor.CheckLatency("contract-123", tt.actualLatencyMs)

			// Get violations
			violations := monitor.GetViolations("contract-123")

			if tt.expectViolation {
				if len(violations) == 0 {
					t.Fatal("Expected violation but got none")
				}

				violation := violations[0]

				if violation.Severity != tt.expectedSeverity {
					t.Errorf("Expected severity %q, got %q", tt.expectedSeverity, violation.Severity)
				}

				if violation.Type != "latency" {
					t.Errorf("Expected type 'latency', got %q", violation.Type)
				}

				if violation.CreditAmount <= 0 {
					t.Error("Expected positive credit amount for latency violation")
				}
			} else {
				if len(violations) > 0 {
					t.Errorf("Expected no violation but got %d", len(violations))
				}
			}
		})
	}
}

func TestClassifyUptimeSeverity(t *testing.T) {
	tests := []struct {
		name             string
		actual           float64
		target           float64
		expectedSeverity string
	}{
		{"minor gap (0.5%)", 99.4, 99.9, "minor"},
		{"minor gap (1.0%)", 98.9, 99.9, "minor"},
		{"major gap (2.0%)", 97.9, 99.9, "major"},
		{"major gap (5.0%)", 94.9, 99.9, "major"},
		{"critical gap (6.0%)", 93.9, 99.9, "critical"},
		{"critical gap (10.0%)", 89.9, 99.9, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := classifyUptimeSeverity(tt.actual, tt.target)
			if severity != tt.expectedSeverity {
				t.Errorf("Expected severity %q, got %q", tt.expectedSeverity, severity)
			}
		})
	}
}

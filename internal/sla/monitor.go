package sla

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// Monitor continuously tracks SLA metrics and detects violations.
type Monitor struct {
	store *store.Store

	mu        sync.RWMutex
	contracts map[string]*Contract
	history   map[string][]Violation // contractID → violations

	violationCh chan Violation
}

// NewMonitor creates a new SLA monitor.
func NewMonitor(s *store.Store) *Monitor {
	return &Monitor{
		store:       s,
		contracts:   make(map[string]*Contract),
		history:     make(map[string][]Violation),
		violationCh: make(chan Violation, 100),
	}
}

// RegisterContract adds a contract to be monitored.
func (m *Monitor) RegisterContract(c *Contract) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contracts[c.ContractID] = c
	log.Printf("[sla] monitoring contract %s (tier=%s, uptime=%.2f%%)", c.ContractID, c.Tier, c.UptimeTarget)
}

// GetContract returns a monitored contract by ID.
func (m *Monitor) GetContract(contractID string) *Contract {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.contracts[contractID]
}

// ViolationChan returns a channel that emits detected violations.
func (m *Monitor) ViolationChan() <-chan Violation {
	return m.violationCh
}

// CheckUptime evaluates the actual uptime percentage against the SLA target.
func (m *Monitor) CheckUptime(contractID string, actualUptime float64) {
	m.mu.RLock()
	contract, ok := m.contracts[contractID]
	m.mu.RUnlock()
	if !ok || !contract.IsActive() {
		return
	}

	if actualUptime < contract.UptimeTarget {
		violation := Violation{
			ViolationID:   generateViolationID(),
			ContractID:    contractID,
			Type:          "uptime",
			Description:   "Uptime below SLA target",
			MeasuredValue: actualUptime,
			TargetValue:   contract.UptimeTarget,
			DetectedAt:    time.Now(),
		}
		violation.Severity = classifyUptimeSeverity(actualUptime, contract.UptimeTarget)
		violation.CreditAmount = m.computeCredit(contract, actualUptime)

		m.recordViolation(violation)
	}
}

// CheckLatency evaluates the measured latency against the SLA target.
func (m *Monitor) CheckLatency(contractID string, actualLatencyMs int) {
	m.mu.RLock()
	contract, ok := m.contracts[contractID]
	m.mu.RUnlock()
	if !ok || !contract.IsActive() {
		return
	}

	if actualLatencyMs > contract.LatencyTargetMs {
		violation := Violation{
			ViolationID:   generateViolationID(),
			ContractID:    contractID,
			Type:          "latency",
			Description:   "Latency above SLA target",
			MeasuredValue: float64(actualLatencyMs),
			TargetValue:   float64(contract.LatencyTargetMs),
			Severity:      "minor",
			DetectedAt:    time.Now(),
		}

		if actualLatencyMs > contract.LatencyTargetMs*3 {
			violation.Severity = "critical"
		} else if actualLatencyMs >= contract.LatencyTargetMs*2 {
			violation.Severity = "major"
		}

		violation.CreditAmount = m.computeLatencyCredit(contract, actualLatencyMs)
		m.recordViolation(violation)
	}
}

// GetViolations returns all violations for a contract.
func (m *Monitor) GetViolations(contractID string) []Violation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.history[contractID]
}

// MonitorLoop periodically evaluates all contracts.
func (m *Monitor) MonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.evaluateAll(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Monitor) evaluateAll(_ context.Context) {
	m.mu.RLock()
	contracts := make([]*Contract, 0, len(m.contracts))
	for _, c := range m.contracts {
		if c.IsActive() {
			contracts = append(contracts, c)
		}
	}
	m.mu.RUnlock()

	for _, c := range contracts {
		// Check penalty thresholds
		m.mu.RLock()
		violations := m.history[c.ContractID]
		m.mu.RUnlock()

		recentCount := 0
		monthAgo := time.Now().Add(-30 * 24 * time.Hour)
		for _, v := range violations {
			if v.DetectedAt.After(monthAgo) {
				recentCount++
			}
		}

		if recentCount >= c.PenaltyPolicy.AutoSuspendAfter {
			log.Printf("[sla] contract %s: auto-suspend threshold reached (%d violations in 30 days)",
				c.ContractID, recentCount)
		} else if recentCount >= c.PenaltyPolicy.NotifyThreshold {
			log.Printf("[sla] contract %s: approaching breach threshold (%d/%d violations in 30 days)",
				c.ContractID, recentCount, c.PenaltyPolicy.AutoSuspendAfter)
		}
	}
}

func (m *Monitor) recordViolation(v Violation) {
	m.mu.Lock()
	m.history[v.ContractID] = append(m.history[v.ContractID], v)
	m.mu.Unlock()

	select {
	case m.violationCh <- v:
	default:
	}

	log.Printf("[sla] VIOLATION: contract=%s type=%s severity=%s measured=%.2f target=%.2f",
		v.ContractID, v.Type, v.Severity, v.MeasuredValue, v.TargetValue)
}

func (m *Monitor) computeCredit(contract *Contract, actualUptime float64) int64 {
	if contract.MonthlyCost <= 0 {
		return 0
	}
	// Match against tiered credit policy — creditPercent is a percentage of monthly cost
	for rangeStr, creditPercent := range contract.CreditPolicy.TierCredits {
		if matchesRange(rangeStr, actualUptime) {
			credit := int64(float64(contract.MonthlyCost) * creditPercent / 100.0)
			// Cap at max credit percentage
			maxCredit := int64(float64(contract.MonthlyCost) * contract.CreditPolicy.MaxCredit / 100.0)
			if credit > maxCredit {
				credit = maxCredit
			}
			return credit
		}
	}
	return 0
}

// computeLatencyCredit calculates credit for latency SLA violations.
// Credit is proportional to the overage percentage above the target.
func (m *Monitor) computeLatencyCredit(contract *Contract, actualLatencyMs int) int64 {
	if contract.MonthlyCost <= 0 || contract.LatencyTargetMs <= 0 {
		return 0
	}
	overage := float64(actualLatencyMs-contract.LatencyTargetMs) / float64(contract.LatencyTargetMs)
	creditPercent := overage * 10.0
	if creditPercent > contract.CreditPolicy.MaxCredit {
		creditPercent = contract.CreditPolicy.MaxCredit
	}
	return int64(float64(contract.MonthlyCost) * creditPercent / 100.0)
}

func matchesRange(rangeStr string, value float64) bool {
	// Simple range matching: "99.0-99.5" means value is in [99.0, 99.5)
	// "<98.0" means value < 98.0
	if len(rangeStr) > 0 && rangeStr[0] == '<' {
		var threshold float64
		if ok := parseFloat(rangeStr[1:], &threshold); !ok {
			return false
		}
		return value < threshold
	}

	var low, high float64
	n := 0
	for i, ch := range rangeStr {
		if ch == '-' && i > 0 {
			if parsedOK := parseFloatInto(rangeStr[:i], &low); !parsedOK {
				return false
			}
			if parsedOK := parseFloatInto(rangeStr[i+1:], &high); !parsedOK {
				return false
			}
			n = 2
			break
		}
	}
	if n == 2 {
		return value >= low && value < high
	}
	return false
}

func parseFloat(s string, out *float64) bool {
	return parseFloatInto(s, out)
}

func parseFloatInto(s string, out *float64) bool {
	var val float64
	var decimal float64
	dotSeen := false
	divisor := 1.0

	for _, ch := range s {
		if ch == '.' {
			if dotSeen {
				return false
			}
			dotSeen = true
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
		digit := float64(ch - '0')
		if dotSeen {
			divisor *= 10
			decimal += digit / divisor
		} else {
			val = val*10 + digit
		}
	}
	*out = val + decimal
	return true
}

func classifyUptimeSeverity(actual, target float64) string {
	gap := target - actual
	if gap > 5.0 {
		return "critical"
	}
	if gap > 1.0 {
		return "major"
	}
	return "minor"
}

func generateViolationID() string {
	return "vio_" + time.Now().Format("20060102150405.000")
}

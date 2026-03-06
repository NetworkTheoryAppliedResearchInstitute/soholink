package rental

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// AutoAcceptRule defines conditions and actions for automatically handling
// incoming rental requests without operator intervention.
type AutoAcceptRule struct {
	RuleID   string
	RuleName string
	Enabled  bool
	Priority int // Lower = higher priority

	// Conditions
	MinUserScore  int    // Minimum LBTAS score
	MaxAmount     int64  // Max transaction amount (cents)
	ResourceType  string // "compute", "storage", "print", "internet", or ""=any
	AllowedHours  []int  // Hours of day to allow (0-23); empty = all
	AllowedDays   []int  // Days of week (0=Sun, 6=Sat); empty = all
	RequirePrepay bool   // Require payment escrow before acceptance

	// Action
	Action         string // "accept", "reject", "pending"
	NotifyOperator bool
}

// RentalRequest represents an incoming request from a user for a resource.
type RentalRequest struct {
	RequestID        string // optional; generated from timestamp when empty
	UserDID          string
	UserScore        int
	ResourceType     string
	Amount           int64
	HasPaymentEscrow bool
}

// Decision is the engine's ruling on a rental request.
type Decision struct {
	Action  string // "accept", "reject", "pending"
	RuleID  string
	Message string
}

// Notification is sent to the operator when a request requires attention.
type Notification struct {
	Type    string
	Request RentalRequest
	RuleID  string
}

// AutoAcceptEngine evaluates rental requests against a prioritised list
// of rules and returns an accept / reject / pending decision.
type AutoAcceptEngine struct {
	store     *store.Store
	rules     []AutoAcceptRule
	notifyCh  chan Notification
}

// NewAutoAcceptEngine creates a new engine, loading rules from the store.
func NewAutoAcceptEngine(s *store.Store) *AutoAcceptEngine {
	return &AutoAcceptEngine{
		store:    s,
		notifyCh: make(chan Notification, 100),
	}
}

// NotifyChan returns the channel that receives operator notifications.
func (e *AutoAcceptEngine) NotifyChan() <-chan Notification {
	return e.notifyCh
}

// LoadRules loads rules from the store into the engine's in-memory list.
func (e *AutoAcceptEngine) LoadRules(ctx context.Context) error {
	rows, err := e.store.GetAutoAcceptRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to load auto-accept rules: %w", err)
	}

	rules := make([]AutoAcceptRule, 0, len(rows))
	for _, r := range rows {
		rules = append(rules, AutoAcceptRule{
			RuleID:         r.RuleID,
			RuleName:       r.RuleName,
			Enabled:        r.Enabled,
			Priority:       r.Priority,
			MinUserScore:   r.MinUserScore,
			MaxAmount:      r.MaxAmount,
			ResourceType:   r.ResourceType,
			AllowedHours:   parseIntSlice(r.AllowedHoursJSON),
			AllowedDays:    parseIntSlice(r.AllowedDaysJSON),
			RequirePrepay:  r.RequirePrepay,
			Action:         r.Action,
			NotifyOperator: r.NotifyOperator,
		})
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	e.rules = rules
	return nil
}

// EvaluateRequest evaluates a rental request against loaded rules.
// Returns the decision from the first matching rule, or "pending" if no
// rule matches. Every decision is written to the rental_audit table for
// compliance review.
func (e *AutoAcceptEngine) EvaluateRequest(ctx context.Context, req RentalRequest) (Decision, error) {
	// Ensure every request has a stable identifier for audit tracing.
	if req.RequestID == "" {
		req.RequestID = fmt.Sprintf("rr_%d", time.Now().UnixNano())
	}

	var decision Decision

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		if matchesRule(req, rule) {
			if rule.Action == "accept" {
				decision = Decision{
					Action:  "accept",
					RuleID:  rule.RuleID,
					Message: fmt.Sprintf("Auto-accepted by rule %q", rule.RuleName),
				}
				e.writeAudit(ctx, req, decision)
				return decision, nil
			}

			if rule.Action == "pending" {
				if rule.NotifyOperator {
					select {
					case e.notifyCh <- Notification{
						Type:    "rental_approval_needed",
						Request: req,
						RuleID:  rule.RuleID,
					}:
					default:
					}
				}
				decision = Decision{
					Action:  "pending",
					RuleID:  rule.RuleID,
					Message: "Requires operator approval",
				}
				e.writeAudit(ctx, req, decision)
				return decision, nil
			}

			decision = Decision{
				Action:  "reject",
				RuleID:  rule.RuleID,
				Message: fmt.Sprintf("Rejected by rule %q", rule.RuleName),
			}
			e.writeAudit(ctx, req, decision)
			return decision, nil
		}
	}

	// No rule matched — default to require approval.
	decision = Decision{
		Action:  "pending",
		Message: "No matching rule - requires manual review",
	}
	e.writeAudit(ctx, req, decision)
	return decision, nil
}

// writeAudit persists a rental engine decision to the audit log.
func (e *AutoAcceptEngine) writeAudit(ctx context.Context, req RentalRequest, d Decision) {
	if e.store == nil {
		return
	}
	row := &store.RentalAuditRow{
		RequestID: req.RequestID,
		UserDID:   req.UserDID,
		RuleID:    d.RuleID,
		Action:    d.Action,
		Reason:    d.Message,
		DecidedAt: time.Now(),
	}
	if err := e.store.InsertRentalAudit(ctx, row); err != nil {
		log.Printf("[rental] failed to write audit for request %s: %v", req.RequestID, err)
	}
}

// AddRule persists a new auto-accept rule.
func (e *AutoAcceptEngine) AddRule(ctx context.Context, rule AutoAcceptRule) error {
	row := &store.AutoAcceptRuleRow{
		RuleID:           rule.RuleID,
		RuleName:         rule.RuleName,
		Enabled:          rule.Enabled,
		Priority:         rule.Priority,
		MinUserScore:     rule.MinUserScore,
		MaxAmount:        rule.MaxAmount,
		ResourceType:     rule.ResourceType,
		AllowedHoursJSON: formatIntSlice(rule.AllowedHours),
		AllowedDaysJSON:  formatIntSlice(rule.AllowedDays),
		RequirePrepay:    rule.RequirePrepay,
		Action:           rule.Action,
		NotifyOperator:   rule.NotifyOperator,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := e.store.CreateAutoAcceptRule(ctx, row); err != nil {
		return err
	}
	return e.LoadRules(ctx) // Reload
}

// ToggleRule enables or disables a rule.
func (e *AutoAcceptEngine) ToggleRule(ctx context.Context, ruleID string, enabled bool) error {
	if err := e.store.ToggleAutoAcceptRule(ctx, ruleID, enabled); err != nil {
		return err
	}
	return e.LoadRules(ctx)
}

// DeleteRule removes a rule.
func (e *AutoAcceptEngine) DeleteRule(ctx context.Context, ruleID string) error {
	if err := e.store.DeleteAutoAcceptRule(ctx, ruleID); err != nil {
		return err
	}
	return e.LoadRules(ctx)
}

// matchesRule checks whether a request satisfies all conditions of a rule.
func matchesRule(req RentalRequest, rule AutoAcceptRule) bool {
	if req.UserScore < rule.MinUserScore {
		return false
	}
	if rule.MaxAmount > 0 && req.Amount > rule.MaxAmount {
		return false
	}
	if rule.ResourceType != "" && req.ResourceType != rule.ResourceType {
		return false
	}

	// Time-of-day restriction
	if len(rule.AllowedHours) > 0 {
		currentHour := time.Now().Hour()
		found := false
		for _, h := range rule.AllowedHours {
			if currentHour == h {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Day-of-week restriction
	if len(rule.AllowedDays) > 0 {
		currentDay := int(time.Now().Weekday())
		found := false
		for _, d := range rule.AllowedDays {
			if currentDay == d {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Prepayment check
	if rule.RequirePrepay && !req.HasPaymentEscrow {
		return false
	}

	return true
}

// parseIntSlice converts a JSON-encoded int slice string to []int.
func parseIntSlice(s string) []int {
	if s == "" || s == "[]" || s == "null" {
		return nil
	}
	// Simple comma-delimited parser (stored as JSON array e.g. "[9,10,11]")
	var result []int
	var current int
	inNumber := false
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			current = current*10 + int(ch-'0')
			inNumber = true
		} else {
			if inNumber {
				result = append(result, current)
				current = 0
				inNumber = false
			}
		}
	}
	if inNumber {
		result = append(result, current)
	}
	return result
}

// formatIntSlice converts an int slice to a JSON-like string for storage.
func formatIntSlice(vals []int) string {
	if len(vals) == 0 {
		return "[]"
	}
	s := "["
	for i, v := range vals {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%d", v)
	}
	s += "]"
	return s
}

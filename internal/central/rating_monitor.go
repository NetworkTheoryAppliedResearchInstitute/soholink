package central

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// RatingMonitor watches for catastrophic ratings (0/5) and fires alerts.
// When mutual catastrophic ratings occur (both parties rate 0), the alert
// is escalated to critical severity and an automatic dispute is created.
type RatingMonitor struct {
	store    *store.Store
	notifier *Notifier
	disputes *DisputeManager

	// centerDID is the DID of this central SOHO node.
	centerDID string
}

// RatingAlert represents a triggered alert from a catastrophic rating.
type RatingAlert struct {
	AlertID       string
	TransactionID string
	UserDID       string
	ProviderDID   string
	CenterDID     string

	UserRating     int
	ProviderRating int

	AlertType string // "single_catastrophic", "mutual_catastrophic"
	Severity  string // "high", "critical"

	Evidence []byte
	Notes    string

	Status     string // "pending", "investigating", "resolved"
	Resolution string // "refund_user", "payout_provider", "split", "no_action"

	InvestigatedBy string
	InvestigatedAt *time.Time
	ResolvedAt     *time.Time

	CreatedAt time.Time
}

// NewRatingMonitor creates a new rating monitor.
func NewRatingMonitor(s *store.Store, notifier *Notifier, disputes *DisputeManager, centerDID string) *RatingMonitor {
	return &RatingMonitor{
		store:     s,
		notifier:  notifier,
		disputes:  disputes,
		centerDID: centerDID,
	}
}

// ProcessRating evaluates a newly submitted rating and triggers alerts for
// catastrophic scores. Must be called after every rating submission.
func (m *RatingMonitor) ProcessRating(ctx context.Context, transactionID string, ratingScore int, raterRole string, feedback string, evidence []byte) error {
	if ratingScore != 0 {
		return nil // Only act on catastrophic ratings
	}

	log.Printf("[rating-monitor] catastrophic rating detected: Transaction %s (role=%s, score=0)", transactionID, raterRole)

	// Get full transaction
	tx, err := m.store.GetTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}
	if tx == nil {
		return fmt.Errorf("transaction not found: %s", transactionID)
	}

	alertID := fmt.Sprintf("alert_%d", time.Now().UnixNano())

	alert := &store.RatingAlertRow{
		AlertID:       alertID,
		TransactionID: transactionID,
		UserDID:       tx.UserDID,
		ProviderDID:   tx.ProviderDID,
		CenterDID:     m.centerDID,
		AlertType:     "single_catastrophic",
		Severity:      "high",
		Evidence:      evidence,
		Notes:         feedback,
		Status:        "pending",
		CreatedAt:     time.Now(),
	}

	// Check if both parties gave catastrophic ratings (mutual catastrophic)
	otherRating, _ := m.store.GetOtherRating(ctx, transactionID, raterRole)
	if otherRating != nil && otherRating.Score == 0 {
		alert.AlertType = "mutual_catastrophic"
		alert.Severity = "critical"
		log.Printf("[rating-monitor] MUTUAL CATASTROPHIC RATING: Transaction %s", transactionID)

		// Auto-trigger dispute investigation
		if m.disputes != nil {
			_ = m.disputes.CreateDispute(ctx, DisputeRequest{
				TransactionID: transactionID,
				FilerDID:      m.centerDID,
				Reason:        "Mutual catastrophic ratings - automatic investigation",
				Priority:      "critical",
			})
		}
	}

	// Store alert
	if err := m.store.CreateRatingAlert(ctx, alert); err != nil {
		return fmt.Errorf("failed to create rating alert: %w", err)
	}

	// Notify central SOHO operators
	if m.notifier != nil {
		m.notifier.Send(Notification{
			Type:     "catastrophic_rating",
			Severity: alert.Severity,
			Title:    fmt.Sprintf("Catastrophic Rating: %s", alert.AlertType),
			Message:  fmt.Sprintf("Transaction %s received a 0/5 rating", transactionID),
			AlertID:  alertID,
		})
	}

	return nil
}

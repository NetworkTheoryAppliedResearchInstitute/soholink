package central

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/payment"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// DisputeRequest is a request to open a dispute for a transaction.
type DisputeRequest struct {
	TransactionID string
	FilerDID      string
	Reason        string
	Priority      string // "normal", "high", "critical"
}

// Resolution describes how a dispute should be resolved.
type Resolution struct {
	Action      string // "refund_user", "payout_provider", "split_50_50", "no_action"
	Explanation string
}

// DisputeManager handles the investigation and resolution workflow
// for disputed transactions.
type DisputeManager struct {
	store    *store.Store
	notifier *Notifier
	payment  *payment.Ledger
}

// NewDisputeManager creates a new dispute manager.
func NewDisputeManager(s *store.Store, notifier *Notifier, paymentLedger *payment.Ledger) *DisputeManager {
	return &DisputeManager{
		store:    s,
		notifier: notifier,
		payment:  paymentLedger,
	}
}

// CreateDispute opens a new dispute and investigation for a transaction.
func (m *DisputeManager) CreateDispute(ctx context.Context, req DisputeRequest) error {
	now := time.Now()
	disputeID := fmt.Sprintf("disp_%d", now.UnixNano())
	investigationID := fmt.Sprintf("inv_%d", now.UnixNano())

	dispute := &store.DisputeRow{
		DisputeID:     disputeID,
		TransactionID: req.TransactionID,
		FilerDID:      req.FilerDID,
		Reason:        req.Reason,
		Priority:      req.Priority,
		Status:        "open",
		CreatedAt:     now,
	}

	if err := m.store.CreateDispute(ctx, dispute); err != nil {
		return fmt.Errorf("failed to create dispute: %w", err)
	}

	// Create investigation
	deadline := now.Add(7 * 24 * time.Hour) // 7-day resolution deadline
	investigation := &store.InvestigationRow{
		InvestigationID: investigationID,
		DisputeID:       disputeID,
		Status:          "investigating",
		CreatedAt:       now,
		Deadline:        deadline,
	}

	if err := m.store.CreateInvestigation(ctx, investigation); err != nil {
		return fmt.Errorf("failed to create investigation: %w", err)
	}

	// Update the underlying transaction's state and dispute reference
	if err := m.store.SetTransactionDispute(ctx, req.TransactionID, disputeID, req.Reason); err != nil {
		log.Printf("[dispute] failed to update transaction dispute ref: %v", err)
	}

	// Notify parties
	tx, _ := m.store.GetTransaction(ctx, req.TransactionID)
	if tx != nil && m.notifier != nil {
		m.notifier.Send(Notification{
			Type:    "dispute_opened",
			Message: fmt.Sprintf("A dispute has been opened for transaction %s.", req.TransactionID),
		})
	}

	log.Printf("[dispute] created dispute %s for transaction %s (priority=%s)", disputeID, req.TransactionID, req.Priority)
	return nil
}

// ResolveDispute carries out the resolution and notifies both parties.
func (m *DisputeManager) ResolveDispute(ctx context.Context, disputeID string, decision Resolution) error {
	dispute, err := m.store.GetDispute(ctx, disputeID)
	if err != nil {
		return fmt.Errorf("failed to get dispute: %w", err)
	}
	if dispute == nil {
		return fmt.Errorf("dispute not found: %s", disputeID)
	}

	tx, err := m.store.GetTransaction(ctx, dispute.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}
	if tx == nil {
		return fmt.Errorf("transaction not found for dispute: %s", dispute.TransactionID)
	}

	// Execute financial resolution
	if m.payment != nil {
		switch decision.Action {
		case "refund_user":
			_ = m.payment.RefundEscrow(ctx, tx.PaymentProof, tx.UserDID)
		case "payout_provider":
			_ = m.payment.ReleaseEscrow(ctx, tx.PaymentProof, tx.ProviderDID)
		case "split_50_50":
			// Split: release half to provider, refund half to user
			// In production this would use a SplitEscrow method
			_ = m.payment.ReleaseEscrow(ctx, tx.PaymentProof, tx.ProviderDID)
		case "no_action":
			// Leave as-is
		}
	}

	// Update dispute status
	now := time.Now()
	if err := m.store.ResolveDispute(ctx, disputeID, decision.Action, now); err != nil {
		return fmt.Errorf("failed to resolve dispute: %w", err)
	}

	// Notify parties
	if m.notifier != nil {
		m.notifier.Send(Notification{
			Type:    "dispute_resolved",
			Message: fmt.Sprintf("Dispute %s resolved: %s. %s", disputeID, decision.Action, decision.Explanation),
		})
	}

	log.Printf("[dispute] resolved dispute %s with action=%s", disputeID, decision.Action)
	return nil
}

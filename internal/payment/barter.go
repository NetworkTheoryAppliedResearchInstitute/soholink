package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// BarterProcessor implements PaymentProcessor using a local credit/debit ledger
// for cooperative federations where no real money changes hands.
type BarterProcessor struct {
	store          *store.Store
	federationOnly bool
}

// NewBarterProcessor creates a new barter/mutual aid payment processor.
func NewBarterProcessor(s *store.Store, federationOnly bool) *BarterProcessor {
	return &BarterProcessor{
		store:          s,
		federationOnly: federationOnly,
	}
}

func (p *BarterProcessor) Name() string {
	return "barter"
}

func (p *BarterProcessor) IsOnline(ctx context.Context) bool {
	return true // Always available - local ledger
}

func (p *BarterProcessor) CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	// Record credit/debit in local ledger.
	// No actual money movement. Periodic reconciliation via governance.
	now := time.Now()
	chargeID := fmt.Sprintf("barter_%d", now.UnixNano())

	// Persist the barter charge in the pending_payments table for tracking.
	if p.store != nil {
		row := &store.PendingPaymentRow{
			ID:           chargeID,
			ChargeID:     chargeID,
			UserDID:      req.UserDID,
			ProviderDID:  req.ProviderDID,
			Amount:       req.Amount,
			Currency:     "BARTER",
			ResourceType: req.ResourceType,
			Status:       "settled",
			Attempts:     0,
			NextRetry:    now,
			CreatedAt:    now,
		}
		// Best effort: if store write fails, we still return success
		// since barter is a local ledger operation.
		_ = p.store.CreatePendingPayment(ctx, row)
	}

	return &ChargeResult{
		ChargeID:     chargeID,
		Status:       "succeeded",
		Amount:       req.Amount,
		ProcessorFee: 0,
		NetAmount:    req.Amount,
		SettledAt:    &now,
	}, nil
}

func (p *BarterProcessor) ConfirmCharge(ctx context.Context, chargeID string) error {
	// Barter charges are instant - no confirmation needed.
	return nil
}

func (p *BarterProcessor) RefundCharge(ctx context.Context, chargeID string, reason string) error {
	// Record reverse credit in ledger.
	// If we have a store, also persist the refund.
	if p.store != nil {
		payment, err := p.store.GetPaymentByChargeID(ctx, chargeID)
		if err != nil {
			return fmt.Errorf("barter: failed to look up charge for refund: %w", err)
		}
		if payment != nil {
			now := time.Now()
			refundID := fmt.Sprintf("barter_refund_%d", now.UnixNano())
			refundRow := &store.PendingPaymentRow{
				ID:           refundID,
				ChargeID:     refundID,
				UserDID:      payment.ProviderDID,
				ProviderDID:  payment.UserDID,
				Amount:       -payment.Amount,
				Currency:     "BARTER",
				ResourceType: payment.ResourceType,
				Status:       "settled",
				Attempts:     0,
				NextRetry:    now,
				CreatedAt:    now,
			}
			_ = p.store.CreatePendingPayment(ctx, refundRow)

			// Mark original as refunded.
			_ = p.store.UpdatePaymentStatus(ctx, payment.ID, "refunded")
		}
	}
	return nil
}

func (p *BarterProcessor) GetChargeStatus(ctx context.Context, chargeID string) (*ChargeStatus, error) {
	if p.store == nil {
		return nil, fmt.Errorf("barter: store not configured for charge lookup")
	}

	payment, err := p.store.GetPaymentByChargeID(ctx, chargeID)
	if err != nil {
		return nil, fmt.Errorf("barter: failed to query charge status: %w", err)
	}
	if payment == nil {
		return nil, fmt.Errorf("barter: charge %s not found", chargeID)
	}

	cs := &ChargeStatus{
		ChargeID:  payment.ChargeID,
		Status:    payment.Status,
		Amount:    payment.Amount,
		CreatedAt: payment.CreatedAt,
	}

	if payment.Status == "settled" {
		t := payment.CreatedAt // Barter settles instantly at creation time
		cs.SettledAt = &t
	}

	return cs, nil
}

func (p *BarterProcessor) ListCharges(ctx context.Context, filter ChargeFilter) ([]ChargeStatus, error) {
	if p.store == nil {
		return nil, fmt.Errorf("barter: store not configured for charge listing")
	}

	payments, err := p.store.ListPaymentsFiltered(ctx,
		filter.UserDID, filter.ProviderDID, filter.Status,
		filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("barter: failed to list charges: %w", err)
	}

	var charges []ChargeStatus
	for _, payment := range payments {
		cs := ChargeStatus{
			ChargeID:  payment.ChargeID,
			Status:    payment.Status,
			Amount:    payment.Amount,
			CreatedAt: payment.CreatedAt,
		}
		if payment.Status == "settled" {
			t := payment.CreatedAt
			cs.SettledAt = &t
		}
		charges = append(charges, cs)
	}

	return charges, nil
}

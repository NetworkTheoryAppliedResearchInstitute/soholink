package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// FederationTokenProcessor implements PaymentProcessor using on-chain federation tokens.
type FederationTokenProcessor struct {
	store         *store.Store
	tokenContract string
	walletPath    string
	online        bool
}

// NewFederationTokenProcessor creates a new federation token payment processor.
func NewFederationTokenProcessor(s *store.Store, tokenContract, walletPath string) *FederationTokenProcessor {
	return &FederationTokenProcessor{
		store:         s,
		tokenContract: tokenContract,
		walletPath:    walletPath,
		online:        tokenContract != "",
	}
}

func (p *FederationTokenProcessor) Name() string {
	return "federation_token"
}

func (p *FederationTokenProcessor) IsOnline(ctx context.Context) bool {
	return p.online && p.tokenContract != ""
}

func (p *FederationTokenProcessor) CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	if p.tokenContract == "" {
		return nil, fmt.Errorf("federation_token: contract address not configured")
	}

	// Create charge record and persist it in the pending_payments table.
	now := time.Now()
	chargeID := fmt.Sprintf("fed_%d", now.UnixNano())

	row := &store.PendingPaymentRow{
		ID:           chargeID,
		ChargeID:     chargeID,
		UserDID:      req.UserDID,
		ProviderDID:  req.ProviderDID,
		Amount:       req.Amount,
		Currency:     req.Currency,
		ResourceType: req.ResourceType,
		Status:       "pending",
		Attempts:     0,
		NextRetry:    now.Add(1 * time.Hour),
		CreatedAt:    now,
	}
	if row.Currency == "" {
		row.Currency = "FED"
	}

	if err := p.store.CreatePendingPayment(ctx, row); err != nil {
		return nil, fmt.Errorf("federation_token: failed to persist charge: %w", err)
	}

	return &ChargeResult{
		ChargeID:     chargeID,
		Status:       "pending",
		Amount:       req.Amount,
		ProcessorFee: 0, // No intermediary fees
		NetAmount:    req.Amount,
	}, nil
}

func (p *FederationTokenProcessor) ConfirmCharge(ctx context.Context, chargeID string) error {
	if p.store == nil {
		return fmt.Errorf("federation_token: store not configured")
	}

	// Look up the payment by charge_id to get its primary key.
	payment, err := p.store.GetPaymentByChargeID(ctx, chargeID)
	if err != nil {
		return fmt.Errorf("federation_token: failed to look up charge: %w", err)
	}
	if payment == nil {
		return fmt.Errorf("federation_token: charge %s not found", chargeID)
	}

	// Mark the payment as confirmed/settled.
	if err := p.store.UpdatePaymentStatus(ctx, payment.ID, "settled"); err != nil {
		return fmt.Errorf("federation_token: failed to confirm charge: %w", err)
	}

	return nil
}

func (p *FederationTokenProcessor) RefundCharge(ctx context.Context, chargeID string, reason string) error {
	if p.store == nil {
		return fmt.Errorf("federation_token: store not configured")
	}

	// Look up the original charge to get amount and parties.
	payment, err := p.store.GetPaymentByChargeID(ctx, chargeID)
	if err != nil {
		return fmt.Errorf("federation_token: failed to look up charge for refund: %w", err)
	}
	if payment == nil {
		return fmt.Errorf("federation_token: charge %s not found for refund", chargeID)
	}

	// Create a reverse charge with negative amount to represent the refund.
	now := time.Now()
	refundID := fmt.Sprintf("fed_refund_%d", now.UnixNano())

	refundRow := &store.PendingPaymentRow{
		ID:           refundID,
		ChargeID:     refundID,
		UserDID:      payment.ProviderDID, // Reverse: provider pays back user
		ProviderDID:  payment.UserDID,
		Amount:       -payment.Amount, // Negative amount for refund
		Currency:     payment.Currency,
		ResourceType: payment.ResourceType,
		Status:       "settled",
		Attempts:     0,
		NextRetry:    now,
		CreatedAt:    now,
	}

	if err := p.store.CreatePendingPayment(ctx, refundRow); err != nil {
		return fmt.Errorf("federation_token: failed to create refund record: %w", err)
	}

	// Mark the original charge as refunded.
	if err := p.store.UpdatePaymentStatus(ctx, payment.ID, "refunded"); err != nil {
		return fmt.Errorf("federation_token: failed to update original charge status: %w", err)
	}

	return nil
}

func (p *FederationTokenProcessor) GetChargeStatus(ctx context.Context, chargeID string) (*ChargeStatus, error) {
	if p.store == nil {
		return nil, fmt.Errorf("federation_token: store not configured")
	}

	payment, err := p.store.GetPaymentByChargeID(ctx, chargeID)
	if err != nil {
		return nil, fmt.Errorf("federation_token: failed to query charge status: %w", err)
	}
	if payment == nil {
		return nil, fmt.Errorf("federation_token: charge %s not found", chargeID)
	}

	cs := &ChargeStatus{
		ChargeID:  payment.ChargeID,
		Status:    payment.Status,
		Amount:    payment.Amount,
		CreatedAt: payment.CreatedAt,
	}

	// If settled, use the created_at as a proxy for settle time
	// (in a real implementation, you would have a separate settled_at column).
	if payment.Status == "settled" {
		t := time.Now()
		cs.SettledAt = &t
	}

	return cs, nil
}

func (p *FederationTokenProcessor) ListCharges(ctx context.Context, filter ChargeFilter) ([]ChargeStatus, error) {
	if p.store == nil {
		return nil, fmt.Errorf("federation_token: store not configured")
	}

	payments, err := p.store.ListPaymentsFiltered(ctx,
		filter.UserDID, filter.ProviderDID, filter.Status,
		filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("federation_token: failed to list charges: %w", err)
	}

	var charges []ChargeStatus
	for _, payment := range payments {
		cs := ChargeStatus{
			ChargeID:  payment.ChargeID,
			Status:    payment.Status,
			Amount:    payment.Amount,
			CreatedAt: payment.CreatedAt,
		}
		charges = append(charges, cs)
	}

	return charges, nil
}

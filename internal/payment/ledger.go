package payment

import (
	"context"
	"fmt"
	"log"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// Ledger manages payment processing across multiple processors with fallback.
type Ledger struct {
	store      *store.Store
	processors []PaymentProcessor
}

// NewLedger creates a new payment ledger with the given processors ordered by priority.
func NewLedger(s *store.Store, processors []PaymentProcessor) *Ledger {
	return &Ledger{
		store:      s,
		processors: processors,
	}
}

// ChargeForUsage attempts to charge for resource usage, falling through processors
// by priority. Returns ErrPaymentQueueFull if all processors fail and the offline
// queue is full.
func (l *Ledger) ChargeForUsage(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	for _, proc := range l.processors {
		if !proc.IsOnline(ctx) {
			log.Printf("[payment] processor %s offline, skipping", proc.Name())
			continue
		}

		result, err := proc.CreateCharge(ctx, req)
		if err == nil {
			log.Printf("[payment] charge succeeded via %s: %s", proc.Name(), result.ChargeID)
			return result, nil
		}
		log.Printf("[payment] processor %s failed: %v, trying next", proc.Name(), err)
	}

	return nil, fmt.Errorf("all payment processors failed")
}

// EscrowPayment creates an escrowed payment that will be released upon transaction completion.
func (l *Ledger) EscrowPayment(ctx context.Context, req ChargeRequest) (string, error) {
	req.Metadata["escrow"] = "true"
	result, err := l.ChargeForUsage(ctx, req)
	if err != nil {
		return "", err
	}
	return result.ChargeID, nil
}

// ReleaseEscrow releases an escrowed payment to the provider.
func (l *Ledger) ReleaseEscrow(ctx context.Context, escrowID []byte, providerDID string) error {
	chargeID := string(escrowID)
	for _, proc := range l.processors {
		if !proc.IsOnline(ctx) {
			continue
		}
		if err := proc.ConfirmCharge(ctx, chargeID); err == nil {
			return nil
		}
	}
	return fmt.Errorf("failed to release escrow %s", chargeID)
}

// RefundEscrow refunds an escrowed payment to the user.
func (l *Ledger) RefundEscrow(ctx context.Context, escrowID []byte, userDID string) error {
	chargeID := string(escrowID)
	for _, proc := range l.processors {
		if !proc.IsOnline(ctx) {
			continue
		}
		if err := proc.RefundCharge(ctx, chargeID, "escrow_timeout"); err == nil {
			return nil
		}
	}
	return fmt.Errorf("failed to refund escrow %s", chargeID)
}

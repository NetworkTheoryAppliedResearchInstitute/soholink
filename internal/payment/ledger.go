package payment

import (
	"context"
	"fmt"
	"log"
	"time"

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

// PayoutRequest specifies a withdrawal of earned revenue by a provider node.
type PayoutRequest struct {
	ProviderDID   string // node DID requesting the payout
	AmountSats    int64  // amount in satoshis (1 BTC = 100 000 000 sats)
	Processor     string // preferred processor name; "" = any available
	PayoutAddress string // Lightning invoice, on-chain address, or bank ref
}

// PayoutResult reports the outcome of a payout request.
type PayoutResult struct {
	PayoutID   string
	Status     string // "pending" (queued), "processing" (submitted), "failed"
	Processor  string // which processor accepted the request
	ExternalID string // processor-side payment or transfer ID
}

// ---------------------------------------------------------------------------
// Wallet methods (buyer-side)
// ---------------------------------------------------------------------------

// GetWalletBalance returns the current sats balance for the given requester DID.
func (l *Ledger) GetWalletBalance(ctx context.Context, did string) (int64, error) {
	return l.store.GetWalletBalance(ctx, did)
}

// TopupWallet creates a payment request (Lightning invoice or Stripe payment intent)
// for the given amount and records a pending topup row.
// Returns (topupID, invoice/intentID, error).
// The wallet is NOT credited until ConfirmTopup is called.
//
// idempotencyKey is an optional client-provided key for deduplication.  When
// non-empty, if a topup with that key already exists it is returned unchanged
// rather than creating a new one (safe to retry after network failures).
func (l *Ledger) TopupWallet(ctx context.Context, did, processor string, amountSats int64, idempotencyKey string) (string, string, error) {
	// Idempotency check: return existing topup when the client re-submits.
	if idempotencyKey != "" {
		existing, err := l.store.GetWalletTopupByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			return "", "", fmt.Errorf("wallet topup: idempotency check: %w", err)
		}
		if existing != nil {
			log.Printf("[payment] wallet topup deduplicated via idempotency key (topup=%s)", existing.TopupID)
			return existing.TopupID, existing.Invoice, nil
		}
	}

	topupID := fmt.Sprintf("tu_%d", time.Now().UnixNano())
	invoice := ""

	// Try to create the invoice/intent via the requested processor.
	for _, proc := range l.processors {
		if proc.Name() != processor {
			continue
		}
		if !proc.IsOnline(ctx) {
			log.Printf("[payment] wallet topup: processor %s offline", processor)
			break
		}
		result, err := proc.CreateCharge(ctx, ChargeRequest{
			Amount:         amountSats,
			Currency:       "sats",
			UserDID:        did,
			ResourceType:   "wallet_topup",
			IdempotencyKey: topupID,
			Metadata:       map[string]string{"topup_id": topupID, "direction": "inbound"},
		})
		if err != nil {
			log.Printf("[payment] wallet topup charge creation failed: %v", err)
			break
		}
		invoice = result.ChargeID
		break
	}

	row := &store.WalletTopupRow{
		TopupID:        topupID,
		DID:            did,
		AmountSats:     amountSats,
		Processor:      processor,
		Invoice:        invoice,
		Status:         "awaiting_payment",
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now().UTC(),
	}
	if err := l.store.CreateWalletTopup(ctx, row); err != nil {
		return "", "", fmt.Errorf("wallet topup: record: %w", err)
	}
	log.Printf("[payment] wallet topup %s created for %s (%d sats via %s)", topupID, did, amountSats, processor)
	return topupID, invoice, nil
}

// ConfirmTopup marks a topup as confirmed and credits the requester's wallet.
// In production this is called by a webhook handler; in development/test it
// can be called manually via POST /api/wallet/confirm-topup.
func (l *Ledger) ConfirmTopup(ctx context.Context, topupID string) error {
	row, err := l.store.GetWalletTopup(ctx, topupID)
	if err != nil {
		return fmt.Errorf("confirm topup: lookup: %w", err)
	}
	if row.Status != "awaiting_payment" {
		return fmt.Errorf("confirm topup: topup %s is already %s", topupID, row.Status)
	}
	now := time.Now().UTC()
	if err := l.store.UpdateTopupStatus(ctx, topupID, "confirmed", &now); err != nil {
		return fmt.Errorf("confirm topup: update status: %w", err)
	}
	if err := l.store.CreditWallet(ctx, row.DID, row.AmountSats); err != nil {
		return fmt.Errorf("confirm topup: credit wallet: %w", err)
	}
	log.Printf("[payment] topup %s confirmed — credited %d sats to %s", topupID, row.AmountSats, row.DID)
	return nil
}

// FailTopup marks a topup as failed.  This is called by the Stripe webhook
// handler when a payment_intent.payment_failed event is received.
// The wallet is NOT credited and the topup record is updated to "failed".
func (l *Ledger) FailTopup(ctx context.Context, topupID string) error {
	row, err := l.store.GetWalletTopup(ctx, topupID)
	if err != nil {
		return fmt.Errorf("fail topup: lookup: %w", err)
	}
	if row.Status != "awaiting_payment" {
		// Already confirmed or failed — idempotent no-op.
		log.Printf("[payment] FailTopup: topup %s is already %s; skipping", topupID, row.Status)
		return nil
	}
	now := time.Now().UTC()
	if err := l.store.UpdateTopupStatus(ctx, topupID, "failed", &now); err != nil {
		return fmt.Errorf("fail topup: update status: %w", err)
	}
	log.Printf("[payment] topup %s marked failed (payment declined)", topupID)
	return nil
}

// DebitWallet atomically subtracts sats from a requester's wallet balance.
// Returns store.ErrInsufficientBalance when balance is too low.
func (l *Ledger) DebitWallet(ctx context.Context, did string, sats int64) error {
	return l.store.DebitWallet(ctx, did, sats)
}

// ---------------------------------------------------------------------------
// Provider payout methods
// ---------------------------------------------------------------------------

// RequestPayout records a provider payout request and attempts to dispatch it
// via an available payment processor.  The payout record is stored in the
// payouts table regardless of whether the processor call succeeds, so that
// operators can inspect and retry failed payouts.
func (l *Ledger) RequestPayout(ctx context.Context, req PayoutRequest) (*PayoutResult, error) {
	payoutID := fmt.Sprintf("po_%d", time.Now().UnixNano())

	row := &store.PayoutRow{
		PayoutID:    payoutID,
		ProviderDID: req.ProviderDID,
		AmountSats:  req.AmountSats,
		Processor:   req.Processor,
		Status:      "pending",
		RequestedAt: time.Now().UTC(),
	}
	if err := l.store.CreatePayout(ctx, row); err != nil {
		return nil, fmt.Errorf("payout: failed to record request: %w", err)
	}

	// Try to dispatch via the preferred (or any online) processor.
	for _, proc := range l.processors {
		if req.Processor != "" && proc.Name() != req.Processor {
			continue
		}
		if !proc.IsOnline(ctx) {
			continue
		}
		// CreateCharge is the generic "move money" method; for payouts we
		// annotate metadata so the processor knows the direction.
		result, err := proc.CreateCharge(ctx, ChargeRequest{
			UserDID:      req.ProviderDID,
			ProviderDID:  req.ProviderDID,
			Amount:       req.AmountSats,
			ResourceType: "provider_payout",
			Metadata: map[string]string{
				"payout_id":      payoutID,
				"payout_address": req.PayoutAddress,
				"direction":      "outbound",
			},
		})
		if err == nil {
			_ = l.store.UpdatePayoutStatus(ctx, payoutID, "processing", result.ChargeID, "")
			log.Printf("[payment] payout %s dispatched via %s (external=%s)", payoutID, proc.Name(), result.ChargeID)
			return &PayoutResult{
				PayoutID:   payoutID,
				Status:     "processing",
				Processor:  proc.Name(),
				ExternalID: result.ChargeID,
			}, nil
		}
		log.Printf("[payment] payout processor %s failed: %v", proc.Name(), err)
	}

	// All processors failed — leave payout as "pending" for operator retry.
	log.Printf("[payment] payout %s queued for manual retry (no processor available)", payoutID)
	return &PayoutResult{PayoutID: payoutID, Status: "pending"}, nil
}

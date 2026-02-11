package payment

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// OfflineSettler periodically attempts to settle queued offline payments.
type OfflineSettler struct {
	store      *store.Store
	processors []PaymentProcessor
	interval   time.Duration
	maxQueue   int
}

// NewOfflineSettler creates a new offline payment settler.
func NewOfflineSettler(s *store.Store, processors []PaymentProcessor, interval time.Duration, maxQueue int) *OfflineSettler {
	return &OfflineSettler{
		store:      s,
		processors: processors,
		interval:   interval,
		maxQueue:   maxQueue,
	}
}

// Run starts the settlement loop. It blocks until ctx is cancelled.
func (s *OfflineSettler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processPending(ctx)
		}
	}
}

func (s *OfflineSettler) processPending(ctx context.Context) {
	pending, err := s.store.GetPendingPayments(ctx, 100)
	if err != nil {
		log.Printf("[payment] failed to query pending payments: %v", err)
		return
	}

	for _, p := range pending {
		for _, proc := range s.processors {
			if !proc.IsOnline(ctx) {
				continue
			}

			if err := proc.ConfirmCharge(ctx, p.ChargeID); err == nil {
				s.store.UpdatePaymentStatus(ctx, p.ID, "settled")
				log.Printf("[payment] settled offline payment %s via %s", p.ID, proc.Name())
				break
			}
		}
	}
}

// QueuePayment adds a payment to the offline settlement queue.
func (s *OfflineSettler) QueuePayment(ctx context.Context, evt PaymentEvent) error {
	count, err := s.store.CountPendingPayments(ctx)
	if err != nil {
		return err
	}
	if count >= s.maxQueue {
		return fmt.Errorf("offline payment queue full (%d/%d)", count, s.maxQueue)
	}

	evt.Attempts++
	evt.NextRetry = time.Now().Add(backoff(evt.Attempts))

	row := &store.PendingPaymentRow{
		ID:           evt.ID,
		ChargeID:     evt.ChargeID,
		UserDID:      evt.UserDID,
		ProviderDID:  evt.ProviderDID,
		Amount:       evt.Amount,
		Currency:     evt.Currency,
		ResourceType: evt.ResourceType,
		Status:       "pending",
		Attempts:     evt.Attempts,
		NextRetry:    evt.NextRetry,
		CreatedAt:    evt.CreatedAt,
	}

	return s.store.CreatePendingPayment(ctx, row)
}

// backoff returns an exponential backoff duration capped at 1 hour.
func backoff(attempts int) time.Duration {
	d := time.Duration(math.Pow(2, float64(attempts))) * time.Second
	if d > time.Hour {
		d = time.Hour
	}
	return d
}

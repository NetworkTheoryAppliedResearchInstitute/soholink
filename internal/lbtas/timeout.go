package lbtas

import (
	"context"
	"log"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/accounting"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// TimeoutResolver periodically checks for timed-out transactions and auto-resolves them.
type TimeoutResolver struct {
	store         *store.Store
	accounting    *accounting.Collector
	checkInterval time.Duration
}

// NewTimeoutResolver creates a new resolver that checks for timed-out ratings.
func NewTimeoutResolver(s *store.Store, ac *accounting.Collector, interval time.Duration) *TimeoutResolver {
	return &TimeoutResolver{
		store:         s,
		accounting:    ac,
		checkInterval: interval,
	}
}

// Run starts the timeout resolver loop. It blocks until ctx is cancelled.
func (r *TimeoutResolver) Run(ctx context.Context) {
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.resolveTimedOut(ctx)
		}
	}
}

func (r *TimeoutResolver) resolveTimedOut(ctx context.Context) {
	rows, err := r.store.GetTimedOutTransactions(ctx)
	if err != nil {
		log.Printf("[lbtas] failed to query timed-out transactions: %v", err)
		return
	}

	for i := range rows {
		tx := TransactionFromRow(&rows[i])
		switch tx.State {
		case StateAwaitingProviderRating:
			r.penalizeProvider(ctx, tx)
		case StateAwaitingUserRating:
			r.autoResolveUserRating(ctx, tx)
		}
	}
}

// autoResolveUserRating handles the case where user downloaded results but never rated.
// Auto-rates as "Acceptable" (3/5), releases payment, penalizes user score.
func (r *TimeoutResolver) autoResolveUserRating(ctx context.Context, tx *ResourceTransaction) {
	autoRating := LBTASRating{
		Score:     3,
		Category:  "auto_resolved",
		Feedback:  "Auto-rated: user did not rate within deadline",
		Timestamp: time.Now(),
	}

	tx.UserRating = &autoRating
	tx.State = StateTimedOut
	tx.UpdatedAt = time.Now()

	if err := r.store.UpdateTransaction(ctx, TransactionToRow(tx)); err != nil {
		log.Printf("[lbtas] failed to auto-resolve transaction %s: %v", tx.TransactionID, err)
		return
	}

	// Update provider score (small boost - they completed the work)
	score := r.getOrDefaultScore(ctx, tx.ProviderDID)
	UpdateScoreFromRating(score, autoRating)
	r.store.UpsertLBTASScore(ctx, ScoreToRow(score))

	// Penalize user for not rating
	userScore := r.getOrDefaultScore(ctx, tx.UserDID)
	ApplyPenalty(userScore, 2)
	r.store.UpsertLBTASScore(ctx, ScoreToRow(userScore))

	r.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "rating_auto_resolved",
		UserDID:   tx.UserDID,
		SessionID: tx.TransactionID,
	})

	log.Printf("[lbtas] auto-resolved transaction %s (user timeout)", tx.TransactionID)
}

// penalizeProvider handles the case where provider completed job but refused to rate user.
// This is bad faith - refund payment and harshly penalize provider.
func (r *TimeoutResolver) penalizeProvider(ctx context.Context, tx *ResourceTransaction) {
	tx.State = StateTimedOut
	tx.UpdatedAt = time.Now()

	if err := r.store.UpdateTransaction(ctx, TransactionToRow(tx)); err != nil {
		log.Printf("[lbtas] failed to penalize provider for transaction %s: %v", tx.TransactionID, err)
		return
	}

	// Harsh penalty to provider's score
	providerScore := r.getOrDefaultScore(ctx, tx.ProviderDID)
	ApplyPenalty(providerScore, 10)
	r.store.UpsertLBTASScore(ctx, ScoreToRow(providerScore))

	r.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "provider_timeout_penalty",
		UserDID:   tx.ProviderDID,
		SessionID: tx.TransactionID,
	})

	log.Printf("[lbtas] penalized provider for transaction %s (provider timeout)", tx.TransactionID)
}

// getOrDefaultScore loads a score from the store or returns a default.
func (r *TimeoutResolver) getOrDefaultScore(ctx context.Context, did string) *LBTASScore {
	row, err := r.store.GetLBTASScore(ctx, did)
	if err != nil || row == nil {
		return &LBTASScore{DID: did, OverallScore: 50}
	}
	return ScoreFromRow(row)
}

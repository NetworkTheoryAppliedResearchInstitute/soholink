package lbtas

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/accounting"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// Manager coordinates LBTAS rating flows, score aggregation, and blockchain anchoring.
type Manager struct {
	store      *store.Store
	accounting *accounting.Collector
}

// NewManager creates a new LBTAS manager.
// The accounting.Collector argument is optional; pass nil or omit it for tests.
func NewManager(s *store.Store, ac ...*accounting.Collector) *Manager {
	m := &Manager{store: s}
	if len(ac) > 0 {
		m.accounting = ac[0]
	}
	return m
}

// ProviderRatesUser handles the provider rating the user after job completion.
// This transitions the transaction to results_escrowed and releases results.
func (m *Manager) ProviderRatesUser(ctx context.Context, req ProviderRatingRequest) error {
	row, err := m.store.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to load transaction: %w", err)
	}
	if row == nil {
		return fmt.Errorf("transaction not found: %s", req.TransactionID)
	}

	tx := TransactionFromRow(row)

	if tx.ProviderDID != req.ProviderDID {
		return ErrUnauthorized
	}

	// Prevent self-review: provider and user must be different identities.
	if req.ProviderDID == tx.UserDID {
		return fmt.Errorf("lbtas: cannot rate yourself")
	}

	if tx.State != StateAwaitingProviderRating {
		return ErrInvalidState
	}

	if err := ValidateRating(req.Rating); err != nil {
		return err
	}

	tx.ProviderRating = &req.Rating
	tx.State = StateResultsEscrowed
	tx.UpdatedAt = time.Now()

	if err := m.store.UpdateTransaction(ctx, TransactionToRow(tx)); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Update user's LBTAS score
	score := m.getOrDefaultScore(ctx, tx.UserDID)
	UpdateScoreFromRating(score, req.Rating)
	if err := m.store.UpsertLBTASScore(ctx, ScoreToRow(score)); err != nil {
		log.Printf("[lbtas] failed to update user score: %v", err)
	}

	m.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "provider_rated_user",
		UserDID:   tx.UserDID,
		SessionID: req.TransactionID,
		Decision:  fmt.Sprintf("score:%d", req.Rating.Score),
	})

	return nil
}

// UserRatesProvider handles the user rating the provider after receiving results.
// This completes the transaction and releases payment.
func (m *Manager) UserRatesProvider(ctx context.Context, req UserRatingRequest) error {
	row, err := m.store.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to load transaction: %w", err)
	}
	if row == nil {
		return fmt.Errorf("transaction not found: %s", req.TransactionID)
	}

	tx := TransactionFromRow(row)

	if tx.UserDID != req.UserDID {
		return ErrUnauthorized
	}

	// Prevent self-review: user and provider must be different identities.
	if req.UserDID == tx.ProviderDID {
		return fmt.Errorf("lbtas: cannot rate yourself")
	}

	if tx.State != StateAwaitingUserRating {
		return ErrInvalidState
	}

	if err := ValidateRating(req.Rating); err != nil {
		return err
	}

	tx.UserRating = &req.Rating
	tx.State = StateCompleted
	tx.UpdatedAt = time.Now()

	if err := m.store.UpdateTransaction(ctx, TransactionToRow(tx)); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Update provider's LBTAS score
	score := m.getOrDefaultScore(ctx, tx.ProviderDID)
	UpdateScoreFromRating(score, req.Rating)
	if err := m.store.UpsertLBTASScore(ctx, ScoreToRow(score)); err != nil {
		log.Printf("[lbtas] failed to update provider score: %v", err)
	}

	m.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "user_rated_provider",
		UserDID:   tx.ProviderDID,
		SessionID: req.TransactionID,
		Decision:  fmt.Sprintf("score:%d", req.Rating.Score),
	})

	return nil
}

// GetScore returns the LBTAS score for a DID, or a default score if not found.
func (m *Manager) GetScore(ctx context.Context, did string) (*LBTASScore, error) {
	row, err := m.store.GetLBTASScore(ctx, did)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &LBTASScore{
			DID:          did,
			OverallScore: 50,
		}, nil
	}
	return ScoreFromRow(row), nil
}

// FileDispute creates a dispute for a transaction with highly divergent ratings.
func (m *Manager) FileDispute(ctx context.Context, req DisputeRequest) error {
	row, err := m.store.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to load transaction: %w", err)
	}
	if row == nil {
		return fmt.Errorf("transaction not found: %s", req.TransactionID)
	}

	tx := TransactionFromRow(row)

	if !CanFileDispute(tx) {
		return ErrDisputeNotJustified
	}

	tx.State = StateDisputed
	tx.DisputeReason = req.Reason
	tx.UpdatedAt = time.Now()

	if err := m.store.UpdateTransaction(ctx, TransactionToRow(tx)); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	m.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "dispute_filed",
		UserDID:   req.FilerDID,
		SessionID: req.TransactionID,
		Reason:    req.Reason,
	})

	return nil
}

// getOrDefaultScore loads a score from the store or returns a default.
func (m *Manager) getOrDefaultScore(ctx context.Context, did string) *LBTASScore {
	row, err := m.store.GetLBTASScore(ctx, did)
	if err != nil || row == nil {
		return &LBTASScore{
			DID:          did,
			OverallScore: 50,
		}
	}
	return ScoreFromRow(row)
}

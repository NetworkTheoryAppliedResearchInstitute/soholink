package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ResourceTransactionRow is the store-layer representation of a resource transaction.
// This avoids circular imports between store and lbtas packages.
type ResourceTransactionRow struct {
	TransactionID   string
	UserDID         string
	ProviderDID     string
	ResourceType    string
	ResourceID      string
	State           string
	PaymentAmount   int64
	PaymentCurrency string
	PaymentEscrowed bool
	PaymentProof    []byte
	ResultsReady    bool
	ResultsHash     []byte
	ResultsPath     string
	ResultsKey      []byte
	RatingDeadline  time.Time
	DisputeID       *string
	DisputeReason   string
	BlockchainBlock *int64
	BlockchainHash  []byte
	UserSignature   []byte
	ProviderSignature []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// LBTASScoreRow is the store-layer representation of an LBTAS score.
type LBTASScoreRow struct {
	DID                   string
	OverallScore          int
	PaymentReliability    float64
	ExecutionQuality      float64
	Communication         float64
	ResourceUsage         float64
	TotalTransactions     int
	CompletedTransactions int
	DisputedTransactions  int
	ScoreHistoryJSON      string
	LastAnchorBlock       *int64
	LastAnchorHash        []byte
	UpdatedAt             time.Time
}

// PendingPaymentRow is the store-layer representation of a pending payment.
type PendingPaymentRow struct {
	ID           string
	ChargeID     string
	UserDID      string
	ProviderDID  string
	Amount       int64
	Currency     string
	ResourceType string
	Status       string
	Attempts     int
	NextRetry    time.Time
	CreatedAt    time.Time
}

// CreateTransaction inserts a new resource transaction record.
func (s *Store) CreateTransaction(ctx context.Context, tx *ResourceTransactionRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO resource_transactions
		(transaction_id, user_did, provider_did, resource_type, resource_id, state,
		 payment_amount, payment_currency, payment_escrowed, payment_proof,
		 rating_deadline, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx.TransactionID, tx.UserDID, tx.ProviderDID, tx.ResourceType, tx.ResourceID,
		tx.State, tx.PaymentAmount, tx.PaymentCurrency, boolToInt(tx.PaymentEscrowed),
		tx.PaymentProof, tx.RatingDeadline, tx.CreatedAt, tx.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}

// GetTransaction retrieves a resource transaction by ID.
func (s *Store) GetTransaction(ctx context.Context, transactionID string) (*ResourceTransactionRow, error) {
	var tx ResourceTransactionRow
	var paymentEscrowed int
	var resultsReady int
	var disputeID sql.NullString
	var ratingDeadline, createdAt, updatedAt string
	var blockchainBlock sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT transaction_id, user_did, provider_did, resource_type, resource_id,
		        state, payment_amount, payment_currency, payment_escrowed, payment_proof,
		        results_ready, results_hash, results_path, results_key,
		        rating_deadline, dispute_id, dispute_reason,
		        blockchain_block, blockchain_hash,
		        user_signature, provider_signature, created_at, updated_at
		 FROM resource_transactions WHERE transaction_id = ?`, transactionID).Scan(
		&tx.TransactionID, &tx.UserDID, &tx.ProviderDID, &tx.ResourceType, &tx.ResourceID,
		&tx.State, &tx.PaymentAmount, &tx.PaymentCurrency, &paymentEscrowed, &tx.PaymentProof,
		&resultsReady, &tx.ResultsHash, &tx.ResultsPath, &tx.ResultsKey,
		&ratingDeadline, &disputeID, &tx.DisputeReason,
		&blockchainBlock, &tx.BlockchainHash,
		&tx.UserSignature, &tx.ProviderSignature, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	tx.PaymentEscrowed = paymentEscrowed != 0
	tx.ResultsReady = resultsReady != 0

	if disputeID.Valid {
		tx.DisputeID = &disputeID.String
	}

	if blockchainBlock.Valid {
		v := blockchainBlock.Int64
		tx.BlockchainBlock = &v
	}

	tx.RatingDeadline, _ = time.Parse("2006-01-02 15:04:05", ratingDeadline)
	tx.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	tx.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &tx, nil
}

// UpdateTransaction updates a resource transaction record.
func (s *Store) UpdateTransaction(ctx context.Context, tx *ResourceTransactionRow) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE resource_transactions SET
		 state = ?, results_ready = ?, results_hash = ?, results_path = ?, results_key = ?,
		 dispute_id = ?, dispute_reason = ?,
		 blockchain_block = ?, blockchain_hash = ?,
		 user_signature = ?, provider_signature = ?, updated_at = ?
		 WHERE transaction_id = ?`,
		tx.State, boolToInt(tx.ResultsReady), tx.ResultsHash, tx.ResultsPath, tx.ResultsKey,
		nullStringPtr(tx.DisputeID), tx.DisputeReason,
		nullInt64Ptr(tx.BlockchainBlock), tx.BlockchainHash,
		tx.UserSignature, tx.ProviderSignature, time.Now(),
		tx.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	return nil
}

// UpdateTransactionState updates only the state of a transaction.
func (s *Store) UpdateTransactionState(ctx context.Context, transactionID string, state string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE resource_transactions SET state = ?, updated_at = ? WHERE transaction_id = ?",
		state, time.Now(), transactionID)
	return err
}

// GetTimedOutTransactions returns transactions past their rating deadline.
func (s *Store) GetTimedOutTransactions(ctx context.Context) ([]ResourceTransactionRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT transaction_id, user_did, provider_did, resource_type, resource_id, state,
		        payment_amount, payment_currency, payment_proof, rating_deadline, created_at, updated_at
		 FROM resource_transactions
		 WHERE rating_deadline < ? AND state IN ('awaiting_provider_rating', 'awaiting_user_rating')`,
		time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query timed-out transactions: %w", err)
	}
	defer rows.Close()

	var txs []ResourceTransactionRow
	for rows.Next() {
		var tx ResourceTransactionRow
		var ratingDeadline, createdAt, updatedAt string
		if err := rows.Scan(
			&tx.TransactionID, &tx.UserDID, &tx.ProviderDID, &tx.ResourceType, &tx.ResourceID,
			&tx.State, &tx.PaymentAmount, &tx.PaymentCurrency, &tx.PaymentProof,
			&ratingDeadline, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		tx.RatingDeadline, _ = time.Parse("2006-01-02 15:04:05", ratingDeadline)
		tx.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		tx.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		txs = append(txs, tx)
	}
	return txs, rows.Err()
}

// GetLBTASScore retrieves the LBTAS score for a DID.
func (s *Store) GetLBTASScore(ctx context.Context, did string) (*LBTASScoreRow, error) {
	var score LBTASScoreRow
	var historyJSON sql.NullString
	var lastAnchorBlock sql.NullInt64
	var updatedAt string

	err := s.db.QueryRowContext(ctx,
		`SELECT did, overall_score, payment_reliability, execution_quality,
		        communication, resource_usage, total_transactions, completed_transactions,
		        disputed_transactions, score_history, last_anchor_block, last_anchor_hash, updated_at
		 FROM lbtas_scores WHERE did = ?`, did).Scan(
		&score.DID, &score.OverallScore, &score.PaymentReliability, &score.ExecutionQuality,
		&score.Communication, &score.ResourceUsage, &score.TotalTransactions,
		&score.CompletedTransactions, &score.DisputedTransactions,
		&historyJSON, &lastAnchorBlock, &score.LastAnchorHash, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get LBTAS score: %w", err)
	}

	if historyJSON.Valid {
		score.ScoreHistoryJSON = historyJSON.String
	}

	if lastAnchorBlock.Valid {
		v := lastAnchorBlock.Int64
		score.LastAnchorBlock = &v
	}

	score.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &score, nil
}

// UpsertLBTASScore inserts or updates an LBTAS score.
func (s *Store) UpsertLBTASScore(ctx context.Context, score *LBTASScoreRow) error {
	var lastBlock sql.NullInt64
	if score.LastAnchorBlock != nil {
		lastBlock = sql.NullInt64{Int64: *score.LastAnchorBlock, Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO lbtas_scores
		(did, overall_score, payment_reliability, execution_quality, communication,
		 resource_usage, total_transactions, completed_transactions, disputed_transactions,
		 score_history, last_anchor_block, last_anchor_hash, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(did) DO UPDATE SET
		 overall_score = excluded.overall_score,
		 payment_reliability = excluded.payment_reliability,
		 execution_quality = excluded.execution_quality,
		 communication = excluded.communication,
		 resource_usage = excluded.resource_usage,
		 total_transactions = excluded.total_transactions,
		 completed_transactions = excluded.completed_transactions,
		 disputed_transactions = excluded.disputed_transactions,
		 score_history = excluded.score_history,
		 last_anchor_block = excluded.last_anchor_block,
		 last_anchor_hash = excluded.last_anchor_hash,
		 updated_at = excluded.updated_at`,
		score.DID, score.OverallScore, score.PaymentReliability, score.ExecutionQuality,
		score.Communication, score.ResourceUsage, score.TotalTransactions,
		score.CompletedTransactions, score.DisputedTransactions,
		score.ScoreHistoryJSON, lastBlock, score.LastAnchorHash,
		time.Now())
	if err != nil {
		return fmt.Errorf("failed to upsert LBTAS score: %w", err)
	}
	return nil
}

// GetPendingPayments retrieves pending payments for offline settlement.
func (s *Store) GetPendingPayments(ctx context.Context, limit int) ([]PendingPaymentRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, charge_id, user_did, provider_did, amount, currency, resource_type, attempts, next_retry, created_at
		 FROM pending_payments
		 WHERE status = 'pending' AND next_retry <= ?
		 ORDER BY next_retry ASC LIMIT ?`,
		time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending payments: %w", err)
	}
	defer rows.Close()

	var payments []PendingPaymentRow
	for rows.Next() {
		var p PendingPaymentRow
		var nextRetry, createdAt string
		if err := rows.Scan(&p.ID, &p.ChargeID, &p.UserDID, &p.ProviderDID,
			&p.Amount, &p.Currency, &p.ResourceType, &p.Attempts, &nextRetry, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		p.NextRetry = parseSQLiteTime(nextRetry)
		p.CreatedAt = parseSQLiteTime(createdAt)
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

// UpdatePaymentStatus updates the status of a pending payment.
func (s *Store) UpdatePaymentStatus(ctx context.Context, paymentID string, status string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE pending_payments SET status = ? WHERE id = ?", status, paymentID)
	return err
}

// CountPendingPayments returns the number of pending payments.
func (s *Store) CountPendingPayments(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pending_payments WHERE status = 'pending'").Scan(&count)
	return count, err
}

// CreatePendingPayment inserts a new pending payment for offline settlement.
func (s *Store) CreatePendingPayment(ctx context.Context, p *PendingPaymentRow) error {
	createdAt := p.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	status := p.Status
	if status == "" {
		status = "pending"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pending_payments (id, charge_id, user_did, provider_did, amount, currency, resource_type, status, attempts, next_retry, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.ChargeID, p.UserDID, p.ProviderDID, p.Amount, p.Currency, p.ResourceType, status, p.Attempts, p.NextRetry, createdAt.Format("2006-01-02 15:04:05"))
	if err != nil {
		return fmt.Errorf("failed to create pending payment: %w", err)
	}
	return nil
}

// parseSQLiteTime parses a datetime string returned by SQLite, trying multiple
// common formats (SQLite stores datetimes as TEXT without enforcing a format).
func parseSQLiteTime(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Helper functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStringPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullInt64Ptr(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

// GetPaymentByChargeID retrieves a pending payment by its charge_id.
func (s *Store) GetPaymentByChargeID(ctx context.Context, chargeID string) (*PendingPaymentRow, error) {
	var p PendingPaymentRow
	var nextRetry, createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, charge_id, user_did, provider_did, amount, currency, resource_type, status, attempts, next_retry, created_at
		 FROM pending_payments WHERE charge_id = ?`, chargeID).Scan(
		&p.ID, &p.ChargeID, &p.UserDID, &p.ProviderDID, &p.Amount, &p.Currency,
		&p.ResourceType, &p.Status, &p.Attempts, &nextRetry, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get payment by charge_id: %w", err)
	}
	p.NextRetry = parseSQLiteTime(nextRetry)
	p.CreatedAt = parseSQLiteTime(createdAt)
	return &p, nil
}

// ListPaymentsFiltered retrieves payments matching optional filter criteria.
func (s *Store) ListPaymentsFiltered(ctx context.Context, userDID, providerDID, status string, limit, offset int) ([]PendingPaymentRow, error) {
	query := `SELECT id, charge_id, user_did, provider_did, amount, currency, resource_type, status, attempts, next_retry, created_at
		 FROM pending_payments WHERE 1=1`
	var args []interface{}

	if userDID != "" {
		query += " AND user_did = ?"
		args = append(args, userDID)
	}
	if providerDID != "" {
		query += " AND provider_did = ?"
		args = append(args, providerDID)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}
	defer rows.Close()

	var payments []PendingPaymentRow
	for rows.Next() {
		var p PendingPaymentRow
		var nextRetry, createdAt string
		if err := rows.Scan(&p.ID, &p.ChargeID, &p.UserDID, &p.ProviderDID,
			&p.Amount, &p.Currency, &p.ResourceType, &p.Status, &p.Attempts, &nextRetry, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		p.NextRetry = parseSQLiteTime(nextRetry)
		p.CreatedAt = parseSQLiteTime(createdAt)
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

// Ensure json is used (for score history JSON operations)
var _ = json.Marshal

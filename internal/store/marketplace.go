package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrInsufficientBalance is returned when a wallet debit exceeds the available balance.
var ErrInsufficientBalance = errors.New("insufficient wallet balance")

// ---------------------------------------------------------------------------
// Wallet balance
// ---------------------------------------------------------------------------

// WalletBalanceRow holds a requester's prepaid sats balance.
type WalletBalanceRow struct {
	DID         string
	BalanceSats int64
	UpdatedAt   time.Time
}

// GetWalletBalance returns the current sats balance for the given DID.
// Returns 0 if no wallet record exists yet (new requester).
func (s *Store) GetWalletBalance(ctx context.Context, did string) (int64, error) {
	var bal int64
	err := s.db.QueryRowContext(ctx,
		`SELECT balance_sats FROM wallet_balances WHERE did = ?`, did,
	).Scan(&bal)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return bal, err
}

// CreditWallet adds sats to the wallet balance, creating the row if needed.
func (s *Store) CreditWallet(ctx context.Context, did string, sats int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO wallet_balances (did, balance_sats, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(did) DO UPDATE
		  SET balance_sats = balance_sats + excluded.balance_sats,
		      updated_at   = excluded.updated_at`,
		did, sats,
	)
	return err
}

// DebitWallet atomically subtracts sats from the wallet balance.
// Returns ErrInsufficientBalance when the current balance is less than sats.
func (s *Store) DebitWallet(ctx context.Context, did string, sats int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("debit wallet: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var bal int64
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(balance_sats, 0) FROM wallet_balances WHERE did = ?`, did,
	).Scan(&bal)
	if errors.Is(err, sql.ErrNoRows) {
		bal = 0
	} else if err != nil {
		return fmt.Errorf("debit wallet: read balance: %w", err)
	}
	if bal < sats {
		return ErrInsufficientBalance
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE wallet_balances
		   SET balance_sats = balance_sats - ?,
		       updated_at   = CURRENT_TIMESTAMP
		 WHERE did = ?`, sats, did,
	)
	if err != nil {
		return fmt.Errorf("debit wallet: update: %w", err)
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Wallet topups
// ---------------------------------------------------------------------------

// WalletTopupRow represents a single topup transaction.
type WalletTopupRow struct {
	TopupID        string
	DID            string
	AmountSats     int64
	Processor      string     // "lightning" | "stripe"
	Invoice        string     // bolt11 invoice or Stripe payment_intent ID
	Status         string     // "awaiting_payment" | "confirmed" | "expired" | "failed"
	IdempotencyKey string     // optional client-provided key for deduplication
	CreatedAt      time.Time
	ConfirmedAt    *time.Time
}

// CreateWalletTopup inserts a new topup record.
// If IdempotencyKey is non-empty and a row with that key already exists, the
// insert is silently skipped (ON CONFLICT DO NOTHING semantics via the unique
// index added in migration v5).
func (s *Store) CreateWalletTopup(ctx context.Context, t *WalletTopupRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO wallet_topups
		  (topup_id, did, amount_sats, processor, invoice, status, idempotency_key, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(idempotency_key) DO NOTHING`,
		t.TopupID, t.DID, t.AmountSats, t.Processor, t.Invoice, t.Status,
		t.IdempotencyKey, t.CreatedAt.UTC(),
	)
	return err
}

// GetWalletTopupByIdempotencyKey looks up an existing topup by idempotency key.
// Returns nil, nil when no matching row is found.
func (s *Store) GetWalletTopupByIdempotencyKey(ctx context.Context, key string) (*WalletTopupRow, error) {
	if key == "" {
		return nil, nil
	}
	t := &WalletTopupRow{}
	var confirmedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT topup_id, did, amount_sats, processor, invoice, status,
		       idempotency_key, created_at, confirmed_at
		  FROM wallet_topups WHERE idempotency_key = ?`, key,
	).Scan(
		&t.TopupID, &t.DID, &t.AmountSats, &t.Processor, &t.Invoice, &t.Status,
		&t.IdempotencyKey, &t.CreatedAt, &confirmedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if confirmedAt.Valid {
		t.ConfirmedAt = &confirmedAt.Time
	}
	return t, nil
}

// UpdateTopupStatus changes the status (and optionally sets confirmed_at) of a topup.
func (s *Store) UpdateTopupStatus(ctx context.Context, topupID, status string, confirmedAt *time.Time) error {
	if confirmedAt != nil {
		_, err := s.db.ExecContext(ctx, `
			UPDATE wallet_topups
			   SET status = ?, confirmed_at = ?
			 WHERE topup_id = ?`,
			status, confirmedAt.UTC(), topupID,
		)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE wallet_topups SET status = ? WHERE topup_id = ?`,
		status, topupID,
	)
	return err
}

// GetWalletTopup retrieves a single topup record by ID.
func (s *Store) GetWalletTopup(ctx context.Context, topupID string) (*WalletTopupRow, error) {
	t := &WalletTopupRow{}
	var confirmedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT topup_id, did, amount_sats, processor, invoice, status,
		       idempotency_key, created_at, confirmed_at
		  FROM wallet_topups WHERE topup_id = ?`, topupID,
	).Scan(
		&t.TopupID, &t.DID, &t.AmountSats, &t.Processor, &t.Invoice, &t.Status,
		&t.IdempotencyKey, &t.CreatedAt, &confirmedAt,
	)
	if err != nil {
		return nil, err
	}
	if confirmedAt.Valid {
		t.ConfirmedAt = &confirmedAt.Time
	}
	return t, nil
}

// ListWalletTopups returns topup history for a DID (most recent first).
func (s *Store) ListWalletTopups(ctx context.Context, did string, limit int) ([]WalletTopupRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT topup_id, did, amount_sats, processor, invoice, status,
		       idempotency_key, created_at, confirmed_at
		  FROM wallet_topups
		 WHERE did = ?
		 ORDER BY created_at DESC LIMIT ?`,
		did, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WalletTopupRow
	for rows.Next() {
		var t WalletTopupRow
		var confirmedAt sql.NullTime
		if err := rows.Scan(
			&t.TopupID, &t.DID, &t.AmountSats, &t.Processor, &t.Invoice, &t.Status,
			&t.IdempotencyKey, &t.CreatedAt, &confirmedAt,
		); err != nil {
			return nil, err
		}
		if confirmedAt.Valid {
			t.ConfirmedAt = &confirmedAt.Time
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

// OrderRow represents a marketplace purchase (workload or managed service).
type OrderRow struct {
	OrderID       string
	RequesterDID  string
	OrderType     string     // "workload" | "service"
	ResourceRefID string     // workload_id or service_instance_id
	Description   string
	CPUCores      float64
	MemoryMB      int64
	DiskGB        int64
	DurationHours int
	EstimatedSats int64
	ChargedSats   int64
	Status        string // "pending"|"running"|"completed"|"cancelled"|"failed"
	ManifestJSON  string // JSON-encoded WorkloadManifest for audit trail (Item 5)
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CreateOrder inserts a new order record.
func (s *Store) CreateOrder(ctx context.Context, o *OrderRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO orders
		  (order_id, requester_did, order_type, resource_ref_id, description,
		   cpu_cores, memory_mb, disk_gb, duration_hours,
		   estimated_sats, charged_sats, status, manifest_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.OrderID, o.RequesterDID, o.OrderType, o.ResourceRefID, o.Description,
		o.CPUCores, o.MemoryMB, o.DiskGB, o.DurationHours,
		o.EstimatedSats, o.ChargedSats, o.Status, o.ManifestJSON,
		o.CreatedAt.UTC(), o.UpdatedAt.UTC(),
	)
	return err
}

// UpdateOrderStatus changes the status of an order and sets updated_at.
func (s *Store) UpdateOrderStatus(ctx context.Context, orderID, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE orders
		   SET status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE order_id = ?`,
		status, orderID,
	)
	return err
}

// GetOrder retrieves a single order by ID.
func (s *Store) GetOrder(ctx context.Context, orderID string) (*OrderRow, error) {
	o := &OrderRow{}
	err := s.db.QueryRowContext(ctx, `
		SELECT order_id, requester_did, order_type, resource_ref_id, description,
		       cpu_cores, memory_mb, disk_gb, duration_hours,
		       estimated_sats, charged_sats, status, created_at, updated_at
		  FROM orders WHERE order_id = ?`, orderID,
	).Scan(
		&o.OrderID, &o.RequesterDID, &o.OrderType, &o.ResourceRefID, &o.Description,
		&o.CPUCores, &o.MemoryMB, &o.DiskGB, &o.DurationHours,
		&o.EstimatedSats, &o.ChargedSats, &o.Status, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// ListOrders returns orders for a requester (most recent first).
func (s *Store) ListOrders(ctx context.Context, requesterDID string, limit int) ([]OrderRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT order_id, requester_did, order_type, resource_ref_id, description,
		       cpu_cores, memory_mb, disk_gb, duration_hours,
		       estimated_sats, charged_sats, status, created_at, updated_at
		  FROM orders
		 WHERE requester_did = ?
		 ORDER BY created_at DESC LIMIT ?`,
		requesterDID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrderRow
	for rows.Next() {
		var o OrderRow
		if err := rows.Scan(
			&o.OrderID, &o.RequesterDID, &o.OrderType, &o.ResourceRefID, &o.Description,
			&o.CPUCores, &o.MemoryMB, &o.DiskGB, &o.DurationHours,
			&o.EstimatedSats, &o.ChargedSats, &o.Status, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

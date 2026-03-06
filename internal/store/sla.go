package store

import (
	"context"
	"time"
)

// RentalAuditRow is the store-layer representation of a rental engine decision.
type RentalAuditRow struct {
	RequestID string
	UserDID   string
	RuleID    string // empty when no rule matched
	Action    string // "accept" | "reject" | "pending"
	Reason    string
	DecidedAt time.Time
}

// InsertRentalAudit records a rental engine auto-accept/reject decision.
func (s *Store) InsertRentalAudit(ctx context.Context, row *RentalAuditRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO rental_audit (request_id, user_did, rule_id, action, reason, decided_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		row.RequestID,
		row.UserDID,
		row.RuleID,
		row.Action,
		row.Reason,
		row.DecidedAt.UTC(),
	)
	return err
}

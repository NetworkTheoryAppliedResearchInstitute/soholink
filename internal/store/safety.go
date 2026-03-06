package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ---------------------------------------------------------------------------
// Content hash blocklist (Item 1 — CSAM / illegal content)
// ---------------------------------------------------------------------------

// ContentHashRow describes a blocked content hash.
type ContentHashRow struct {
	HashSHA256  string
	HashType    string // "sha256"|"photodna"|"md5"
	Reason      string // "csam"|"illegal_content"|"known_malware"
	Source      string // "ncmec"|"manual"|"clamav_heuristic"
	AddedAt     time.Time
	AddedByDID  string
}

// AddContentHash inserts a hash into the blocklist (idempotent on conflict).
func (s *Store) AddContentHash(ctx context.Context, sha256hex, hashType, reason, source, byDID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO content_hash_blocklist
			(hash_sha256, hash_type, reason, source, added_by_did)
		VALUES (?, ?, ?, ?, ?)`,
		sha256hex, hashType, reason, source, byDID)
	return err
}

// IsHashBlocked returns (blocked, reason, error).
func (s *Store) IsHashBlocked(ctx context.Context, sha256hex string) (bool, string, error) {
	var reason string
	err := s.db.QueryRowContext(ctx,
		`SELECT reason FROM content_hash_blocklist WHERE hash_sha256 = ?`, sha256hex,
	).Scan(&reason)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, reason, nil
}

// ListContentHashes returns up to limit blocked hashes, newest first.
func (s *Store) ListContentHashes(ctx context.Context, limit int) ([]ContentHashRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT hash_sha256, hash_type, reason, source, added_at, added_by_did
		 FROM content_hash_blocklist ORDER BY added_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ContentHashRow
	for rows.Next() {
		var r ContentHashRow
		if err := rows.Scan(&r.HashSHA256, &r.HashType, &r.Reason, &r.Source, &r.AddedAt, &r.AddedByDID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// DID blocklist (Item 2 — federation-wide account suspension)
// ---------------------------------------------------------------------------

// BlockedDIDRow describes a blocked DID entry.
type BlockedDIDRow struct {
	DID        string
	Reason     string
	BlockedAt  time.Time
	BlockedBy  string
	ExpiresAt  *time.Time // nil = permanent
	Propagated bool
}

// BlockDID adds a DID to the blocklist. If the DID is already present, the
// reason and expiry are updated.
func (s *Store) BlockDID(ctx context.Context, did, reason, byDID string, expiresAt *time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO blocked_dids (did, reason, blocked_by, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(did) DO UPDATE SET
			reason     = excluded.reason,
			blocked_by = excluded.blocked_by,
			expires_at = excluded.expires_at,
			blocked_at = CURRENT_TIMESTAMP,
			propagated = 0`,
		did, reason, byDID, expiresAt)
	return err
}

// UnblockDID removes a DID from the blocklist.
func (s *Store) UnblockDID(ctx context.Context, did string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM blocked_dids WHERE did = ?`, did)
	return err
}

// IsDIDBlocked returns (blocked, reason, error). Expired entries are treated as
// not blocked (caller is responsible for periodic cleanup).
func (s *Store) IsDIDBlocked(ctx context.Context, did string) (bool, string, error) {
	var reason string
	var expiresAt *time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT reason, expires_at FROM blocked_dids WHERE did = ?`, did,
	).Scan(&reason, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return false, "", nil // expired — treat as not blocked
	}
	return true, reason, nil
}

// ListBlockedDIDs returns up to limit blocked DIDs, newest first.
func (s *Store) ListBlockedDIDs(ctx context.Context, limit int) ([]BlockedDIDRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT did, reason, blocked_at, blocked_by, expires_at, propagated
		 FROM blocked_dids ORDER BY blocked_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BlockedDIDRow
	for rows.Next() {
		var r BlockedDIDRow
		var prop int
		if err := rows.Scan(&r.DID, &r.Reason, &r.BlockedAt, &r.BlockedBy, &r.ExpiresAt, &prop); err != nil {
			return nil, err
		}
		r.Propagated = prop == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// FederationBlocklistSnapshot returns all permanent (non-expiring) blocked DIDs
// for federation pull by peer nodes.
func (s *Store) FederationBlocklistSnapshot(ctx context.Context) ([]BlockedDIDRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT did, reason, blocked_at, blocked_by, expires_at, propagated
		 FROM blocked_dids WHERE expires_at IS NULL ORDER BY blocked_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BlockedDIDRow
	for rows.Next() {
		var r BlockedDIDRow
		var prop int
		if err := rows.Scan(&r.DID, &r.Reason, &r.BlockedAt, &r.BlockedBy, &r.ExpiresAt, &prop); err != nil {
			return nil, err
		}
		r.Propagated = prop == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

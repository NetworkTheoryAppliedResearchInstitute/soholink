// Package moderation provides content safety and legal compliance infrastructure.
// It implements NCMEC-compatible CSAM hash matching, DID-based account blocklisting,
// workload manifest validation, and OPA safety policy evaluation.
//
// NCMEC compliance note: In production deployments the CSAMHashChecker.Check()
// method should be augmented with an HTTP call to the NCMEC CyberTipline API or
// Project VIC hash matching service. Both require NCMEC membership:
//
//	https://www.missingkids.org/gethelpnow/cybertipline
//
// For US-based platforms, 18 U.S.C. § 2258A mandates reporting within 24 hours
// of discovering CSAM. The local SQLite blocklist is a necessary starting point;
// it must be seeded with NCMEC-provided hashes upon membership approval.
package moderation

import (
	"context"
	"fmt"
	"log"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// ErrContentBlocked is returned when uploaded content matches a blocked hash.
// HTTP handlers should translate this to 451 Unavailable For Legal Reasons.
var ErrContentBlocked = fmt.Errorf("content blocked: matches illegal content hash")

// CSAMHashChecker checks content SHA-256 hashes against the local blocklist.
// The blocklist is seeded by platform administrators and can be augmented with
// NCMEC-provided hash sets upon membership approval.
type CSAMHashChecker struct {
	store *store.Store
}

// NewCSAMHashChecker creates a checker backed by the given store.
func NewCSAMHashChecker(s *store.Store) *CSAMHashChecker {
	return &CSAMHashChecker{store: s}
}

// Check returns (blocked bool, reason string, err error).
// sha256hex must be the lowercase hex-encoded SHA-256 of the raw file bytes.
//
// Production upgrade path: replace or augment this method with an HTTP call
// to NCMEC's hash matching API. The signature is intentionally kept simple so
// the underlying implementation can be swapped without changing callers.
func (c *CSAMHashChecker) Check(ctx context.Context, sha256hex string) (bool, string, error) {
	blocked, reason, err := c.store.IsHashBlocked(ctx, sha256hex)
	if err != nil {
		log.Printf("[moderation] hash check error for %s: %v", sha256hex[:8]+"...", err)
		return false, "", err
	}
	if blocked {
		log.Printf("[moderation] BLOCKED content hash matched: sha256=%s reason=%s", sha256hex[:16]+"...", reason)
	}
	return blocked, reason, nil
}

// AddHash adds a hash to the local blocklist. This is called by administrators
// to seed the blocklist from NCMEC-provided hash sets or manual reviews.
func (c *CSAMHashChecker) AddHash(ctx context.Context, sha256hex, reason, source, byDID string) error {
	if sha256hex == "" || reason == "" || source == "" {
		return fmt.Errorf("sha256hex, reason, and source are required")
	}
	if err := c.store.AddContentHash(ctx, sha256hex, "sha256", reason, source, byDID); err != nil {
		return fmt.Errorf("add hash: %w", err)
	}
	log.Printf("[moderation] hash blocklist entry added: reason=%s source=%s by=%s", reason, source, byDID)
	return nil
}

// ListHashes returns up to limit blocked hashes, newest first.
func (c *CSAMHashChecker) ListHashes(ctx context.Context, limit int) ([]store.ContentHashRow, error) {
	return c.store.ListContentHashes(ctx, limit)
}

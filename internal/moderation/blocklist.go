package moderation

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// ErrDIDBlocked is returned when a DID is on the platform blocklist.
// HTTP handlers should translate this to 403 Forbidden.
var ErrDIDBlocked = fmt.Errorf("account suspended by platform")

// DIDBlocklist maintains a hot in-memory cache of blocked DIDs backed by the
// store. The cache is populated lazily on first check and invalidated on writes.
type DIDBlocklist struct {
	store *store.Store
	mu    sync.RWMutex
	cache map[string]string // did → reason; "" means not cached
}

// NewDIDBlocklist creates a blocklist backed by the given store.
func NewDIDBlocklist(s *store.Store) *DIDBlocklist {
	return &DIDBlocklist{
		store: s,
		cache: make(map[string]string),
	}
}

// IsBlocked returns (blocked bool, reason string, err error).
// The hot-path uses the in-memory cache; on cache miss it falls through to
// the store. Expired entries are treated as not blocked.
func (b *DIDBlocklist) IsBlocked(ctx context.Context, did string) (bool, string, error) {
	if did == "" {
		return false, "", nil
	}

	// Fast-path: cache hit
	b.mu.RLock()
	reason, hit := b.cache[did]
	b.mu.RUnlock()
	if hit {
		return reason != "", reason, nil
	}

	// Slow-path: store lookup
	blocked, storeReason, err := b.store.IsDIDBlocked(ctx, did)
	if err != nil {
		return false, "", fmt.Errorf("blocklist check: %w", err)
	}

	// Populate cache (negative entries are cached as empty string)
	b.mu.Lock()
	if blocked {
		b.cache[did] = storeReason
	} else {
		b.cache[did] = ""
	}
	b.mu.Unlock()

	return blocked, storeReason, nil
}

// Block adds a DID to the blocklist and invalidates the cache entry.
func (b *DIDBlocklist) Block(ctx context.Context, did, reason, byDID string, expiresAt *time.Time) error {
	if did == "" || reason == "" {
		return fmt.Errorf("did and reason are required")
	}
	if err := b.store.BlockDID(ctx, did, reason, byDID, expiresAt); err != nil {
		return fmt.Errorf("block DID: %w", err)
	}
	b.mu.Lock()
	b.cache[did] = reason
	b.mu.Unlock()
	log.Printf("[moderation] DID blocked: did=%s reason=%s by=%s", did, reason, byDID)
	return nil
}

// Unblock removes a DID from the blocklist and clears the cache entry.
func (b *DIDBlocklist) Unblock(ctx context.Context, did string) error {
	if err := b.store.UnblockDID(ctx, did); err != nil {
		return fmt.Errorf("unblock DID: %w", err)
	}
	b.mu.Lock()
	delete(b.cache, did)
	b.mu.Unlock()
	log.Printf("[moderation] DID unblocked: did=%s", did)
	return nil
}

// ListBlocked returns up to limit blocked DIDs, newest first.
func (b *DIDBlocklist) ListBlocked(ctx context.Context, limit int) ([]store.BlockedDIDRow, error) {
	return b.store.ListBlockedDIDs(ctx, limit)
}

// FederationSnapshot returns all permanent blocks for federation peer pull.
func (b *DIDBlocklist) FederationSnapshot(ctx context.Context) ([]store.BlockedDIDRow, error) {
	return b.store.FederationBlocklistSnapshot(ctx)
}

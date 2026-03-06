package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// node_info keys for owner identity.
const (
	keyOwnerPublicKey = "owner_public_key" // base64-encoded Ed25519 public key (32 bytes)
	keyOwnerDID       = "owner_did"        // did:key shorthand for display

	// Node signing keypair — used for federation announcements.
	// Unlike the owner keypair, BOTH keys are stored so the node can sign
	// heartbeats autonomously without operator involvement.
	keyNodeSignPriv = "node_sign_priv" // hex-encoded 32-byte seed
	keyNodeSignPub  = "node_sign_pub"  // base64-encoded 32-byte public key
)

// EnsureOwnerKeypair checks whether an owner public key has been stored.
// If not, it generates a fresh Ed25519 keypair, persists the public key in
// node_info, and returns the 32-byte seed as a 64-char hex string — the only
// time the private key material is ever available in plaintext.
//
// If the keypair already exists the returned privateKeyHex is empty and
// isNew is false; the caller should not log anything in that case.
func (s *Store) EnsureOwnerKeypair(ctx context.Context) (privateKeyHex string, isNew bool, err error) {
	existing, err := s.GetNodeInfo(ctx, keyOwnerPublicKey)
	if err != nil {
		return "", false, fmt.Errorf("EnsureOwnerKeypair: %w", err)
	}
	if existing != "" {
		return "", false, nil // already configured
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", false, fmt.Errorf("EnsureOwnerKeypair generate: %w", err)
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	if err := s.SetNodeInfo(ctx, keyOwnerPublicKey, pubB64); err != nil {
		return "", false, fmt.Errorf("EnsureOwnerKeypair store pubkey: %w", err)
	}

	// Derive a lightweight DID for display purposes.
	did := "did:key:z" + hex.EncodeToString(pub[:8])
	_ = s.SetNodeInfo(ctx, keyOwnerDID, did)

	// Return the seed (32 bytes) as a 64-character hex string.
	// The caller is responsible for displaying it exactly once.
	return hex.EncodeToString(priv.Seed()), true, nil
}

// GetOwnerPublicKey returns the base64-encoded Ed25519 public key stored in
// node_info. Returns ("", false, nil) if not yet initialised.
func (s *Store) GetOwnerPublicKey(ctx context.Context) (pubB64 string, ok bool, err error) {
	v, err := s.GetNodeInfo(ctx, keyOwnerPublicKey)
	if err != nil {
		return "", false, err
	}
	return v, v != "", nil
}

// StoreDeviceToken persists a new device session.
// tokenRawHex is the 64-char hex representation of a random 32-byte token;
// only its SHA-256 hash is stored so a database leak cannot replay tokens.
func (s *Store) StoreDeviceToken(ctx context.Context, tokenRawHex, deviceName string) error {
	h := sha256.Sum256([]byte(tokenRawHex))
	hash := hex.EncodeToString(h[:])
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO device_tokens (token_hash, device_name, created_at, last_seen, revoked)
		 VALUES (?, ?, ?, ?, 0)
		 ON CONFLICT(token_hash) DO NOTHING`,
		hash, deviceName, now, now)
	return err
}

// ValidateDeviceToken checks whether a raw token is valid (exists and not
// revoked). It updates last_seen on every successful validation.
func (s *Store) ValidateDeviceToken(ctx context.Context, tokenRawHex string) (bool, error) {
	h := sha256.Sum256([]byte(tokenRawHex))
	hash := hex.EncodeToString(h[:])

	var revoked int
	err := s.db.QueryRowContext(ctx,
		"SELECT revoked FROM device_tokens WHERE token_hash = ?", hash).Scan(&revoked)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if revoked == 1 {
		return false, nil
	}

	// Bump last_seen (best-effort — ignore error).
	_, _ = s.db.ExecContext(ctx,
		"UPDATE device_tokens SET last_seen = ? WHERE token_hash = ?",
		time.Now().UTC(), hash)
	return true, nil
}

// RevokeDeviceToken marks a token as revoked by its raw hex value.
func (s *Store) RevokeDeviceToken(ctx context.Context, tokenRawHex string) error {
	h := sha256.Sum256([]byte(tokenRawHex))
	hash := hex.EncodeToString(h[:])
	_, err := s.db.ExecContext(ctx,
		"UPDATE device_tokens SET revoked = 1 WHERE token_hash = ?", hash)
	return err
}

// ---------------------------------------------------------------------------
// Node signing keypair (for federation announcements)
// Both keys are persisted — this is a machine identity, not a user secret.
// ---------------------------------------------------------------------------

// EnsureNodeSigningKeypair generates and stores an Ed25519 keypair for
// signing federation announcements if one does not already exist.
// Returns the public key as base64 on every call (new or existing).
func (s *Store) EnsureNodeSigningKeypair(ctx context.Context) (pubKeyB64 string, err error) {
	existing, err := s.GetNodeInfo(ctx, keyNodeSignPub)
	if err != nil {
		return "", fmt.Errorf("EnsureNodeSigningKeypair: %w", err)
	}
	if existing != "" {
		return existing, nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("EnsureNodeSigningKeypair generate: %w", err)
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privHex := hex.EncodeToString(priv.Seed())

	if err := s.SetNodeInfo(ctx, keyNodeSignPub, pubB64); err != nil {
		return "", fmt.Errorf("EnsureNodeSigningKeypair store pub: %w", err)
	}
	if err := s.SetNodeInfo(ctx, keyNodeSignPriv, privHex); err != nil {
		return "", fmt.Errorf("EnsureNodeSigningKeypair store priv: %w", err)
	}
	return pubB64, nil
}

// GetNodeSigningKey returns the node's federation signing keypair.
// privSeed is a 64-char hex string (32-byte Ed25519 seed).
// pubKeyB64 is a base64-encoded 32-byte Ed25519 public key.
// Returns ("", "", nil) if the keypair has not been generated yet.
func (s *Store) GetNodeSigningKey(ctx context.Context) (privSeedHex, pubKeyB64 string, err error) {
	priv, err := s.GetNodeInfo(ctx, keyNodeSignPriv)
	if err != nil {
		return "", "", err
	}
	pub, err := s.GetNodeInfo(ctx, keyNodeSignPub)
	if err != nil {
		return "", "", err
	}
	return priv, pub, nil
}

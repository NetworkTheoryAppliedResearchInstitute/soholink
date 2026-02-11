package blockchain

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

func TestLocalChainSubmitAndVerify(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	_ = pub

	chain := NewLocalChain(s, "did:key:test123", priv)

	ctx := context.Background()

	// Submit first batch (genesis)
	root1 := []byte("merkle-root-hash-1-aaaaaabbbbbb")
	txHash1, height1, err := chain.SubmitBatch(ctx, root1, BatchMetadata{
		SourceFile: "2026-02-08.jsonl",
		LeafCount:  100,
		TreeHeight: 7,
		NodeDID:    "did:key:test123",
	})
	if err != nil {
		t.Fatalf("submit batch 1 failed: %v", err)
	}
	if height1 != 0 {
		t.Errorf("expected height 0, got %d", height1)
	}
	if txHash1 == "" {
		t.Error("expected non-empty tx hash")
	}

	// Submit second batch
	root2 := []byte("merkle-root-hash-2-ccccccdddddd")
	txHash2, height2, err := chain.SubmitBatch(ctx, root2, BatchMetadata{
		SourceFile: "2026-02-08.jsonl",
		LeafCount:  50,
		TreeHeight: 6,
		NodeDID:    "did:key:test123",
	})
	if err != nil {
		t.Fatalf("submit batch 2 failed: %v", err)
	}
	if height2 != 1 {
		t.Errorf("expected height 1, got %d", height2)
	}

	// Verify batch 1
	valid, err := chain.VerifyBatch(ctx, txHash1, root1)
	if err != nil {
		t.Fatalf("verify batch 1 failed: %v", err)
	}
	if !valid {
		t.Error("expected batch 1 to be valid")
	}

	// Verify batch 2
	valid, err = chain.VerifyBatch(ctx, txHash2, root2)
	if err != nil {
		t.Fatalf("verify batch 2 failed: %v", err)
	}
	if !valid {
		t.Error("expected batch 2 to be valid")
	}

	// Verify with wrong root should fail
	wrongRoot := []byte("wrong-root-hash-xxxxxxxxyyyyyy")
	valid, err = chain.VerifyBatch(ctx, txHash1, wrongRoot)
	if err != nil {
		t.Fatalf("verify with wrong root returned error: %v", err)
	}
	if valid {
		t.Error("expected verification to fail with wrong root")
	}

	// Check chain height
	h, err := chain.ChainHeight(ctx)
	if err != nil {
		t.Fatalf("chain height failed: %v", err)
	}
	if h != 1 {
		t.Errorf("expected chain height 1, got %d", h)
	}

	// Verify chain integrity
	if err := chain.VerifyChainIntegrity(ctx); err != nil {
		t.Fatalf("chain integrity check failed: %v", err)
	}

	// Get latest checkpoint
	cp, err := chain.GetLatestCheckpoint(ctx)
	if err != nil {
		t.Fatalf("get latest checkpoint failed: %v", err)
	}
	if cp == nil {
		t.Fatal("expected non-nil checkpoint")
	}
	if cp.Height != 1 {
		t.Errorf("expected checkpoint height 1, got %d", cp.Height)
	}
}

func TestComputeBlockHash(t *testing.T) {
	// Ensure same inputs produce same hash
	root := []byte("test-root")
	prev := make([]byte, 32)
	ts := time.Now()

	hash1 := computeBlockHash(0, root, prev, ts, "did:key:test")
	hash2 := computeBlockHash(0, root, prev, ts, "did:key:test")

	if len(hash1) != 32 {
		t.Errorf("expected 32-byte hash, got %d", len(hash1))
	}

	for i := range hash1 {
		if hash1[i] != hash2[i] {
			t.Fatal("same inputs produced different hashes")
		}
	}

	// Different inputs produce different hash
	hash3 := computeBlockHash(1, root, prev, ts, "did:key:test")
	same := true
	for i := range hash1 {
		if hash1[i] != hash3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different inputs produced same hash")
	}
}

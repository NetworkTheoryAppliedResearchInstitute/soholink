package blockchain

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

func TestBlockchainMerkleIntegration(t *testing.T) {
	// Create test store
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Generate test key pair
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = pubKey

	nodeDID := "did:soho:test-node"

	// Create blockchain
	chain := NewLocalChain(s, nodeDID, privKey)

	ctx := context.Background()

	// Test 1: Submit genesis block
	t.Run("submit genesis block", func(t *testing.T) {
		merkleRoot := []byte("test-merkle-root-0000000000000000")
		metadata := BatchMetadata{
			SourceFile: "2025-01-01.jsonl",
			LeafCount:  100,
			TreeHeight: 7,
			NodeDID:    nodeDID,
		}

		txHash, height, err := chain.SubmitBatch(ctx, merkleRoot, metadata)
		if err != nil {
			t.Fatalf("Failed to submit genesis block: %v", err)
		}

		if height != 0 {
			t.Errorf("Expected genesis block height 0, got %d", height)
		}

		if txHash == "" {
			t.Error("Expected non-empty transaction hash")
		}

		// Verify the block
		valid, err := chain.VerifyBatch(ctx, txHash, merkleRoot)
		if err != nil {
			t.Fatalf("Failed to verify batch: %v", err)
		}

		if !valid {
			t.Error("Batch verification failed")
		}
	})

	// Test 2: Submit multiple blocks
	t.Run("submit chain of blocks", func(t *testing.T) {
		for i := 1; i <= 5; i++ {
			merkleRoot := []byte("test-merkle-root-" + string(rune('0'+i)))
			metadata := BatchMetadata{
				SourceFile: "test.jsonl",
				LeafCount:  100 * i,
				TreeHeight: 7,
				NodeDID:    nodeDID,
			}

			txHash, height, err := chain.SubmitBatch(ctx, merkleRoot, metadata)
			if err != nil {
				t.Fatalf("Failed to submit block %d: %v", i, err)
			}

			if height != uint64(i) {
				t.Errorf("Expected block height %d, got %d", i, height)
			}

			// Verify the block
			valid, err := chain.VerifyBatch(ctx, txHash, merkleRoot)
			if err != nil {
				t.Fatalf("Failed to verify batch %d: %v", i, err)
			}

			if !valid {
				t.Errorf("Batch %d verification failed", i)
			}
		}

		// Verify chain height
		height, err := chain.ChainHeight(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if height != 5 {
			t.Errorf("Expected chain height 5, got %d", height)
		}
	})

	// Test 3: Verify chain integrity
	t.Run("verify chain integrity", func(t *testing.T) {
		err := chain.VerifyChainIntegrity(ctx)
		if err != nil {
			t.Errorf("Chain integrity check failed: %v", err)
		}
	})

	// Test 4: Get latest checkpoint
	t.Run("get latest checkpoint", func(t *testing.T) {
		checkpoint, err := chain.GetLatestCheckpoint(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if checkpoint == nil {
			t.Fatal("Expected checkpoint, got nil")
		}

		if checkpoint.Height != 5 {
			t.Errorf("Expected checkpoint height 5, got %d", checkpoint.Height)
		}

		if len(checkpoint.RootHash) == 0 {
			t.Error("Expected non-empty root hash in checkpoint")
		}

		if len(checkpoint.BlockHash) == 0 {
			t.Error("Expected non-empty block hash in checkpoint")
		}
	})

	// Test 5: Get specific block
	t.Run("get specific block", func(t *testing.T) {
		block, err := chain.GetBlock(ctx, 3)
		if err != nil {
			t.Fatal(err)
		}

		if block.Height != 3 {
			t.Errorf("Expected block height 3, got %d", block.Height)
		}

		if len(block.MerkleRoot) == 0 {
			t.Error("Expected non-empty Merkle root")
		}

		if len(block.Signature) == 0 {
			t.Error("Expected non-empty signature")
		}

		if block.NodeDID != nodeDID {
			t.Errorf("Expected node DID %s, got %s", nodeDID, block.NodeDID)
		}
	})

	// Test 6: Verify batch with wrong root fails
	t.Run("verify batch with wrong root fails", func(t *testing.T) {
		// Get latest block's tx hash
		checkpoint, err := chain.GetLatestCheckpoint(ctx)
		if err != nil {
			t.Fatal(err)
		}

		wrongRoot := []byte("wrong-merkle-root-0000000000000")

		valid, err := chain.VerifyBatch(ctx, string(checkpoint.BlockHash), wrongRoot)
		if err != nil {
			t.Fatal(err)
		}

		if valid {
			t.Error("Expected verification to fail with wrong root, but it succeeded")
		}
	})
}

func TestMerkleBlockchainProofVerification(t *testing.T) {
	// Create test store
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Generate test key pair
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	nodeDID := "did:soho:test-node"
	chain := NewLocalChain(s, nodeDID, privKey)
	ctx := context.Background()

	// Submit a block with known Merkle root
	merkleRoot := []byte("known-merkle-root-for-proof-test")
	metadata := BatchMetadata{
		SourceFile: "2025-01-01.jsonl",
		LeafCount:  100,
		TreeHeight: 7,
		NodeDID:    nodeDID,
	}

	txHash, blockHeight, err := chain.SubmitBatch(ctx, merkleRoot, metadata)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("verify proof against blockchain", func(t *testing.T) {
		// Simulate verifying a Merkle proof against the anchored root
		// In reality, you would:
		// 1. Get the block by txHash
		// 2. Extract the Merkle root from the block
		// 3. Verify the proof leads to that root

		block, err := chain.GetBlock(ctx, blockHeight)
		if err != nil {
			t.Fatalf("Failed to get block %d: %v", blockHeight, err)
		}

		// Verify the root matches
		if len(block.MerkleRoot) != len(merkleRoot) {
			t.Fatal("Merkle root length mismatch")
		}

		for i := range merkleRoot {
			if block.MerkleRoot[i] != merkleRoot[i] {
				t.Fatal("Merkle root mismatch in blockchain")
			}
		}

		// Verify metadata was preserved
		if block.Metadata.LeafCount != metadata.LeafCount {
			t.Errorf("Expected leaf count %d, got %d", metadata.LeafCount, block.Metadata.LeafCount)
		}

		if block.Metadata.TreeHeight != metadata.TreeHeight {
			t.Errorf("Expected tree height %d, got %d", metadata.TreeHeight, block.Metadata.TreeHeight)
		}

		if block.Metadata.SourceFile != metadata.SourceFile {
			t.Errorf("Expected source file %s, got %s", metadata.SourceFile, block.Metadata.SourceFile)
		}
	})

	t.Run("verify transaction hash resolves to block", func(t *testing.T) {
		// Verify that the txHash returned by SubmitBatch can be used to verify the batch
		valid, err := chain.VerifyBatch(ctx, txHash, merkleRoot)
		if err != nil {
			t.Fatalf("Failed to verify batch: %v", err)
		}

		if !valid {
			t.Error("Batch verification failed for valid root")
		}
	})
}

func TestChainIntegrityDetectsT ampering(t *testing.T) {
	// Create test store
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Generate test key pair
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	nodeDID := "did:soho:test-node"
	chain := NewLocalChain(s, nodeDID, privKey)
	ctx := context.Background()

	// Submit a few blocks
	for i := 0; i < 3; i++ {
		merkleRoot := []byte("test-merkle-root-" + string(rune('0'+i)))
		metadata := BatchMetadata{
			SourceFile: "test.jsonl",
			LeafCount:  100,
			TreeHeight: 7,
			NodeDID:    nodeDID,
		}

		_, _, err := chain.SubmitBatch(ctx, merkleRoot, metadata)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Verify integrity before tampering
	t.Run("chain integrity valid before tampering", func(t *testing.T) {
		err := chain.VerifyChainIntegrity(ctx)
		if err != nil {
			t.Errorf("Chain integrity check should pass, but got: %v", err)
		}
	})

	// Tamper with a block in the database
	t.Run("detect tampering", func(t *testing.T) {
		// Modify block 1's Merkle root directly in the database
		_, err := s.DB().ExecContext(ctx,
			"UPDATE blockchain_batches SET merkle_root = ? WHERE height = ?",
			[]byte("tampered-root"), 1)
		if err != nil {
			t.Fatal(err)
		}

		// Reload the chain
		chain2 := NewLocalChain(s, nodeDID, privKey)

		// Verify integrity - should detect tampering
		err = chain2.VerifyChainIntegrity(ctx)
		if err == nil {
			t.Error("Expected chain integrity check to fail after tampering, but it passed")
		}
	})
}

func TestEmptyChain(t *testing.T) {
	// Create test store
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	nodeDID := "did:soho:test-node"
	chain := NewLocalChain(s, nodeDID, privKey)
	ctx := context.Background()

	t.Run("empty chain height is 0", func(t *testing.T) {
		height, err := chain.ChainHeight(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if height != 0 {
			t.Errorf("Expected empty chain height 0, got %d", height)
		}
	})

	t.Run("empty chain checkpoint is nil", func(t *testing.T) {
		checkpoint, err := chain.GetLatestCheckpoint(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if checkpoint != nil {
			t.Error("Expected nil checkpoint for empty chain")
		}
	})

	t.Run("empty chain integrity check passes", func(t *testing.T) {
		err := chain.VerifyChainIntegrity(ctx)
		if err != nil {
			t.Errorf("Empty chain integrity should pass, got: %v", err)
		}
	})
}

func TestConcurrentBlockSubmission(t *testing.T) {
	// Create test store
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	nodeDID := "did:soho:test-node"
	chain := NewLocalChain(s, nodeDID, privKey)
	ctx := context.Background()

	// Submit blocks sequentially (LocalChain uses mutex to serialize)
	t.Run("sequential block submission", func(t *testing.T) {
		done := make(chan bool)

		for i := 0; i < 10; i++ {
			go func(idx int) {
				merkleRoot := []byte("concurrent-root-" + string(rune('0'+idx)))
				metadata := BatchMetadata{
					SourceFile: "test.jsonl",
					LeafCount:  100,
					TreeHeight: 7,
					NodeDID:    nodeDID,
				}

				_, _, err := chain.SubmitBatch(ctx, merkleRoot, metadata)
				if err != nil {
					t.Errorf("Failed to submit block %d: %v", idx, err)
				}
				done <- true
			}(i)
		}

		// Wait for all submissions
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify chain height
		height, err := chain.ChainHeight(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if height != 9 {
			t.Errorf("Expected chain height 9, got %d", height)
		}

		// Verify chain integrity
		err = chain.VerifyChainIntegrity(ctx)
		if err != nil {
			t.Errorf("Chain integrity check failed: %v", err)
		}
	})
}

package blockchain

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// Block represents a single block in the local chain.
type Block struct {
	Height     uint64
	MerkleRoot []byte
	PrevHash   []byte
	Hash       []byte
	Timestamp  time.Time
	NodeDID    string
	Signature  []byte
	Metadata   BatchMetadata
}

// LocalChain implements Chain using an append-only SQLite-backed block store.
type LocalChain struct {
	store      *store.Store
	nodeDID    string
	privateKey ed25519.PrivateKey

	mu          sync.RWMutex
	latestBlock *Block
}

// NewLocalChain creates a new local blockchain instance.
func NewLocalChain(s *store.Store, nodeDID string, privateKey ed25519.PrivateKey) *LocalChain {
	lc := &LocalChain{
		store:      s,
		nodeDID:    nodeDID,
		privateKey: privateKey,
	}
	// Load latest block from store
	if latest, err := s.GetLatestBlockchainBatch(context.Background()); err == nil && latest != nil {
		lc.latestBlock = &Block{
			Height:     uint64(latest.Height), // #nosec G115 -- block heights are non-negative by protocol invariant
			MerkleRoot: latest.MerkleRoot,
			PrevHash:   latest.PrevHash,
			Hash:       latest.Hash,
			Timestamp:  latest.Timestamp,
			NodeDID:    latest.NodeDID,
			Signature:  latest.Signature,
		}
	}
	return lc
}

// SubmitBatch creates a new block containing the given Merkle root.
func (lc *LocalChain) SubmitBatch(ctx context.Context, root []byte, metadata BatchMetadata) (string, uint64, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	var prevHash []byte
	var height uint64

	if lc.latestBlock != nil {
		prevHash = lc.latestBlock.Hash
		height = lc.latestBlock.Height + 1
	} else {
		prevHash = make([]byte, 32) // genesis block has zero prev hash
		height = 0
	}

	now := time.Now().UTC()

	// Compute block hash: SHA3-256(height || merkle_root || prev_hash || timestamp || node_did)
	blockHash := computeBlockHash(height, root, prevHash, now, lc.nodeDID)

	// Sign the block hash
	var signature []byte
	if lc.privateKey != nil {
		signature = ed25519.Sign(lc.privateKey, blockHash)
	}

	block := &Block{
		Height:     height,
		MerkleRoot: root,
		PrevHash:   prevHash,
		Hash:       blockHash,
		Timestamp:  now,
		NodeDID:    lc.nodeDID,
		Signature:  signature,
		Metadata:   metadata,
	}

	// Persist to store
	row := &store.BlockchainBatchRow{
		Height:     int64(height),
		MerkleRoot: root,
		PrevHash:   prevHash,
		Hash:       blockHash,
		Timestamp:  now,
		NodeDID:    lc.nodeDID,
		Signature:  signature,
		SourceFile: metadata.SourceFile,
		LeafCount:  metadata.LeafCount,
		TreeHeight: metadata.TreeHeight,
	}
	if err := lc.store.CreateBlockchainBatch(ctx, row); err != nil {
		return "", 0, fmt.Errorf("failed to persist block: %w", err)
	}

	lc.latestBlock = block
	txHash := hex.EncodeToString(blockHash)

	log.Printf("[blockchain] block %d anchored (root=%s, tx=%s)", height, hex.EncodeToString(root)[:16]+"...", txHash[:16]+"...")
	return txHash, height, nil
}

// VerifyBatch checks that a block at the given txHash contains the expected root.
func (lc *LocalChain) VerifyBatch(ctx context.Context, txHash string, expectedRoot []byte) (bool, error) {
	hashBytes, err := hex.DecodeString(txHash)
	if err != nil {
		return false, fmt.Errorf("invalid tx hash: %w", err)
	}

	row, err := lc.store.GetBlockchainBatchByHash(ctx, hashBytes)
	if err != nil {
		return false, fmt.Errorf("block not found: %w", err)
	}

	if len(row.MerkleRoot) != len(expectedRoot) {
		return false, nil
	}
	for i := range row.MerkleRoot {
		if row.MerkleRoot[i] != expectedRoot[i] {
			return false, nil
		}
	}

	// Verify chain integrity: recompute hash and check
	recomputed := computeBlockHash(uint64(row.Height), row.MerkleRoot, row.PrevHash, row.Timestamp, row.NodeDID) // #nosec G115 -- block heights are non-negative by protocol invariant
	for i := range recomputed {
		if recomputed[i] != row.Hash[i] {
			return false, fmt.Errorf("block hash mismatch: chain integrity violated")
		}
	}

	return true, nil
}

// GetLatestCheckpoint returns the most recent block as a checkpoint.
func (lc *LocalChain) GetLatestCheckpoint(ctx context.Context) (*Checkpoint, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if lc.latestBlock == nil {
		return nil, nil
	}

	return &Checkpoint{
		Height:    lc.latestBlock.Height,
		BlockHash: lc.latestBlock.Hash,
		RootHash:  lc.latestBlock.MerkleRoot,
		Timestamp: lc.latestBlock.Timestamp,
	}, nil
}

// GetBlock returns a specific block by height.
func (lc *LocalChain) GetBlock(ctx context.Context, height uint64) (*Block, error) {
	row, err := lc.store.GetBlockchainBatchByHeight(ctx, int64(height)) // #nosec G115 -- height is a uint64 block index; blockchain heights fit in int64 (< 2^63)
	if err != nil {
		return nil, fmt.Errorf("block %d not found: %w", height, err)
	}

	return &Block{
		Height:     uint64(row.Height), // #nosec G115 -- block heights are non-negative by protocol invariant
		MerkleRoot: row.MerkleRoot,
		PrevHash:   row.PrevHash,
		Hash:       row.Hash,
		Timestamp:  row.Timestamp,
		NodeDID:    row.NodeDID,
		Signature:  row.Signature,
		Metadata: BatchMetadata{
			SourceFile: row.SourceFile,
			LeafCount:  row.LeafCount,
			TreeHeight: row.TreeHeight,
		},
	}, nil
}

// ChainHeight returns the current chain height.
func (lc *LocalChain) ChainHeight(ctx context.Context) (uint64, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if lc.latestBlock == nil {
		return 0, nil
	}
	return lc.latestBlock.Height, nil
}

// VerifyChainIntegrity walks the entire chain and verifies hash linkage.
func (lc *LocalChain) VerifyChainIntegrity(ctx context.Context) error {
	// Empty chain is always valid.
	lc.mu.RLock()
	isEmpty := lc.latestBlock == nil
	lc.mu.RUnlock()
	if isEmpty {
		return nil
	}

	height, err := lc.ChainHeight(ctx)
	if err != nil {
		return err
	}

	var prevHash []byte
	for h := uint64(0); h <= height; h++ {
		block, err := lc.GetBlock(ctx, h)
		if err != nil {
			return fmt.Errorf("missing block %d: %w", h, err)
		}

		// Check prev hash linkage
		if h == 0 {
			// Genesis block prev hash should be all zeros
			for _, b := range block.PrevHash {
				if b != 0 {
					return fmt.Errorf("genesis block has non-zero prev hash")
				}
			}
		} else {
			if len(prevHash) != len(block.PrevHash) {
				return fmt.Errorf("block %d: prev hash length mismatch", h)
			}
			for i := range prevHash {
				if prevHash[i] != block.PrevHash[i] {
					return fmt.Errorf("block %d: prev hash mismatch (chain broken)", h)
				}
			}
		}

		// Verify block hash
		recomputed := computeBlockHash(block.Height, block.MerkleRoot, block.PrevHash, block.Timestamp, block.NodeDID)
		if len(recomputed) != len(block.Hash) {
			return fmt.Errorf("block %d: hash length mismatch", h)
		}
		for i := range recomputed {
			if recomputed[i] != block.Hash[i] {
				return fmt.Errorf("block %d: hash mismatch (tampering detected)", h)
			}
		}

		prevHash = block.Hash
	}

	return nil
}

// computeBlockHash computes SHA3-256 of the block's canonical fields.
func computeBlockHash(height uint64, merkleRoot, prevHash []byte, timestamp time.Time, nodeDID string) []byte {
	h := sha3.New256()

	// Height as 8-byte big-endian
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, height)
	h.Write(heightBytes)

	h.Write(merkleRoot)
	h.Write(prevHash)

	// Timestamp as Unix seconds (8-byte big-endian)
	ts := uint64(timestamp.Unix()) // #nosec G115 -- blockchain timestamps are always after the Unix epoch
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, ts)
	h.Write(tsBytes)

	h.Write([]byte(nodeDID))

	return h.Sum(nil)
}

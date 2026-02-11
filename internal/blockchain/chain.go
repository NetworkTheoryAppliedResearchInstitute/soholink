package blockchain

import (
	"context"
	"time"
)

// BatchMetadata holds information about a Merkle batch being anchored.
type BatchMetadata struct {
	SourceFile string
	LeafCount  int
	TreeHeight int
	NodeDID    string
}

// Checkpoint represents a verified point in the blockchain.
type Checkpoint struct {
	Height    uint64
	BlockHash []byte
	RootHash  []byte
	Timestamp time.Time
}

// Chain is the interface for blockchain backends.
// The local chain is the default; Ethereum/Polygon can be added later.
type Chain interface {
	// SubmitBatch anchors a Merkle root hash on the chain.
	SubmitBatch(ctx context.Context, root []byte, metadata BatchMetadata) (txHash string, blockHeight uint64, err error)

	// VerifyBatch checks that a previously submitted batch matches the chain.
	VerifyBatch(ctx context.Context, txHash string, expectedRoot []byte) (bool, error)

	// GetLatestCheckpoint returns the most recent anchored checkpoint.
	GetLatestCheckpoint(ctx context.Context) (*Checkpoint, error)

	// GetBlock returns a specific block by height.
	GetBlock(ctx context.Context, height uint64) (*Block, error)

	// ChainHeight returns the current chain height.
	ChainHeight(ctx context.Context) (uint64, error)
}

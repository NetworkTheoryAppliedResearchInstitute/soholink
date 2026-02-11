package lbtas

import (
	"time"
)

// TransactionState represents the current state of a resource transaction.
type TransactionState string

const (
	StateInitiated              TransactionState = "initiated"
	StateExecuting              TransactionState = "executing"
	StateAwaitingProviderRating TransactionState = "awaiting_provider_rating"
	StateResultsEscrowed        TransactionState = "results_escrowed"
	StateAwaitingUserRating     TransactionState = "awaiting_user_rating"
	StateCompleted              TransactionState = "completed"
	StateDisputed               TransactionState = "disputed"
	StateCancelled              TransactionState = "cancelled"
	StateTimedOut               TransactionState = "timed_out"
)

// ResourceTransaction tracks a complete resource sharing transaction
// including payment escrow, result delivery, and bidirectional LBTAS ratings.
type ResourceTransaction struct {
	TransactionID string
	UserDID       string
	ProviderDID   string
	ResourceType  string
	ResourceID    string

	// State tracking
	State     TransactionState
	CreatedAt time.Time
	UpdatedAt time.Time

	// Payment escrow
	PaymentAmount   int64
	PaymentCurrency string
	PaymentEscrowed bool
	PaymentProof    []byte

	// Results escrow
	ResultsReady bool
	ResultsHash  [32]byte
	ResultsPath  string
	ResultsKey   []byte // Decryption key released after rating

	// LBTAS ratings
	ProviderRating *LBTASRating // Provider's rating of user
	UserRating     *LBTASRating // User's rating of provider

	// Timeouts
	RatingDeadline time.Time

	// Dispute
	DisputeID     *string
	DisputeReason string

	// Blockchain anchor
	BlockchainAnchor *BlockchainAnchor

	// Signatures
	UserSignature     []byte
	ProviderSignature []byte
}

// LBTASRating represents a single LBTAS rating (0-5 scale).
type LBTASRating struct {
	Score     int       // 0-5
	Category  string    // e.g. "payment_reliability", "execution_quality"
	Feedback  string    // Optional text feedback (max 500 chars)
	Evidence  []byte    // Optional proof (logs, screenshots)
	Timestamp time.Time
	Signature []byte    // Ed25519 signature
}

// BlockchainAnchor records when a transaction was committed to the blockchain.
type BlockchainAnchor struct {
	BlockHeight uint64
	DataHash    [32]byte
}

// Bytes serializes the rating for signing.
func (r *LBTASRating) Bytes() []byte {
	// Simple deterministic serialization for signing
	data := make([]byte, 0, 256)
	data = append(data, byte(r.Score))
	data = append(data, []byte(r.Category)...)
	data = append(data, []byte(r.Feedback)...)
	ts := r.Timestamp.UTC().Unix()
	data = append(data,
		byte(ts>>56), byte(ts>>48), byte(ts>>40), byte(ts>>32),
		byte(ts>>24), byte(ts>>16), byte(ts>>8), byte(ts),
	)
	return data
}

// TransactionSummary is a compact representation for blockchain commitment.
type TransactionSummary struct {
	TransactionID  string
	UserDID        string
	ProviderDID    string
	ResourceType   string
	PaymentAmount  int64
	ProviderRating int
	UserRating     int
	CompletedAt    time.Time
	ResultsHash    [32]byte
}

// Bytes serializes the summary for hashing.
func (s *TransactionSummary) Bytes() []byte {
	data := make([]byte, 0, 512)
	data = append(data, []byte(s.TransactionID)...)
	data = append(data, []byte(s.UserDID)...)
	data = append(data, []byte(s.ProviderDID)...)
	data = append(data, []byte(s.ResourceType)...)
	data = append(data, s.ResultsHash[:]...)
	return data
}

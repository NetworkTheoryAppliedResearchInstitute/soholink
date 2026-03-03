package lbtas

import (
	"encoding/json"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// TransactionFromRow converts a store.ResourceTransactionRow to an lbtas.ResourceTransaction.
// Fields that exist only in the domain model (ProviderRating, UserRating) are left nil
// because the store row does not carry embedded rating objects.
func TransactionFromRow(row *store.ResourceTransactionRow) *ResourceTransaction {
	if row == nil {
		return nil
	}
	tx := &ResourceTransaction{
		TransactionID:   row.TransactionID,
		UserDID:         row.UserDID,
		ProviderDID:     row.ProviderDID,
		ResourceType:    row.ResourceType,
		ResourceID:      row.ResourceID,
		State:           TransactionState(row.State),
		PaymentAmount:   row.PaymentAmount,
		PaymentCurrency: row.PaymentCurrency,
		PaymentEscrowed: row.PaymentEscrowed,
		PaymentProof:    row.PaymentProof,
		ResultsReady:    row.ResultsReady,
		ResultsPath:     row.ResultsPath,
		ResultsKey:      row.ResultsKey,
		RatingDeadline:  row.RatingDeadline,
		DisputeID:       row.DisputeID,
		DisputeReason:   row.DisputeReason,
		UserSignature:   row.UserSignature,
		ProviderSignature: row.ProviderSignature,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}

	// Copy ResultsHash (store uses []byte, domain uses [32]byte)
	if len(row.ResultsHash) == 32 {
		copy(tx.ResultsHash[:], row.ResultsHash)
	}

	// Reconstruct BlockchainAnchor if present
	if row.BlockchainBlock != nil {
		anchor := &BlockchainAnchor{
			BlockHeight: uint64(*row.BlockchainBlock), // #nosec G115 -- blockchain block heights stored as non-negative int64 by DB schema
		}
		if len(row.BlockchainHash) == 32 {
			copy(anchor.DataHash[:], row.BlockchainHash)
		}
		tx.BlockchainAnchor = anchor
	}

	return tx
}

// TransactionToRow converts an lbtas.ResourceTransaction to a store.ResourceTransactionRow.
func TransactionToRow(tx *ResourceTransaction) *store.ResourceTransactionRow {
	if tx == nil {
		return nil
	}
	row := &store.ResourceTransactionRow{
		TransactionID:   tx.TransactionID,
		UserDID:         tx.UserDID,
		ProviderDID:     tx.ProviderDID,
		ResourceType:    tx.ResourceType,
		ResourceID:      tx.ResourceID,
		State:           string(tx.State),
		PaymentAmount:   tx.PaymentAmount,
		PaymentCurrency: tx.PaymentCurrency,
		PaymentEscrowed: tx.PaymentEscrowed,
		PaymentProof:    tx.PaymentProof,
		ResultsReady:    tx.ResultsReady,
		ResultsHash:     tx.ResultsHash[:],
		ResultsPath:     tx.ResultsPath,
		ResultsKey:      tx.ResultsKey,
		RatingDeadline:  tx.RatingDeadline,
		DisputeID:       tx.DisputeID,
		DisputeReason:   tx.DisputeReason,
		UserSignature:   tx.UserSignature,
		ProviderSignature: tx.ProviderSignature,
		CreatedAt:       tx.CreatedAt,
		UpdatedAt:       tx.UpdatedAt,
	}

	if tx.BlockchainAnchor != nil {
		block := int64(tx.BlockchainAnchor.BlockHeight) // #nosec G115 -- block heights fit in int64; DB schema uses int64 for nullable column
		row.BlockchainBlock = &block
		row.BlockchainHash = tx.BlockchainAnchor.DataHash[:]
	}

	return row
}

// ScoreFromRow converts a store.LBTASScoreRow to an lbtas.LBTASScore.
func ScoreFromRow(row *store.LBTASScoreRow) *LBTASScore {
	if row == nil {
		return nil
	}
	score := &LBTASScore{
		DID:                   row.DID,
		OverallScore:          row.OverallScore,
		PaymentReliability:    row.PaymentReliability,
		ExecutionQuality:      row.ExecutionQuality,
		Communication:         row.Communication,
		ResourceUsage:         row.ResourceUsage,
		TotalTransactions:     row.TotalTransactions,
		CompletedTransactions: row.CompletedTransactions,
		DisputedTransactions:  row.DisputedTransactions,
		UpdatedAt:             row.UpdatedAt,
	}

	// Parse ScoreHistory from JSON
	if row.ScoreHistoryJSON != "" {
		_ = json.Unmarshal([]byte(row.ScoreHistoryJSON), &score.ScoreHistory)
	}

	// Convert LastAnchorBlock
	if row.LastAnchorBlock != nil {
		score.LastAnchorBlock = uint64(*row.LastAnchorBlock) // #nosec G115 -- anchor block heights stored as non-negative int64 by DB schema
	}

	// Copy LastAnchorHash
	if len(row.LastAnchorHash) == 32 {
		copy(score.LastAnchorHash[:], row.LastAnchorHash)
	}

	return score
}

// ScoreToRow converts an lbtas.LBTASScore to a store.LBTASScoreRow.
func ScoreToRow(score *LBTASScore) *store.LBTASScoreRow {
	if score == nil {
		return nil
	}
	row := &store.LBTASScoreRow{
		DID:                   score.DID,
		OverallScore:          score.OverallScore,
		PaymentReliability:    score.PaymentReliability,
		ExecutionQuality:      score.ExecutionQuality,
		Communication:         score.Communication,
		ResourceUsage:         score.ResourceUsage,
		TotalTransactions:     score.TotalTransactions,
		CompletedTransactions: score.CompletedTransactions,
		DisputedTransactions:  score.DisputedTransactions,
		UpdatedAt:             time.Now(),
	}

	// Serialize ScoreHistory to JSON
	if len(score.ScoreHistory) > 0 {
		data, err := json.Marshal(score.ScoreHistory)
		if err == nil {
			row.ScoreHistoryJSON = string(data)
		}
	}

	// Convert LastAnchorBlock
	if score.LastAnchorBlock > 0 {
		block := int64(score.LastAnchorBlock) // #nosec G115 -- anchor block heights fit in int64; DB schema uses int64 for nullable column
		row.LastAnchorBlock = &block
	}

	// Copy LastAnchorHash
	row.LastAnchorHash = score.LastAnchorHash[:]

	return row
}

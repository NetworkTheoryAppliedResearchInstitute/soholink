package lbtas

import (
	"time"
)

// LBTASScore holds the aggregated reputation score for a DID.
type LBTASScore struct {
	DID string

	// Overall reputation (0-100)
	OverallScore int

	// Category breakdowns (0-5 each)
	PaymentReliability float64
	ExecutionQuality   float64
	Communication      float64
	ResourceUsage      float64

	// Transaction counts
	TotalTransactions     int
	CompletedTransactions int
	DisputedTransactions  int

	// Score history (for trend analysis)
	ScoreHistory []ScoreSnapshot

	// Last updated
	UpdatedAt time.Time

	// Blockchain anchor
	LastAnchorBlock uint64
	LastAnchorHash  [32]byte
}

// ScoreSnapshot records a score at a point in time.
type ScoreSnapshot struct {
	Score     int
	Timestamp time.Time
}

// CalculateWeight returns the weight for a new rating based on total transactions.
// Early transactions have more impact.
func CalculateWeight(totalTransactions int) float64 {
	if totalTransactions < 10 {
		return 0.3 // 30% weight for first 10
	} else if totalTransactions < 50 {
		return 0.1 // 10% weight for 10-50
	}
	return 0.05 // 5% weight thereafter
}

// WeightedAverage computes a weighted rolling average.
func WeightedAverage(oldValue, newValue float64, weight float64) float64 {
	return oldValue*(1-weight) + newValue*weight
}

// CalculateOverallScore computes the 0-100 overall score from category scores
// and dispute ratio.
func CalculateOverallScore(score *LBTASScore) int {
	// Weighted combination of category scores
	weighted := (score.PaymentReliability*0.3 +
		score.ExecutionQuality*0.3 +
		score.Communication*0.2 +
		score.ResourceUsage*0.2) * 20 // Scale to 0-100

	// Apply penalties for disputes
	disputeRatio := 0.0
	if score.TotalTransactions > 0 {
		disputeRatio = float64(score.DisputedTransactions) / float64(score.TotalTransactions)
	}
	penalty := disputeRatio * 20 // Up to -20 points

	overall := int(weighted - penalty)

	// Clamp to 0-100
	if overall < 0 {
		overall = 0
	} else if overall > 100 {
		overall = 100
	}

	return overall
}

// UpdateScoreFromRating updates an LBTAS score with a new rating.
func UpdateScoreFromRating(score *LBTASScore, rating LBTASRating) {
	weight := CalculateWeight(score.TotalTransactions)

	switch rating.Category {
	case "payment_reliability":
		score.PaymentReliability = WeightedAverage(score.PaymentReliability, float64(rating.Score), weight)
	case "resource_usage":
		score.ResourceUsage = WeightedAverage(score.ResourceUsage, float64(rating.Score), weight)
	case "execution_quality":
		score.ExecutionQuality = WeightedAverage(score.ExecutionQuality, float64(rating.Score), weight)
	case "communication":
		score.Communication = WeightedAverage(score.Communication, float64(rating.Score), weight)
	case "auto_resolved":
		// Auto-resolved ratings apply evenly across all categories
		score.PaymentReliability = WeightedAverage(score.PaymentReliability, float64(rating.Score), weight*0.5)
		score.ExecutionQuality = WeightedAverage(score.ExecutionQuality, float64(rating.Score), weight*0.5)
		score.Communication = WeightedAverage(score.Communication, float64(rating.Score), weight*0.5)
		score.ResourceUsage = WeightedAverage(score.ResourceUsage, float64(rating.Score), weight*0.5)
	}

	score.OverallScore = CalculateOverallScore(score)
	score.TotalTransactions++
	score.CompletedTransactions++

	// Add snapshot to history
	score.ScoreHistory = append(score.ScoreHistory, ScoreSnapshot{
		Score:     score.OverallScore,
		Timestamp: time.Now(),
	})

	// Keep only last 100 snapshots
	if len(score.ScoreHistory) > 100 {
		score.ScoreHistory = score.ScoreHistory[1:]
	}

	score.UpdatedAt = time.Now()
}

// ApplyPenalty reduces a score by the given number of points.
func ApplyPenalty(score *LBTASScore, points int) {
	// Reduce overall score directly
	score.OverallScore -= points
	if score.OverallScore < 0 {
		score.OverallScore = 0
	}
	score.UpdatedAt = time.Now()
}

package central

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// CenterRatingManager handles ratings of Central SOHO nodes themselves.
// After a dispute is resolved, both the user and provider rate how the
// central SOHO handled the dispute. If the center's score drops to -1,
// the center is suspended.
type CenterRatingManager struct {
	store    *store.Store
	notifier *Notifier
}

// CenterRating is a single rating of a center's dispute handling.
type CenterRating struct {
	RatingID  string
	DisputeID string
	CenterDID string
	RaterDID  string
	RaterRole string // "user" or "provider"
	Score     int    // 0-5
	Feedback  string
	CreatedAt time.Time
	Signature []byte
}

// NewCenterRatingManager creates a new center rating manager.
func NewCenterRatingManager(s *store.Store, notifier *Notifier) *CenterRatingManager {
	return &CenterRatingManager{store: s, notifier: notifier}
}

// RecordCenterRating records a rating of the central SOHO and updates
// the center's aggregate score. If the score drops to 0 or below,
// the center is suspended and notifications are sent.
func (m *CenterRatingManager) RecordCenterRating(ctx context.Context, rating CenterRating) error {
	// Store the individual rating
	row := &store.CenterRatingRow{
		RatingID:  rating.RatingID,
		DisputeID: rating.DisputeID,
		CenterDID: rating.CenterDID,
		RaterDID:  rating.RaterDID,
		RaterRole: rating.RaterRole,
		Score:     rating.Score,
		Feedback:  rating.Feedback,
		CreatedAt: rating.CreatedAt,
		Signature: rating.Signature,
	}
	if err := m.store.CreateCenterRating(ctx, row); err != nil {
		return fmt.Errorf("failed to create center rating: %w", err)
	}

	// Get or create center score
	centerScore, err := m.store.GetCenterScore(ctx, rating.CenterDID)
	if err != nil {
		return fmt.Errorf("failed to get center score: %w", err)
	}
	if centerScore == nil {
		centerScore = &store.CenterScoreRow{
			CenterDID:    rating.CenterDID,
			OverallScore: 50,
			Active:       true,
			UpdatedAt:    time.Now(),
		}
	}

	// Update score with weighted average
	weight := calculateCenterWeight(centerScore.TotalRatings)
	newVal := float64(rating.Score) / 5.0 * 100.0 // Normalise 0-5 → 0-100

	oldOverall := float64(centerScore.OverallScore)
	centerScore.OverallScore = int(oldOverall*(1-weight) + newVal*weight)
	centerScore.TotalRatings++
	centerScore.AverageScore = (centerScore.AverageScore*float64(centerScore.TotalRatings-1) + float64(rating.Score)) / float64(centerScore.TotalRatings)
	centerScore.TotalDisputes++ // Each rating corresponds to a dispute
	centerScore.UpdatedAt = time.Now()

	// Check critical threshold
	if centerScore.OverallScore <= 0 {
		centerScore.Active = false
		now := time.Now()
		centerScore.SuspendedAt = &now

		log.Printf("[center-rating] CRITICAL: Center %s suspended (score=%d)", rating.CenterDID, centerScore.OverallScore)

		if m.notifier != nil {
			m.notifier.Send(Notification{
				Type:     "center_suspended",
				Severity: "critical",
				Message:  fmt.Sprintf("Center %s has been suspended due to low rating score (%d)", rating.CenterDID, centerScore.OverallScore),
			})
		}

		// Notify connected thin clients about migration
		m.notifyClientsOfSuspension(ctx, rating.CenterDID)
	}

	return m.store.UpsertCenterScore(ctx, centerScore)
}

// GetCenterScore returns the current score for a central SOHO.
func (m *CenterRatingManager) GetCenterScore(ctx context.Context, centerDID string) (*store.CenterScoreRow, error) {
	return m.store.GetCenterScore(ctx, centerDID)
}

// notifyClientsOfSuspension alerts all thin clients connected to a suspended center.
func (m *CenterRatingManager) notifyClientsOfSuspension(ctx context.Context, centerDID string) {
	tenants, err := m.store.GetTenantsByCenter(ctx, centerDID)
	if err != nil {
		log.Printf("[center-rating] failed to get tenants for center %s: %v", centerDID, err)
		return
	}

	for _, tenant := range tenants {
		if m.notifier != nil {
			m.notifier.Send(Notification{
				Type:    "center_suspended",
				Message: fmt.Sprintf("Central SOHO %s has been suspended. Please migrate to another center.", centerDID),
				AlertID: tenant.TenantID,
			})
		}
	}
}

// calculateCenterWeight returns a weight factor for incremental score updates.
func calculateCenterWeight(totalRatings int) float64 {
	if totalRatings < 5 {
		return 0.3
	}
	if totalRatings < 20 {
		return 0.2
	}
	return 0.1
}

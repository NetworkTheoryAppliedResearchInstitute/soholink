package lbtas

import (
	"errors"
	"fmt"
)

// Valid rating categories.
var validCategories = map[string]bool{
	"payment_reliability": true,
	"resource_usage":      true,
	"job_quality":         true,
	"communication":       true,
	"execution_quality":   true,
	"performance":         true,
	"reliability":         true,
	"print_quality":       true,
	"material_accuracy":   true,
	"timeliness":          true,
	"connection_quality":  true,
	"privacy":             true,
	"transparency":        true,
	"data_legality":       true,
	"quota_respect":       true,
	"security":            true,
	"bandwidth_respect":   true,
	"content_legality":    true,
	"job_legality":        true,
	"material_used":       true,
	"auto_resolved":       true,
}

// ProviderRatingRequest is submitted when a provider rates a user.
type ProviderRatingRequest struct {
	TransactionID string
	ProviderDID   string
	Rating        LBTASRating
}

// UserRatingRequest is submitted when a user rates a provider.
type UserRatingRequest struct {
	TransactionID string
	UserDID       string
	Rating        LBTASRating
}

// DisputeRequest is filed when rating divergence is too large.
type DisputeRequest struct {
	TransactionID string
	FilerDID      string
	Reason        string
	Evidence      []byte
}

// Dispute represents a contested rating outcome.
type Dispute struct {
	DisputeID      string
	TransactionID  string
	FilerDID       string
	RespondentDID  string
	Reason         string
	Evidence       []byte
	Status         string // "pending", "resolved_user", "resolved_provider", "partial"
	FiledAt        string
	VotingDeadline string
}

// Sentinel errors for rating operations.
var (
	ErrInvalidCredential   = errors.New("invalid credential")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidState        = errors.New("invalid transaction state for this operation")
	ErrDisputeNotJustified = errors.New("rating divergence too small to justify dispute")
	ErrPaymentQueueFull    = errors.New("offline payment queue is full")
)

// ValidateRating checks that a rating has valid score, category, and feedback length.
func ValidateRating(rating LBTASRating) error {
	if rating.Score < 0 || rating.Score > 5 {
		return fmt.Errorf("invalid score: must be 0-5, got %d", rating.Score)
	}

	if len(rating.Feedback) > 500 {
		return fmt.Errorf("feedback too long: max 500 characters, got %d", len(rating.Feedback))
	}

	if !validCategories[rating.Category] {
		return fmt.Errorf("invalid category: %s", rating.Category)
	}

	return nil
}

// ComputeUserRating captures how a provider rates a compute user.
type ComputeUserRating struct {
	PaymentReliability int // 0-5
	ResourceUsage      int // 0-5
	JobQuality         int // 0-5
	Communication      int // 0-5
	OverallScore       int // Weighted average
}

// ComputeProviderRating captures how a user rates a compute provider.
type ComputeProviderRating struct {
	ExecutionQuality int // 0-5
	Performance      int // 0-5
	Reliability      int // 0-5
	Communication    int // 0-5
	OverallScore     int // Weighted average
}

// StorageUserRating captures how a provider rates a storage user.
type StorageUserRating struct {
	PaymentReliability int // 0-5
	DataLegality       int // 0-5
	QuotaRespect       int // 0-5
	OverallScore       int
}

// StorageProviderRating captures how a user rates a storage provider.
type StorageProviderRating struct {
	Reliability  int // 0-5
	Performance  int // 0-5
	Security     int // 0-5
	OverallScore int
}

// PrintUserRating captures how a provider rates a print user.
type PrintUserRating struct {
	PaymentReliability int // 0-5
	JobLegality        int // 0-5
	MaterialAccuracy   int // 0-5
	OverallScore       int
}

// PrintProviderRating captures how a user rates a print provider.
type PrintProviderRating struct {
	PrintQuality int // 0-5
	MaterialUsed int // 0-5
	Timeliness   int // 0-5
	OverallScore int
}

// PortalUserRating captures how a provider rates a portal user.
type PortalUserRating struct {
	PaymentReliability int // 0-5
	BandwidthRespect   int // 0-5
	ContentLegality    int // 0-5
	OverallScore       int
}

// PortalProviderRating captures how a user rates a portal provider.
type PortalProviderRating struct {
	ConnectionQuality int // 0-5
	Privacy           int // 0-5
	Transparency      int // 0-5
	OverallScore      int
}

// abs returns the absolute value of an int.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// CanFileDispute returns true if rating divergence justifies a dispute.
func CanFileDispute(tx *ResourceTransaction) bool {
	if tx.ProviderRating == nil || tx.UserRating == nil {
		return false
	}
	divergence := abs(tx.ProviderRating.Score - tx.UserRating.Score)
	return divergence >= 3
}

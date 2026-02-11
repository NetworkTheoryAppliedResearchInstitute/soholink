package payment

import (
	"context"
	"time"
)

// PaymentProcessor is the pluggable interface for payment backends.
type PaymentProcessor interface {
	// Name returns the processor identifier (e.g. "stripe", "lightning").
	Name() string
	// IsOnline reports whether the processor can currently process payments.
	IsOnline(ctx context.Context) bool

	// CreateCharge initiates a payment.
	CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)
	// ConfirmCharge confirms a pending charge.
	ConfirmCharge(ctx context.Context, chargeID string) error
	// RefundCharge refunds a completed charge.
	RefundCharge(ctx context.Context, chargeID string, reason string) error

	// GetChargeStatus queries the status of a charge.
	GetChargeStatus(ctx context.Context, chargeID string) (*ChargeStatus, error)
	// ListCharges lists charges matching a filter.
	ListCharges(ctx context.Context, filter ChargeFilter) ([]ChargeStatus, error)
}

// ChargeRequest describes a payment to be created.
type ChargeRequest struct {
	Amount         int64
	Currency       string
	UserDID        string
	ProviderDID    string
	ResourceType   string
	UsageRecordID  string
	Metadata       map[string]string
	IdempotencyKey string
}

// ChargeResult is returned after a charge is created.
type ChargeResult struct {
	ChargeID     string
	Status       string // "pending", "succeeded", "failed"
	Amount       int64
	ProcessorFee int64
	NetAmount    int64
	SettledAt    *time.Time
}

// ChargeStatus describes the current state of a charge.
type ChargeStatus struct {
	ChargeID  string
	Status    string
	Amount    int64
	CreatedAt time.Time
	SettledAt *time.Time
}

// ChargeFilter specifies criteria for listing charges.
type ChargeFilter struct {
	UserDID     string
	ProviderDID string
	Status      string
	Limit       int
	Offset      int
}

// PaymentEvent represents a payment that needs processing or settlement.
type PaymentEvent struct {
	ID           string
	ChargeID     string
	UserDID      string
	ProviderDID  string
	Amount       int64
	Currency     string
	ResourceType string
	Attempts     int
	NextRetry    time.Time
	CreatedAt    time.Time
}

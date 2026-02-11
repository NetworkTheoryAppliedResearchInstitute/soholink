package lbtas

import (
	"sort"
	"time"
)

// ResourceAnnouncement advertises a resource available from a provider node.
type ResourceAnnouncement struct {
	ProviderDID  string
	ResourceType string                 // "compute", "storage", "print", "internet"
	Capabilities map[string]interface{} // Type-specific capabilities
	Pricing      []PricingTier
	Availability AvailabilitySchedule
	MaxUsers     int
	Reputation   int // LBTAS overall score
	Signature    []byte
	AnnouncedAt  time.Time
	ExpiresAt    time.Time
}

// PricingTier defines a rate structure for resource usage.
type PricingTier struct {
	Unit        string  // "cpu_hour", "gb_month", "page", "mbps_hour"
	RateCents   int64   // Price per unit
	Currency    string  // "USD", "FED", "BTC"
	MinQuantity float64
	MaxQuantity float64
}

// AvailabilitySchedule defines when a resource is available.
type AvailabilitySchedule struct {
	Timezone           string
	AlwaysOn           bool
	Schedule           []TimeWindow
	MaintenanceWindows []TimeWindow
}

// TimeWindow represents a start/end time range.
type TimeWindow struct {
	Start time.Time
	End   time.Time
}

// ResourceQuery specifies criteria for finding resources.
type ResourceQuery struct {
	ResourceType     string
	MinCPU           int
	MinStorage       int64
	MaxPriceCents    int64
	Currency         string
	MinProviderScore int
}

// ProviderMatch is a resource provider matching a query.
type ProviderMatch struct {
	ProviderDID       string
	Score             int
	PriceCentsPerUnit int64
	ResourceType      string
	Availability      AvailabilitySchedule
}

// RankProviders sorts provider matches by a composite score of reputation and price.
// Higher reputation and lower price rank higher.
func RankProviders(matches []ProviderMatch) {
	sort.Slice(matches, func(i, j int) bool {
		// Weighted ranking: 60% reputation, 40% price (inverted)
		scoreI := float64(matches[i].Score)*0.6 - float64(matches[i].PriceCentsPerUnit)*0.4
		scoreJ := float64(matches[j].Score)*0.6 - float64(matches[j].PriceCentsPerUnit)*0.4
		return scoreI > scoreJ
	})
}

// FederatedSession tracks an active resource usage session across nodes.
type FederatedSession struct {
	SessionID    string
	UserDID      string
	ProviderDID  string
	ResourceType string
	ResourceID   string
	StartTime    time.Time
	ExpiresAt    time.Time
	Status       string // "active", "suspended", "completed", "failed"

	LastUpdate time.Time
	UpdatedBy  string

	PaymentProof []byte
	UsageSoFar   float64

	Signature []byte
}

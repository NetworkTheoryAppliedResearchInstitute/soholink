package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func createTestTenant(t *testing.T, s *Store, ctx context.Context, tenantID string) {
	t.Helper()
	tenant := &TenantRow{TenantID: tenantID, Name: "Test Tenant", Status: "active"}
	if err := s.CreateTenant(ctx, tenant); err != nil {
		t.Fatalf("failed to create test tenant %s: %v", tenantID, err)
	}
}

func TestGetTotalRevenue(t *testing.T) {
	s, err := NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	createTestTenant(t, s, ctx, "tenant1")

	// Initially should be zero
	total, err := s.GetTotalRevenue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("Expected total revenue 0, got %d", total)
	}

	// Add some revenue
	rev1 := &CentralRevenueRow{
		RevenueID:      "rev1",
		TransactionID:  "tx1",
		TenantID:       "tenant1",
		TotalAmount:    10000, // $100
		CentralFee:     100,   // $1
		ProducerPayout: 9700,  // $97
		ProcessorFee:   200,   // $2
		Currency:       "USD",
		CreatedAt:      time.Now(),
	}

	rev2 := &CentralRevenueRow{
		RevenueID:      "rev2",
		TransactionID:  "tx2",
		TenantID:       "tenant1",
		TotalAmount:    5000, // $50
		CentralFee:     50,   // $0.50
		ProducerPayout: 4850, // $48.50
		ProcessorFee:   100,  // $1
		Currency:       "USD",
		CreatedAt:      time.Now(),
	}

	if err := s.CreateCentralRevenue(ctx, rev1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCentralRevenue(ctx, rev2); err != nil {
		t.Fatal(err)
	}

	// Total should be sum of both
	total, err = s.GetTotalRevenue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expectedTotal := int64(15000) // $100 + $50
	if total != expectedTotal {
		t.Errorf("Expected total revenue %d, got %d", expectedTotal, total)
	}
}

func TestGetRevenueSince(t *testing.T) {
	s, err := NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	createTestTenant(t, s, ctx, "tenant1")

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	// Add revenue from different times
	rev1 := &CentralRevenueRow{
		RevenueID:      "rev1",
		TransactionID:  "tx1",
		TenantID:       "tenant1",
		TotalAmount:    10000,
		CentralFee:     100,
		ProducerPayout: 9700,
		ProcessorFee:   200,
		Currency:       "USD",
		CreatedAt:      lastWeek,
	}

	rev2 := &CentralRevenueRow{
		RevenueID:      "rev2",
		TransactionID:  "tx2",
		TenantID:       "tenant1",
		TotalAmount:    5000,
		CentralFee:     50,
		ProducerPayout: 4850,
		ProcessorFee:   100,
		Currency:       "USD",
		CreatedAt:      yesterday,
	}

	rev3 := &CentralRevenueRow{
		RevenueID:      "rev3",
		TransactionID:  "tx3",
		TenantID:       "tenant1",
		TotalAmount:    3000,
		CentralFee:     30,
		ProducerPayout: 2910,
		ProcessorFee:   60,
		Currency:       "USD",
		CreatedAt:      now,
	}

	if err := s.CreateCentralRevenue(ctx, rev1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCentralRevenue(ctx, rev2); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCentralRevenue(ctx, rev3); err != nil {
		t.Fatal(err)
	}

	// Revenue since yesterday should include rev2 and rev3
	total, err := s.GetRevenueSince(ctx, yesterday.Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	expectedTotal := int64(8000) // rev2 + rev3
	if total != expectedTotal {
		t.Errorf("Expected revenue since yesterday %d, got %d", expectedTotal, total)
	}

	// Revenue since last week should include all
	total, err = s.GetRevenueSince(ctx, lastWeek.Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	expectedTotal = int64(18000) // all three
	if total != expectedTotal {
		t.Errorf("Expected revenue since last week %d, got %d", expectedTotal, total)
	}
}

func TestGetPendingPayout(t *testing.T) {
	s, err := NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	createTestTenant(t, s, ctx, "tenant1")

	// Add settled and unsettled revenue
	settledTime := time.Now()
	rev1 := &CentralRevenueRow{
		RevenueID:      "rev1",
		TransactionID:  "tx1",
		TenantID:       "tenant1",
		TotalAmount:    10000,
		CentralFee:     100,
		ProducerPayout: 9700, // Should NOT be in pending
		ProcessorFee:   200,
		Currency:       "USD",
		SettledAt:      &settledTime, // Already settled
		CreatedAt:      time.Now(),
	}

	rev2 := &CentralRevenueRow{
		RevenueID:      "rev2",
		TransactionID:  "tx2",
		TenantID:       "tenant1",
		TotalAmount:    5000,
		CentralFee:     50,
		ProducerPayout: 4850, // Should be in pending
		ProcessorFee:   100,
		Currency:       "USD",
		SettledAt:      nil, // Not settled yet
		CreatedAt:      time.Now(),
	}

	rev3 := &CentralRevenueRow{
		RevenueID:      "rev3",
		TransactionID:  "tx3",
		TenantID:       "tenant1",
		TotalAmount:    3000,
		CentralFee:     30,
		ProducerPayout: 2910, // Should be in pending
		ProcessorFee:   60,
		Currency:       "USD",
		SettledAt:      nil, // Not settled yet
		CreatedAt:      time.Now(),
	}

	if err := s.CreateCentralRevenue(ctx, rev1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCentralRevenue(ctx, rev2); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCentralRevenue(ctx, rev3); err != nil {
		t.Fatal(err)
	}

	// Pending payout should be sum of unsettled producer payouts
	pending, err := s.GetPendingPayout(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expectedPending := int64(7760) // rev2 + rev3 producer payouts
	if pending != expectedPending {
		t.Errorf("Expected pending payout %d, got %d", expectedPending, pending)
	}
}

func TestGetRecentRevenue(t *testing.T) {
	s, err := NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	createTestTenant(t, s, ctx, "tenant1")

	// Add multiple revenue entries
	for i := 0; i < 5; i++ {
		rev := &CentralRevenueRow{
			RevenueID:      fmt.Sprintf("rev%d", i),
			TransactionID:  fmt.Sprintf("tx%d", i),
			TenantID:       "tenant1",
			TotalAmount:    int64(1000 * (i + 1)),
			CentralFee:     int64(10 * (i + 1)),
			ProducerPayout: int64(970 * (i + 1)),
			ProcessorFee:   int64(20 * (i + 1)),
			Currency:       "USD",
			CreatedAt:      time.Now().Add(time.Duration(-i) * time.Hour),
		}
		if err := s.CreateCentralRevenue(ctx, rev); err != nil {
			t.Fatal(err)
		}
	}

	// Get recent 3
	recent, err := s.GetRecentRevenue(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(recent) != 3 {
		t.Errorf("Expected 3 recent revenue entries, got %d", len(recent))
	}

	// Should be ordered by created_at DESC (most recent first)
	if len(recent) > 0 && recent[0].RevenueID != "rev0" {
		t.Errorf("Expected most recent to be rev0, got %s", recent[0].RevenueID)
	}
}

func TestGetActiveRentals(t *testing.T) {
	s, err := NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	// Create resource transactions with different states
	txs := []struct {
		id    string
		state string
	}{
		{"tx1", "initiated"},  // Active
		{"tx2", "executing"},  // Active
		{"tx3", "completed"},  // Not active
		{"tx4", "failed"},     // Not active
		{"tx5", "initiated"},  // Active
	}

	for _, tx := range txs {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO resource_transactions (transaction_id, user_did, provider_did, resource_type, resource_id, state, payment_amount, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			tx.id, "did:soho:user1", "did:soho:provider1", "compute", "resource1", tx.state, 5000, time.Now())
		if err != nil {
			t.Fatal(err)
		}
	}

	// Get active rentals
	active, err := s.GetActiveRentals(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Should have 3 active (initiated + executing)
	if len(active) != 3 {
		t.Errorf("Expected 3 active rentals, got %d", len(active))
	}

	// Verify they are the correct ones
	activeIDs := make(map[string]bool)
	for _, rental := range active {
		activeIDs[rental.TransactionID] = true
	}

	if !activeIDs["tx1"] || !activeIDs["tx2"] || !activeIDs["tx5"] {
		t.Error("Expected tx1, tx2, tx5 to be active")
	}
}

func TestGetRecentAlerts(t *testing.T) {
	s, err := NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	// Create rating alerts
	for i := 0; i < 5; i++ {
		alert := &RatingAlertRow{
			AlertID:       fmt.Sprintf("alert%d", i),
			TransactionID: fmt.Sprintf("tx%d", i),
			UserDID:       "did:soho:user1",
			ProviderDID:   "did:soho:provider1",
			CenterDID:     "did:soho:center1",
			AlertType:     "catastrophic_rating",
			Severity:      "high",
			Status:        "pending",
			CreatedAt:     time.Now().Add(time.Duration(-i) * time.Hour),
		}
		if err := s.CreateRatingAlert(ctx, alert); err != nil {
			t.Fatal(err)
		}
	}

	// Get recent 3 alerts
	recent, err := s.GetRecentAlerts(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(recent) != 3 {
		t.Errorf("Expected 3 recent alerts, got %d", len(recent))
	}

	// Should be ordered by created_at DESC (most recent first)
	if len(recent) > 0 && recent[0].AlertID != "alert0" {
		t.Errorf("Expected most recent to be alert0, got %s", recent[0].AlertID)
	}
}

func TestGetRevenueByType(t *testing.T) {
	s, err := NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	createTestTenant(t, s, ctx, "tenant1")

	// Create resource transactions
	txs := []struct {
		id           string
		resourceType string
		amount       int64
	}{
		{"tx1", "compute", 10000},
		{"tx2", "storage", 5000},
		{"tx3", "compute", 8000},
		{"tx4", "network", 3000},
		{"tx5", "compute", 12000},
	}

	for _, tx := range txs {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO resource_transactions (transaction_id, user_did, provider_did, resource_type, resource_id, state, payment_amount, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			tx.id, "did:soho:user1", "did:soho:provider1", tx.resourceType, "resource1", "completed", tx.amount, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		// Add corresponding revenue
		rev := &CentralRevenueRow{
			RevenueID:      "rev_" + tx.id,
			TransactionID:  tx.id,
			TenantID:       "tenant1",
			TotalAmount:    tx.amount,
			CentralFee:     tx.amount / 100,
			ProducerPayout: tx.amount * 97 / 100,
			ProcessorFee:   tx.amount * 2 / 100,
			Currency:       "USD",
			CreatedAt:      time.Now(),
		}
		if err := s.CreateCentralRevenue(ctx, rev); err != nil {
			t.Fatal(err)
		}
	}

	// Get revenue by type
	computeRevenue, err := s.GetRevenueByType(ctx, "compute")
	if err != nil {
		t.Fatal(err)
	}

	expectedCompute := int64(30000) // tx1 + tx3 + tx5
	if computeRevenue != expectedCompute {
		t.Errorf("Expected compute revenue %d, got %d", expectedCompute, computeRevenue)
	}

	storageRevenue, err := s.GetRevenueByType(ctx, "storage")
	if err != nil {
		t.Fatal(err)
	}

	expectedStorage := int64(5000) // tx2
	if storageRevenue != expectedStorage {
		t.Errorf("Expected storage revenue %d, got %d", expectedStorage, storageRevenue)
	}
}

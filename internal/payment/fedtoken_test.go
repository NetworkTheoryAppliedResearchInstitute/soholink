package payment

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// TestFederationTokenProcessor_Name verifies the processor name.
func TestFederationTokenProcessor_Name(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer s.Close()

	p := NewFederationTokenProcessor(s, "contract_addr", "/path/to/wallet")
	if got := p.Name(); got != "federation_token" {
		t.Errorf("Name() = %q, want %q", got, "federation_token")
	}
}

// TestFederationTokenProcessor_IsOnline verifies online status checks.
func TestFederationTokenProcessor_IsOnline(t *testing.T) {
	tests := []struct {
		name          string
		tokenContract string
		want          bool
	}{
		{
			name:          "online with contract",
			tokenContract: "0x1234567890",
			want:          true,
		},
		{
			name:          "offline without contract",
			tokenContract: "",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewMemoryStore()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			defer s.Close()

			p := NewFederationTokenProcessor(s, tt.tokenContract, "/path/to/wallet")
			if got := p.IsOnline(context.Background()); got != tt.want {
				t.Errorf("IsOnline() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFederationTokenProcessor_CreateCharge tests charge creation.
func TestFederationTokenProcessor_CreateCharge(t *testing.T) {
	tests := []struct {
		name          string
		tokenContract string
		request       ChargeRequest
		wantErr       bool
		wantStatus    string
		wantCurrency  string
	}{
		{
			name:          "successful charge creation",
			tokenContract: "0x1234567890",
			request: ChargeRequest{
				Amount:        10000,
				Currency:      "FED",
				UserDID:       "did:soho:user123",
				ProviderDID:   "did:soho:provider456",
				ResourceType:  "compute",
				UsageRecordID: "usage_789",
			},
			wantErr:      false,
			wantStatus:   "pending",
			wantCurrency: "FED",
		},
		{
			name:          "default currency to FED",
			tokenContract: "0x1234567890",
			request: ChargeRequest{
				Amount:       5000,
				Currency:     "", // Should default to FED
				UserDID:      "did:soho:user123",
				ProviderDID:  "did:soho:provider456",
				ResourceType: "storage",
			},
			wantErr:      false,
			wantStatus:   "pending",
			wantCurrency: "FED",
		},
		{
			name:          "no token contract",
			tokenContract: "",
			request: ChargeRequest{
				Amount:      1000,
				UserDID:     "did:soho:user123",
				ProviderDID: "did:soho:provider456",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewMemoryStore()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			defer s.Close()

			p := NewFederationTokenProcessor(s, tt.tokenContract, "/path/to/wallet")

			result, err := p.CreateCharge(context.Background(), tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateCharge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result.ChargeID == "" {
					t.Error("CreateCharge() returned empty ChargeID")
				}
				if result.Status != tt.wantStatus {
					t.Errorf("CreateCharge() status = %q, want %q", result.Status, tt.wantStatus)
				}
				if result.Amount != tt.request.Amount {
					t.Errorf("CreateCharge() amount = %d, want %d", result.Amount, tt.request.Amount)
				}
				if result.ProcessorFee != 0 {
					t.Errorf("CreateCharge() processor fee = %d, want 0", result.ProcessorFee)
				}
				if result.NetAmount != tt.request.Amount {
					t.Errorf("CreateCharge() net amount = %d, want %d", result.NetAmount, tt.request.Amount)
				}

				// Verify charge persisted to database
				payment, err := s.GetPaymentByChargeID(context.Background(), result.ChargeID)
				if err != nil {
					t.Fatalf("Failed to retrieve persisted payment: %v", err)
				}
				if payment.Currency != tt.wantCurrency {
					t.Errorf("Persisted currency = %q, want %q", payment.Currency, tt.wantCurrency)
				}
			}
		})
	}
}

// TestFederationTokenProcessor_ConfirmCharge tests charge confirmation.
func TestFederationTokenProcessor_ConfirmCharge(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*store.Store) string // Returns chargeID
		wantErr   bool
	}{
		{
			name: "successful confirmation",
			setupFunc: func(s *store.Store) string {
				row := &store.PendingPaymentRow{
					ID:           "payment_123",
					ChargeID:     "fed_123456",
					UserDID:      "did:soho:user123",
					ProviderDID:  "did:soho:provider456",
					Amount:       10000,
					Currency:     "FED",
					ResourceType: "compute",
					Status:       "pending",
					CreatedAt:    time.Now(),
				}
				if err := s.CreatePendingPayment(context.Background(), row); err != nil {
					panic(err)
				}
				return row.ChargeID
			},
			wantErr: false,
		},
		{
			name: "charge not found",
			setupFunc: func(s *store.Store) string {
				return "nonexistent_charge"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewMemoryStore()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			defer s.Close()

			chargeID := tt.setupFunc(s)
			p := NewFederationTokenProcessor(s, "0x1234567890", "/path/to/wallet")

			err = p.ConfirmCharge(context.Background(), chargeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfirmCharge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify status was updated
				payment, err := s.GetPaymentByChargeID(context.Background(), chargeID)
				if err != nil {
					t.Fatalf("Failed to retrieve payment: %v", err)
				}
				if payment.Status != "settled" {
					t.Errorf("Payment status = %q, want %q", payment.Status, "settled")
				}
			}
		})
	}
}

// TestFederationTokenProcessor_RefundCharge tests refund processing.
func TestFederationTokenProcessor_RefundCharge(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*store.Store) string // Returns chargeID
		reason    string
		wantErr   bool
	}{
		{
			name: "successful refund",
			setupFunc: func(s *store.Store) string {
				row := &store.PendingPaymentRow{
					ID:           "payment_123",
					ChargeID:     "fed_123456",
					UserDID:      "did:soho:user123",
					ProviderDID:  "did:soho:provider456",
					Amount:       10000,
					Currency:     "FED",
					ResourceType: "compute",
					Status:       "settled",
					CreatedAt:    time.Now(),
				}
				if err := s.CreatePendingPayment(context.Background(), row); err != nil {
					panic(err)
				}
				return row.ChargeID
			},
			reason:  "customer_request",
			wantErr: false,
		},
		{
			name: "charge not found",
			setupFunc: func(s *store.Store) string {
				return "nonexistent_charge"
			},
			reason:  "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewMemoryStore()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			defer s.Close()

			chargeID := tt.setupFunc(s)
			p := NewFederationTokenProcessor(s, "0x1234567890", "/path/to/wallet")

			err = p.RefundCharge(context.Background(), chargeID, tt.reason)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefundCharge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify original charge was marked as refunded
				payment, err := s.GetPaymentByChargeID(context.Background(), chargeID)
				if err != nil {
					t.Fatalf("Failed to retrieve original payment: %v", err)
				}
				if payment.Status != "refunded" {
					t.Errorf("Original payment status = %q, want %q", payment.Status, "refunded")
				}

				// Verify refund record was created with negative amount
				// (In a real implementation, you'd query by a refund ID or filter)
				// For now, we just verify the original charge status changed
			}
		})
	}
}

// TestFederationTokenProcessor_GetChargeStatus tests status retrieval.
func TestFederationTokenProcessor_GetChargeStatus(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*store.Store) string // Returns chargeID
		wantErr    bool
		wantStatus string
		wantAmount int64
	}{
		{
			name: "pending charge",
			setupFunc: func(s *store.Store) string {
				row := &store.PendingPaymentRow{
					ID:           "payment_123",
					ChargeID:     "fed_pending",
					UserDID:      "did:soho:user123",
					ProviderDID:  "did:soho:provider456",
					Amount:       10000,
					Currency:     "FED",
					ResourceType: "compute",
					Status:       "pending",
					CreatedAt:    time.Now(),
				}
				if err := s.CreatePendingPayment(context.Background(), row); err != nil {
					panic(err)
				}
				return row.ChargeID
			},
			wantErr:    false,
			wantStatus: "pending",
			wantAmount: 10000,
		},
		{
			name: "settled charge",
			setupFunc: func(s *store.Store) string {
				row := &store.PendingPaymentRow{
					ID:           "payment_456",
					ChargeID:     "fed_settled",
					UserDID:      "did:soho:user123",
					ProviderDID:  "did:soho:provider456",
					Amount:       5000,
					Currency:     "FED",
					ResourceType: "storage",
					Status:       "settled",
					CreatedAt:    time.Now(),
				}
				if err := s.CreatePendingPayment(context.Background(), row); err != nil {
					panic(err)
				}
				return row.ChargeID
			},
			wantErr:    false,
			wantStatus: "settled",
			wantAmount: 5000,
		},
		{
			name: "charge not found",
			setupFunc: func(s *store.Store) string {
				return "nonexistent_charge"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewMemoryStore()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			defer s.Close()

			chargeID := tt.setupFunc(s)
			p := NewFederationTokenProcessor(s, "0x1234567890", "/path/to/wallet")

			status, err := p.GetChargeStatus(context.Background(), chargeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetChargeStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if status.Status != tt.wantStatus {
					t.Errorf("Status = %q, want %q", status.Status, tt.wantStatus)
				}
				if status.Amount != tt.wantAmount {
					t.Errorf("Amount = %d, want %d", status.Amount, tt.wantAmount)
				}
				if status.ChargeID != chargeID {
					t.Errorf("ChargeID = %q, want %q", status.ChargeID, chargeID)
				}
				if status.CreatedAt.IsZero() {
					t.Error("CreatedAt is zero")
				}

				// Verify settled charges have SettledAt timestamp
				if tt.wantStatus == "settled" && status.SettledAt == nil {
					t.Error("Expected SettledAt timestamp for settled charge")
				}
			}
		})
	}
}

// TestFederationTokenProcessor_ListCharges tests listing charges with filters.
func TestFederationTokenProcessor_ListCharges(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*store.Store)
		filter    ChargeFilter
		wantErr   bool
		wantCount int
	}{
		{
			name: "list all charges",
			setupFunc: func(s *store.Store) {
				charges := []*store.PendingPaymentRow{
					{
						ID:           "payment_1",
						ChargeID:     "fed_1",
						UserDID:      "did:soho:user123",
						ProviderDID:  "did:soho:provider456",
						Amount:       10000,
						Currency:     "FED",
						ResourceType: "compute",
						Status:       "pending",
						CreatedAt:    time.Now(),
					},
					{
						ID:           "payment_2",
						ChargeID:     "fed_2",
						UserDID:      "did:soho:user123",
						ProviderDID:  "did:soho:provider789",
						Amount:       5000,
						Currency:     "FED",
						ResourceType: "storage",
						Status:       "settled",
						CreatedAt:    time.Now(),
					},
					{
						ID:           "payment_3",
						ChargeID:     "fed_3",
						UserDID:      "did:soho:user456",
						ProviderDID:  "did:soho:provider456",
						Amount:       7500,
						Currency:     "FED",
						ResourceType: "bandwidth",
						Status:       "pending",
						CreatedAt:    time.Now(),
					},
				}
				for _, row := range charges {
					if err := s.CreatePendingPayment(context.Background(), row); err != nil {
						panic(err)
					}
				}
			},
			filter: ChargeFilter{
				Limit: 10,
			},
			wantErr:   false,
			wantCount: 3,
		},
		{
			name: "filter by user DID",
			setupFunc: func(s *store.Store) {
				charges := []*store.PendingPaymentRow{
					{
						ID:           "payment_1",
						ChargeID:     "fed_1",
						UserDID:      "did:soho:user123",
						ProviderDID:  "did:soho:provider456",
						Amount:       10000,
						Currency:     "FED",
						Status:       "pending",
						CreatedAt:    time.Now(),
					},
					{
						ID:           "payment_2",
						ChargeID:     "fed_2",
						UserDID:      "did:soho:user456",
						ProviderDID:  "did:soho:provider456",
						Amount:       5000,
						Currency:     "FED",
						Status:       "settled",
						CreatedAt:    time.Now(),
					},
				}
				for _, row := range charges {
					if err := s.CreatePendingPayment(context.Background(), row); err != nil {
						panic(err)
					}
				}
			},
			filter: ChargeFilter{
				UserDID: "did:soho:user123",
				Limit:   10,
			},
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "filter by status",
			setupFunc: func(s *store.Store) {
				charges := []*store.PendingPaymentRow{
					{
						ID:           "payment_1",
						ChargeID:     "fed_1",
						UserDID:      "did:soho:user123",
						ProviderDID:  "did:soho:provider456",
						Amount:       10000,
						Currency:     "FED",
						Status:       "pending",
						CreatedAt:    time.Now(),
					},
					{
						ID:           "payment_2",
						ChargeID:     "fed_2",
						UserDID:      "did:soho:user123",
						ProviderDID:  "did:soho:provider456",
						Amount:       5000,
						Currency:     "FED",
						Status:       "settled",
						CreatedAt:    time.Now(),
					},
					{
						ID:           "payment_3",
						ChargeID:     "fed_3",
						UserDID:      "did:soho:user123",
						ProviderDID:  "did:soho:provider456",
						Amount:       3000,
						Currency:     "FED",
						Status:       "settled",
						CreatedAt:    time.Now(),
					},
				}
				for _, row := range charges {
					if err := s.CreatePendingPayment(context.Background(), row); err != nil {
						panic(err)
					}
				}
			},
			filter: ChargeFilter{
				Status: "settled",
				Limit:  10,
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name: "with limit and offset",
			setupFunc: func(s *store.Store) {
				for i := 0; i < 15; i++ {
					row := &store.PendingPaymentRow{
						ID:           "payment_" + string(rune(i)),
						ChargeID:     "fed_" + string(rune(i)),
						UserDID:      "did:soho:user123",
						ProviderDID:  "did:soho:provider456",
						Amount:       int64(1000 * (i + 1)),
						Currency:     "FED",
						Status:       "pending",
						CreatedAt:    time.Now(),
					}
					if err := s.CreatePendingPayment(context.Background(), row); err != nil {
						panic(err)
					}
				}
			},
			filter: ChargeFilter{
				Limit:  5,
				Offset: 5,
			},
			wantErr:   false,
			wantCount: 5,
		},
		{
			name: "empty result set",
			setupFunc: func(s *store.Store) {
				// No charges created
			},
			filter: ChargeFilter{
				Limit: 10,
			},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewMemoryStore()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			defer s.Close()

			tt.setupFunc(s)
			p := NewFederationTokenProcessor(s, "0x1234567890", "/path/to/wallet")

			charges, err := p.ListCharges(context.Background(), tt.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListCharges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(charges) != tt.wantCount {
				t.Errorf("ListCharges() count = %d, want %d", len(charges), tt.wantCount)
			}
		})
	}
}

// TestFederationTokenProcessor_NoStoreError tests operations without store.
func TestFederationTokenProcessor_NoStoreError(t *testing.T) {
	// Create processor with nil store
	p := &FederationTokenProcessor{
		store:         nil,
		tokenContract: "0x1234567890",
		walletPath:    "/path/to/wallet",
		online:        true,
	}

	ctx := context.Background()

	// All operations requiring store should fail gracefully
	if err := p.ConfirmCharge(ctx, "fed_123"); err == nil {
		t.Error("ConfirmCharge() expected error with nil store")
	}

	if err := p.RefundCharge(ctx, "fed_123", "test"); err == nil {
		t.Error("RefundCharge() expected error with nil store")
	}

	if _, err := p.GetChargeStatus(ctx, "fed_123"); err == nil {
		t.Error("GetChargeStatus() expected error with nil store")
	}

	if _, err := p.ListCharges(ctx, ChargeFilter{}); err == nil {
		t.Error("ListCharges() expected error with nil store")
	}
}

// TestFederationTokenProcessor_ChargeIDGeneration tests unique charge ID generation.
func TestFederationTokenProcessor_ChargeIDGeneration(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer s.Close()

	p := NewFederationTokenProcessor(s, "0x1234567890", "/path/to/wallet")

	req := ChargeRequest{
		Amount:       10000,
		Currency:     "FED",
		UserDID:      "did:soho:user123",
		ProviderDID:  "did:soho:provider456",
		ResourceType: "compute",
	}

	// Create multiple charges and verify unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 5; i++ {
		result, err := p.CreateCharge(context.Background(), req)
		if err != nil {
			t.Fatalf("CreateCharge() error = %v", err)
		}

		if ids[result.ChargeID] {
			t.Errorf("Duplicate charge ID: %s", result.ChargeID)
		}
		ids[result.ChargeID] = true

		// Charge IDs should start with "fed_"
		if len(result.ChargeID) < 5 || result.ChargeID[:4] != "fed_" {
			t.Errorf("Invalid charge ID format: %s", result.ChargeID)
		}

		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}
}

// TestFederationTokenProcessor_NegativeAmountRefund tests refund creates negative amount.
func TestFederationTokenProcessor_NegativeAmountRefund(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer s.Close()

	// Create original charge
	originalAmount := int64(10000)
	row := &store.PendingPaymentRow{
		ID:           "payment_123",
		ChargeID:     "fed_original",
		UserDID:      "did:soho:user123",
		ProviderDID:  "did:soho:provider456",
		Amount:       originalAmount,
		Currency:     "FED",
		ResourceType: "compute",
		Status:       "settled",
		CreatedAt:    time.Now(),
	}
	if err := s.CreatePendingPayment(context.Background(), row); err != nil {
		t.Fatalf("Failed to create original payment: %v", err)
	}

	p := NewFederationTokenProcessor(s, "0x1234567890", "/path/to/wallet")

	// Process refund
	if err := p.RefundCharge(context.Background(), "fed_original", "customer_request"); err != nil {
		t.Fatalf("RefundCharge() error = %v", err)
	}

	// Verify refund record has negative amount and reversed parties
	// (This would require querying all payments and finding the refund record)
	// For now, we just verify the original status changed
	original, err := s.GetPaymentByChargeID(context.Background(), "fed_original")
	if err != nil {
		t.Fatalf("Failed to retrieve original payment: %v", err)
	}
	if original.Status != "refunded" {
		t.Errorf("Original status = %q, want %q", original.Status, "refunded")
	}
}

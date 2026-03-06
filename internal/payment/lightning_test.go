package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestLightningProcessor_Name verifies the processor name.
func TestLightningProcessor_Name(t *testing.T) {
	p := NewLightningProcessor("localhost:8080", "macaroon_hex", "")
	if got := p.Name(); got != "lightning" {
		t.Errorf("Name() = %q, want %q", got, "lightning")
	}
}

// TestLightningProcessor_IsOnline verifies online status checks.
func TestLightningProcessor_IsOnline(t *testing.T) {
	tests := []struct {
		name    string
		lndHost string
		want    bool
	}{
		{
			name:    "online with host",
			lndHost: "localhost:8080",
			want:    true,
		},
		{
			name:    "offline without host",
			lndHost: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewLightningProcessor(tt.lndHost, "macaroon_hex", "")
			if got := p.IsOnline(context.Background()); got != tt.want {
				t.Errorf("IsOnline() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLightningProcessor_CreateCharge tests invoice creation.
func TestLightningProcessor_CreateCharge(t *testing.T) {
	tests := []struct {
		name       string
		lndHost    string
		request    ChargeRequest
		mockStatus int
		mockBody   string
		wantErr    bool
		wantStatus string
	}{
		{
			name:    "successful invoice creation",
			lndHost: "localhost:8080",
			request: ChargeRequest{
				Amount:       100000, // 100k satoshis
				Currency:     "BTC",
				UserDID:      "did:soho:user123",
				ProviderDID:  "did:soho:provider456",
				ResourceType: "storage",
			},
			mockStatus: http.StatusOK,
			mockBody: `{
				"r_hash": "abcd1234base64hash",
				"payment_request": "lnbc1000n1...",
				"add_index": "12345"
			}`,
			wantErr:    false,
			wantStatus: "pending",
		},
		{
			name:    "no LND host",
			lndHost: "",
			request: ChargeRequest{
				Amount: 50000,
			},
			wantErr: true,
		},
		{
			name:    "LND API error",
			lndHost: "localhost:8080",
			request: ChargeRequest{
				Amount: 1000,
			},
			mockStatus: http.StatusBadRequest,
			mockBody:   `{"error": "Invalid invoice amount"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lndHost == "" {
				p := NewLightningProcessor(tt.lndHost, "macaroon_hex", "")
				_, err := p.CreateCharge(context.Background(), tt.request)
				if (err != nil) != tt.wantErr {
					t.Errorf("CreateCharge() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			// Create mock LND server
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify macaroon header
				mac := r.Header.Get("Grpc-Metadata-macaroon")
				if mac == "" {
					t.Error("Expected Grpc-Metadata-macaroon header")
				}

				// Verify request method and path
				if r.Method != http.MethodPost {
					t.Errorf("Method = %q, want POST", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/v1/invoices") {
					t.Errorf("Path = %q, want /v1/invoices", r.URL.Path)
				}

				w.WriteHeader(tt.mockStatus)
				w.Write([]byte(tt.mockBody))
			}))
			defer server.Close()

			// Extract host from test server URL
			host := strings.TrimPrefix(server.URL, "https://")
			p := NewLightningProcessor(host, "macaroon_hex", "")

			result, err := p.CreateCharge(context.Background(), tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateCharge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result.Status != tt.wantStatus {
					t.Errorf("CreateCharge() status = %q, want %q", result.Status, tt.wantStatus)
				}
				if result.ChargeID == "" {
					t.Error("CreateCharge() returned empty ChargeID")
				}
				if result.Amount != tt.request.Amount {
					t.Errorf("CreateCharge() amount = %d, want %d", result.Amount, tt.request.Amount)
				}
			}
		})
	}
}

// TestLightningProcessor_ConfirmCharge tests invoice confirmation.
func TestLightningProcessor_ConfirmCharge(t *testing.T) {
	tests := []struct {
		name       string
		lndHost    string
		chargeID   string
		mockStatus int
		mockBody   string
		wantErr    bool
	}{
		{
			name:     "settled invoice",
			lndHost:  "localhost:8080",
			chargeID: "abcd1234",
			mockStatus: http.StatusOK,
			mockBody: `{
				"r_hash": "abcd1234",
				"state": "SETTLED",
				"settled": true,
				"value": "100000"
			}`,
			wantErr: false,
		},
		{
			name:     "pending invoice",
			lndHost:  "localhost:8080",
			chargeID: "abcd1234",
			mockStatus: http.StatusOK,
			mockBody: `{
				"r_hash": "abcd1234",
				"state": "OPEN",
				"settled": false,
				"value": "100000"
			}`,
			wantErr: true, // Not yet settled
		},
		{
			name:     "no LND host",
			lndHost:  "",
			chargeID: "abcd1234",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lndHost == "" {
				p := NewLightningProcessor(tt.lndHost, "macaroon_hex", "")
				err := p.ConfirmCharge(context.Background(), tt.chargeID)
				if err == nil {
					t.Error("Expected error with no LND host")
				}
				return
			}

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Method = %q, want GET", r.Method)
				}
				w.WriteHeader(tt.mockStatus)
				w.Write([]byte(tt.mockBody))
			}))
			defer server.Close()

			host := strings.TrimPrefix(server.URL, "https://")
			p := NewLightningProcessor(host, "macaroon_hex", "")

			err := p.ConfirmCharge(context.Background(), tt.chargeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfirmCharge() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLightningProcessor_GetChargeStatus tests invoice status retrieval.
func TestLightningProcessor_GetChargeStatus(t *testing.T) {
	tests := []struct {
		name       string
		lndHost    string
		chargeID   string
		mockStatus int
		mockBody   string
		wantErr    bool
		wantStatus string
		wantAmount int64
	}{
		{
			name:     "settled invoice with timestamps",
			lndHost:  "localhost:8080",
			chargeID: "abcd1234",
			mockStatus: http.StatusOK,
			mockBody: `{
				"r_hash": "abcd1234",
				"state": "SETTLED",
				"settled": true,
				"value": "100000",
				"creation_date": "1609459200",
				"settle_date": "1609459300"
			}`,
			wantErr:    false,
			wantStatus: "succeeded",
			wantAmount: 100000,
		},
		{
			name:     "open invoice",
			lndHost:  "localhost:8080",
			chargeID: "xyz789",
			mockStatus: http.StatusOK,
			mockBody: `{
				"r_hash": "xyz789",
				"state": "OPEN",
				"settled": false,
				"value": "50000",
				"creation_date": "1609459200"
			}`,
			wantErr:    false,
			wantStatus: "pending",
			wantAmount: 50000,
		},
		{
			name:     "canceled invoice",
			lndHost:  "localhost:8080",
			chargeID: "canceled123",
			mockStatus: http.StatusOK,
			mockBody: `{
				"r_hash": "canceled123",
				"state": "CANCELED",
				"settled": false,
				"value": "25000"
			}`,
			wantErr:    false,
			wantStatus: "failed",
			wantAmount: 25000,
		},
		{
			name:     "no LND host",
			lndHost:  "",
			chargeID: "abcd1234",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lndHost == "" {
				p := NewLightningProcessor(tt.lndHost, "macaroon_hex", "")
				_, err := p.GetChargeStatus(context.Background(), tt.chargeID)
				if err == nil {
					t.Error("Expected error with no LND host")
				}
				return
			}

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatus)
				w.Write([]byte(tt.mockBody))
			}))
			defer server.Close()

			host := strings.TrimPrefix(server.URL, "https://")
			p := NewLightningProcessor(host, "macaroon_hex", "")

			status, err := p.GetChargeStatus(context.Background(), tt.chargeID)
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
				if status.ChargeID != tt.chargeID {
					t.Errorf("ChargeID = %q, want %q", status.ChargeID, tt.chargeID)
				}
			}
		})
	}
}

// TestLightningProcessor_ListCharges tests listing invoices.
func TestLightningProcessor_ListCharges(t *testing.T) {
	tests := []struct {
		name       string
		lndHost    string
		filter     ChargeFilter
		mockStatus int
		mockBody   string
		wantErr    bool
		wantCount  int
	}{
		{
			name:    "list with default limit",
			lndHost: "localhost:8080",
			filter: ChargeFilter{
				Limit: 0, // Should default to 10
			},
			mockStatus: http.StatusOK,
			mockBody: `{
				"invoices": [
					{
						"r_hash": "hash1",
						"state": "SETTLED",
						"settled": true,
						"value": "100000",
						"creation_date": "1609459200"
					},
					{
						"r_hash": "hash2",
						"state": "OPEN",
						"settled": false,
						"value": "50000",
						"creation_date": "1609459100"
					}
				]
			}`,
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:    "list with custom limit and offset",
			lndHost: "localhost:8080",
			filter: ChargeFilter{
				Limit:  5,
				Offset: 10,
			},
			mockStatus: http.StatusOK,
			mockBody: `{
				"invoices": []
			}`,
			wantErr:   false,
			wantCount: 0,
		},
		{
			name:    "no LND host",
			lndHost: "",
			filter: ChargeFilter{
				Limit: 10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lndHost == "" {
				p := NewLightningProcessor(tt.lndHost, "macaroon_hex", "")
				_, err := p.ListCharges(context.Background(), tt.filter)
				if err == nil {
					t.Error("Expected error with no LND host")
				}
				return
			}

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify query parameters
				query := r.URL.Query()
				if tt.filter.Limit > 0 {
					if query.Get("num_max_invoices") == "" {
						t.Error("Expected num_max_invoices parameter")
					}
				}
				if tt.filter.Offset > 0 {
					if query.Get("index_offset") == "" {
						t.Error("Expected index_offset parameter")
					}
				}
				if query.Get("reversed") != "true" {
					t.Error("Expected reversed=true parameter")
				}

				w.WriteHeader(tt.mockStatus)
				w.Write([]byte(tt.mockBody))
			}))
			defer server.Close()

			host := strings.TrimPrefix(server.URL, "https://")
			p := NewLightningProcessor(host, "macaroon_hex", "")

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

// TestLightningProcessor_RefundCharge tests Lightning refunds via keysend.
func TestLightningProcessor_RefundCharge(t *testing.T) {
	tests := []struct {
		name        string
		lndHost     string
		chargeID    string
		reason      string
		lookupBody  string
		sendStatus  int
		wantErr     bool
	}{
		{
			name:     "successful refund",
			lndHost:  "localhost:8080",
			chargeID: "abcd1234",
			reason:   "customer_request",
			lookupBody: `{
				"r_hash": "abcd1234",
				"value": "100000",
				"state": "SETTLED"
			}`,
			sendStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:     "refund with zero amount",
			lndHost:  "localhost:8080",
			chargeID: "zero_amt",
			reason:   "test",
			lookupBody: `{
				"r_hash": "zero_amt",
				"value": "0",
				"state": "SETTLED"
			}`,
			wantErr: true,
		},
		{
			name:     "no LND host",
			lndHost:  "",
			chargeID: "abcd1234",
			reason:   "test",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lndHost == "" {
				p := NewLightningProcessor(tt.lndHost, "macaroon_hex", "")
				err := p.RefundCharge(context.Background(), tt.chargeID, tt.reason)
				if err == nil {
					t.Error("Expected error with no LND host")
				}
				return
			}

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					// Invoice lookup
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tt.lookupBody))
				} else if r.Method == http.MethodPost {
					// Keysend payment
					w.WriteHeader(tt.sendStatus)
					w.Write([]byte(`{"payment_hash": "refund_hash"}`))
				}
			}))
			defer server.Close()

			host := strings.TrimPrefix(server.URL, "https://")
			p := NewLightningProcessor(host, "macaroon_hex", "")

			err := p.RefundCharge(context.Background(), tt.chargeID, tt.reason)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefundCharge() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestMapLNDStatus tests the LND status mapping function.
func TestMapLNDStatus(t *testing.T) {
	tests := []struct {
		state   string
		settled bool
		want    string
	}{
		{"OPEN", false, "pending"},
		{"OPEN", true, "succeeded"},
		{"SETTLED", false, "succeeded"},
		{"SETTLED", true, "succeeded"},
		{"CANCELED", false, "failed"},
		{"ACCEPTED", false, "pending"},
		{"UNKNOWN", false, "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := mapLNDStatus(tt.state, tt.settled); got != tt.want {
				t.Errorf("mapLNDStatus(%q, %v) = %q, want %q", tt.state, tt.settled, got, tt.want)
			}
		})
	}
}

// TestLightningProcessor_TLSConfiguration tests TLS settings.
func TestLightningProcessor_TLSConfiguration(t *testing.T) {
	p := NewLightningProcessor("localhost:8080", "macaroon_hex", "")

	// Verify TLS client is configured
	if p.client == nil {
		t.Fatal("HTTP client is nil")
	}

	// Verify timeout is set
	if p.client.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", p.client.Timeout)
	}

	// Verify TLS transport exists
	transport, ok := p.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected http.Transport")
	}
	if transport.TLSClientConfig == nil {
		t.Error("Expected TLS configuration")
	}
}

// TestLightningProcessor_DoLNDRequest tests the LND HTTP request helper.
func TestLightningProcessor_DoLNDRequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify macaroon header
		if r.Header.Get("Grpc-Metadata-macaroon") != "test_macaroon" {
			t.Error("Missing or incorrect macaroon header")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "https://")
	p := NewLightningProcessor(host, "test_macaroon", "")

	body, err := p.doLNDRequest(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("doLNDRequest() error = %v", err)
	}

	if !strings.Contains(string(body), "success") {
		t.Errorf("Unexpected response body: %s", string(body))
	}
}

// TestLightningProcessor_JSONParsing tests invoice response parsing.
func TestLightningProcessor_JSONParsing(t *testing.T) {
	tests := []struct {
		name     string
		jsonBody string
		wantErr  bool
	}{
		{
			name: "valid invoice response",
			jsonBody: `{
				"r_hash": "abcd1234",
				"payment_request": "lnbc1000n1...",
				"add_index": "12345"
			}`,
			wantErr: false,
		},
		{
			name: "valid invoice lookup",
			jsonBody: `{
				"r_hash": "abcd1234",
				"value": "100000",
				"state": "SETTLED",
				"settled": true,
				"creation_date": "1609459200",
				"settle_date": "1609459300"
			}`,
			wantErr: false,
		},
		{
			name:     "invalid JSON",
			jsonBody: `{broken json`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ir lndInvoiceResponse
			err1 := json.Unmarshal([]byte(tt.jsonBody), &ir)

			var il lndInvoiceLookup
			err2 := json.Unmarshal([]byte(tt.jsonBody), &il)

			// At least one should succeed if valid JSON
			if !tt.wantErr && err1 != nil && err2 != nil {
				t.Errorf("Both unmarshal attempts failed: %v, %v", err1, err2)
			}
			if tt.wantErr && err1 == nil {
				t.Error("Expected JSON parsing error")
			}
		})
	}
}

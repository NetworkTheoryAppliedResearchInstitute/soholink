package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/payment"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// handleGetBalance returns the current revenue balance
// GET /api/revenue/balance
func (s *Server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get total revenue
	total, err := s.store.GetTotalRevenue(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get revenue: %v", err), http.StatusInternalServerError)
		return
	}

	// Get pending payout
	pending, err := s.store.GetPendingPayout(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get pending payout: %v", err), http.StatusInternalServerError)
		return
	}

	settled := total - pending

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_revenue": total,
		"settled":       settled,
		"pending":       pending,
		"currency":      "USD",
	})
}

// handleGetRevenueHistory returns transaction history
// GET /api/revenue/history
func (s *Server) handleGetRevenueHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	// Get recent revenue entries
	entries, err := s.store.GetRecentRevenue(r.Context(), limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get history: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"transactions": entries,
		"count":        len(entries),
	})
}

// handleGetRevenueStats returns revenue statistics
// GET /api/revenue/stats
func (s *Server) handleGetRevenueStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get total revenue
	total, err := s.store.GetTotalRevenue(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get total: %v", err), http.StatusInternalServerError)
		return
	}

	// Get revenue since various time periods
	day := time.Now().Add(-24 * time.Hour)
	week := time.Now().Add(-7 * 24 * time.Hour)
	month := time.Now().Add(-30 * 24 * time.Hour)

	dayRevenue, _ := s.store.GetRevenueSince(r.Context(), day)
	weekRevenue, _ := s.store.GetRevenueSince(r.Context(), week)
	monthRevenue, _ := s.store.GetRevenueSince(r.Context(), month)

	// Get revenue by resource type
	cpuRevenue, _ := s.store.GetRevenueByType(r.Context(), "cpu")
	memoryRevenue, _ := s.store.GetRevenueByType(r.Context(), "memory")
	storageRevenue, _ := s.store.GetRevenueByType(r.Context(), "storage")
	gpuRevenue, _ := s.store.GetRevenueByType(r.Context(), "gpu")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total": total,
		"periods": map[string]float64{
			"last_24h": float64(dayRevenue),
			"last_7d":  float64(weekRevenue),
			"last_30d": float64(monthRevenue),
		},
		"by_resource_type": map[string]float64{
			"cpu":     float64(cpuRevenue),
			"memory":  float64(memoryRevenue),
			"storage": float64(storageRevenue),
			"gpu":     float64(gpuRevenue),
		},
	})
}

// handleGetActiveRentals returns currently active resource rentals
// GET /api/revenue/active-rentals
func (s *Server) handleGetActiveRentals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rentals, err := s.store.GetActiveRentals(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get rentals: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rentals": rentals,
		"count":   len(rentals),
	})
}

// handleRequestPayout requests a payout of pending revenue.
// POST /api/revenue/request-payout
func (s *Server) handleRequestPayout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Amount        float64 `json:"amount"`         // satoshis
		PaymentMethod string  `json:"payment_method"` // "lightning", "stripe", "barter"
		Address       string  `json:"address"`        // Lightning invoice / bank ref
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	if req.PaymentMethod == "" {
		http.Error(w, "Payment method required", http.StatusBadRequest)
		return
	}

	// Resolve the provider's node DID from node_info.
	providerDID, _ := s.store.GetNodeInfo(r.Context(), "owner_did")
	if providerDID == "" {
		providerDID = "unknown"
	}

	// Check pending balance (unsettled producer_payout in central_revenue).
	pending, err := s.store.GetPendingPayout(r.Context())
	if err != nil {
		http.Error(w, "Failed to check balance", http.StatusInternalServerError)
		return
	}

	amountSats := int64(req.Amount)
	if amountSats > pending {
		// Do not reveal the exact balance in the error response.
		http.Error(w, "Insufficient balance for this payout", http.StatusBadRequest)
		return
	}

	// Dispatch via the payment ledger when available; fall back to a simple
	// store-recorded request so history works even without a live processor.
	if s.paymentLedger != nil {
		result, err := s.paymentLedger.RequestPayout(r.Context(), payment.PayoutRequest{
			ProviderDID:   providerDID,
			AmountSats:    amountSats,
			Processor:     req.PaymentMethod,
			PayoutAddress: req.Address,
		})
		if err != nil {
			log.Printf("[revenue] RequestPayout error: %v", err)
			http.Error(w, "Payout request failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
			"payout_id":      result.PayoutID,
			"amount_sats":    amountSats,
			"payment_method": req.PaymentMethod,
			"status":         result.Status,
			"external_id":    result.ExternalID,
			"created_at":     time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	// No ledger attached — still record the request directly.
	payoutID := fmt.Sprintf("po_%d", time.Now().UnixNano())
	pr := storePayoutRow(providerDID, payoutID, amountSats, req.PaymentMethod)
	_ = s.store.CreatePayout(r.Context(), &pr)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"payout_id":      payoutID,
		"amount_sats":    amountSats,
		"payment_method": req.PaymentMethod,
		"status":         "pending",
		"created_at":     time.Now().UTC().Format(time.RFC3339),
	})
}

// storePayoutRow builds a store.PayoutRow value for direct insertion.
func storePayoutRow(providerDID, payoutID string, amountSats int64, processor string) store.PayoutRow {
	return store.PayoutRow{
		PayoutID:    payoutID,
		ProviderDID: providerDID,
		AmountSats:  amountSats,
		Processor:   processor,
		Status:      "pending",
		RequestedAt: time.Now().UTC(),
	}
}

// handleGetPayoutHistory returns payout history for this node.
// GET /api/revenue/payouts?limit=N
func (s *Server) handleGetPayoutHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Cap at 100 to prevent resource exhaustion.
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	providerDID, _ := s.store.GetNodeInfo(r.Context(), "owner_did")

	payouts, err := s.store.ListPayouts(r.Context(), providerDID, limit)
	if err != nil {
		log.Printf("[revenue] ListPayouts error: %v", err)
		http.Error(w, "Failed to load payout history", http.StatusInternalServerError)
		return
	}
	if payouts == nil {
		payouts = []store.PayoutRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"payouts": payouts,
		"count":   len(payouts),
	})
}

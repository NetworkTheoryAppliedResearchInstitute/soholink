package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
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

// handleRequestPayout requests a payout of pending revenue
// POST /api/revenue/request-payout
func (s *Server) handleRequestPayout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Amount        float64 `json:"amount"`
		PaymentMethod string  `json:"payment_method"`
		Address       string  `json:"address"`
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

	// Check pending balance
	pending, err := s.store.GetPendingPayout(r.Context())
	if err != nil {
		http.Error(w, "Failed to check balance", http.StatusInternalServerError)
		return
	}

	if req.Amount > float64(pending) {
		http.Error(w, "Insufficient pending balance", http.StatusBadRequest)
		return
	}

	// Create payout request (this would integrate with payment processor)
	payoutID := fmt.Sprintf("payout-%d", time.Now().Unix())

	// TODO: Integrate with actual payment processor
	// For now, just record the request

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"payout_id":      payoutID,
		"amount":         req.Amount,
		"payment_method": req.PaymentMethod,
		"status":         "pending",
		"created_at":     time.Now().Format(time.RFC3339),
	})
}

// handleGetPayoutHistory returns payout history
// GET /api/revenue/payouts
func (s *Server) handleGetPayoutHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement payout history query from database

	// Placeholder response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"payouts": []interface{}{},
		"count":   0,
	})
}

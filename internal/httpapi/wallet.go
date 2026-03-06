package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// handleWalletBalance serves GET /api/wallet/balance.
// Returns the requester's prepaid sats balance with BTC/USD conversion.
func (s *Server) handleWalletBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	did, err := s.store.GetNodeInfo(ctx, "owner_did")
	if err != nil || did == "" {
		http.Error(w, "Owner DID not configured", http.StatusInternalServerError)
		return
	}

	var balanceSats int64
	if s.paymentLedger != nil {
		balanceSats, err = s.paymentLedger.GetWalletBalance(ctx, did)
		if err != nil {
			log.Printf("[wallet] GetWalletBalance error: %v", err)
			http.Error(w, "Failed to retrieve wallet balance", http.StatusInternalServerError)
			return
		}
	}

	btcRate := GetBtcUsdRate()
	balanceBtc := float64(balanceSats) / 1e8
	balanceUsd := balanceBtc * btcRate

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"balance_sats": balanceSats,
		"balance_btc":  balanceBtc,
		"balance_usd":  balanceUsd,
		"btc_usd_rate": btcRate,
	})
}

// walletTopupRequest is the JSON body for POST /api/wallet/topup.
type walletTopupRequest struct {
	AmountSats     int64  `json:"amount_sats"`
	Processor      string `json:"processor"`       // "lightning" | "stripe"
	IdempotencyKey string `json:"idempotency_key"` // optional: deduplication key for retries
}

// handleWalletTopup serves POST /api/wallet/topup.
// Creates a Lightning invoice or Stripe payment intent.
// The wallet is credited only after ConfirmTopup is called (or webhook fires).
func (s *Server) handleWalletTopup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	did, err := s.store.GetNodeInfo(ctx, "owner_did")
	if err != nil || did == "" {
		http.Error(w, "Owner DID not configured", http.StatusInternalServerError)
		return
	}

	var req walletTopupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.AmountSats <= 0 {
		http.Error(w, "amount_sats must be positive", http.StatusBadRequest)
		return
	}
	if req.Processor == "" {
		req.Processor = "lightning"
	}

	if s.paymentLedger == nil {
		http.Error(w, "Payment system not configured", http.StatusServiceUnavailable)
		return
	}

	topupID, invoice, err := s.paymentLedger.TopupWallet(ctx, did, req.Processor, req.AmountSats, req.IdempotencyKey)
	if err != nil {
		log.Printf("[wallet] TopupWallet error: %v", err)
		http.Error(w, "Failed to create topup request", http.StatusInternalServerError)
		return
	}
	walletTopupTotal.Inc() // Prometheus: count successfully initiated topups

	instructions := "Pay this Lightning invoice to credit your wallet"
	if req.Processor == "stripe" {
		instructions = "Complete the Stripe payment to credit your wallet"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"topup_id":     topupID,
		"invoice":      invoice,
		"amount_sats":  req.AmountSats,
		"processor":    req.Processor,
		"status":       "awaiting_payment",
		"instructions": instructions,
	})
}

// handleWalletTopups serves GET /api/wallet/topups.
// Returns paginated topup history for the owner DID.
func (s *Server) handleWalletTopups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	did, err := s.store.GetNodeInfo(ctx, "owner_did")
	if err != nil || did == "" {
		http.Error(w, "Owner DID not configured", http.StatusInternalServerError)
		return
	}

	// Cap at 100 to prevent resource exhaustion (matches revenue history pattern).
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, parseErr := strconv.Atoi(l); parseErr == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	topups, err := s.store.ListWalletTopups(ctx, did, limit)
	if err != nil {
		log.Printf("[wallet] ListWalletTopups error: %v", err)
		http.Error(w, "Failed to retrieve topup history", http.StatusInternalServerError)
		return
	}
	if topups == nil {
		topups = []store.WalletTopupRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"topups": topups,
		"count":  len(topups),
	})
}

// confirmTopupRequest is the JSON body for POST /api/wallet/confirm-topup.
type confirmTopupRequest struct {
	TopupID string `json:"topup_id"`
}

// handleConfirmTopup serves POST /api/wallet/confirm-topup.
// For manual / development confirmation; in production webhooks call this path.
func (s *Server) handleConfirmTopup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	did, err := s.store.GetNodeInfo(ctx, "owner_did")
	if err != nil || did == "" {
		http.Error(w, "Owner DID not configured", http.StatusInternalServerError)
		return
	}

	var req confirmTopupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.TopupID == "" {
		http.Error(w, "Missing topup_id", http.StatusBadRequest)
		return
	}

	if s.paymentLedger == nil {
		http.Error(w, "Payment system not configured", http.StatusServiceUnavailable)
		return
	}

	if err := s.paymentLedger.ConfirmTopup(ctx, req.TopupID); err != nil {
		log.Printf("[wallet] ConfirmTopup error: %v", err)
		// Do not leak internal error details to the client.
		http.Error(w, "Payment confirmation failed", http.StatusBadRequest)
		return
	}

	newBalance, err := s.paymentLedger.GetWalletBalance(ctx, did)
	if err != nil {
		log.Printf("[wallet] GetWalletBalance after confirm error: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"status":           "confirmed",
		"new_balance_sats": newBalance,
	})
}

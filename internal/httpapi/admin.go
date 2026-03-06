package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/moderation"
)

// ---------------------------------------------------------------------------
// Admin blocklist API (Item 1 & 2 — CSAM hash + DID blocklist management)
// ---------------------------------------------------------------------------
// Routes (all require admin auth — device token with owner DID):
//
//   POST   /api/admin/blocklist/dids            — block a DID
//   DELETE /api/admin/blocklist/dids/{did}      — unblock a DID
//   GET    /api/admin/blocklist/dids            — list blocked DIDs
//   GET    /api/federation/blocklist            — public federation pull (no auth)
//   POST   /api/admin/blocklist/hashes          — add content hash
//   GET    /api/admin/blocklist/hashes          — list blocked hashes
// ---------------------------------------------------------------------------

// handleAdminBlocklistDIDs routes GET / POST on /api/admin/blocklist/dids.
func (s *Server) handleAdminBlocklistDIDs(w http.ResponseWriter, r *http.Request) {
	if s.blocklist == nil {
		http.Error(w, "blocklist not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleListBlockedDIDs(w, r)
	case http.MethodPost:
		s.handleBlockDID(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAdminBlocklistDIDByID routes DELETE on /api/admin/blocklist/dids/{did}.
func (s *Server) handleAdminBlocklistDIDByID(w http.ResponseWriter, r *http.Request) {
	if s.blocklist == nil {
		http.Error(w, "blocklist not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	did := strings.TrimPrefix(r.URL.Path, "/api/admin/blocklist/dids/")
	did = strings.TrimSuffix(did, "/")
	if did == "" {
		http.Error(w, "DID is required in path", http.StatusBadRequest)
		return
	}
	if err := s.blocklist.Unblock(r.Context(), did); err != nil {
		log.Printf("[admin] Unblock DID error: %v", err)
		http.Error(w, "Failed to unblock DID", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "unblocked", "did": did}) //nolint:errcheck
}

// handleListBlockedDIDs serves GET /api/admin/blocklist/dids.
func (s *Server) handleListBlockedDIDs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.blocklist.ListBlocked(r.Context(), 200)
	if err != nil {
		log.Printf("[admin] ListBlockedDIDs error: %v", err)
		http.Error(w, "Failed to list blocked DIDs", http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = nil // keep nil for json null → frontend treats as empty
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"blocked_dids": rows,
		"count":        len(rows),
	})
}

// blockDIDRequest is the JSON body for POST /api/admin/blocklist/dids.
type blockDIDRequest struct {
	DID       string  `json:"did"`
	Reason    string  `json:"reason"`
	ExpiresAt *string `json:"expires_at,omitempty"` // RFC3339 string or null for permanent
}

// handleBlockDID serves POST /api/admin/blocklist/dids.
func (s *Server) handleBlockDID(w http.ResponseWriter, r *http.Request) {
	var req blockDIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.DID == "" || req.Reason == "" {
		http.Error(w, "did and reason are required", http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			http.Error(w, "expires_at must be RFC3339 or null", http.StatusBadRequest)
			return
		}
		expiresAt = &t
	}

	ctx := r.Context()
	// Use the owner DID as the "blocked_by" actor
	byDID, _ := s.store.GetNodeInfo(ctx, "owner_did")
	if byDID == "" {
		byDID = "platform"
	}

	if err := s.blocklist.Block(ctx, req.DID, req.Reason, byDID, expiresAt); err != nil {
		log.Printf("[admin] Block DID error: %v", err)
		http.Error(w, "Failed to block DID", http.StatusInternalServerError)
		return
	}

	log.Printf("[admin] DID blocked: %s reason=%s by=%s", req.DID, req.Reason, byDID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status": "blocked",
		"did":    req.DID,
		"reason": req.Reason,
	})
}

// handleFederationBlocklist serves GET /api/federation/blocklist.
// No authentication required — this is a public endpoint that peer nodes
// call periodically to pull the list of permanently banned DIDs.
func (s *Server) handleFederationBlocklist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.blocklist == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"blocked_dids": []interface{}{},
			"count":        0,
		})
		return
	}
	rows, err := s.blocklist.FederationSnapshot(r.Context())
	if err != nil {
		log.Printf("[federation] FederationBlocklist error: %v", err)
		http.Error(w, "Failed to retrieve blocklist", http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = nil
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"blocked_dids": rows,
		"count":        len(rows),
	})
}

// ---------------------------------------------------------------------------
// Content hash blocklist (CSAM / illegal content — Item 1)
// ---------------------------------------------------------------------------

// handleAdminBlocklistHashes routes GET / POST on /api/admin/blocklist/hashes.
func (s *Server) handleAdminBlocklistHashes(w http.ResponseWriter, r *http.Request) {
	if s.hashChecker == nil {
		http.Error(w, "hash checker not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleListBlockedHashes(w, r)
	case http.MethodPost:
		s.handleAddBlockedHash(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListBlockedHashes serves GET /api/admin/blocklist/hashes.
func (s *Server) handleListBlockedHashes(w http.ResponseWriter, r *http.Request) {
	hashes, err := s.hashChecker.ListHashes(r.Context(), 200)
	if err != nil {
		log.Printf("[admin] ListBlockedHashes error: %v", err)
		http.Error(w, "Failed to list blocked hashes", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"blocked_hashes": hashes,
		"count":          len(hashes),
	})
}

// addHashRequest is the JSON body for POST /api/admin/blocklist/hashes.
type addHashRequest struct {
	HashSHA256 string `json:"hash_sha256"`
	Reason     string `json:"reason"` // "csam"|"illegal_content"|"known_malware"
	Source     string `json:"source"` // "ncmec"|"manual"|"clamav_heuristic"
}

// handleAddBlockedHash serves POST /api/admin/blocklist/hashes.
func (s *Server) handleAddBlockedHash(w http.ResponseWriter, r *http.Request) {
	var req addHashRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.HashSHA256 == "" || req.Reason == "" || req.Source == "" {
		http.Error(w, "hash_sha256, reason, and source are required", http.StatusBadRequest)
		return
	}
	if len(req.HashSHA256) != 64 {
		http.Error(w, "hash_sha256 must be a 64-character hex-encoded SHA-256", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	byDID, _ := s.store.GetNodeInfo(ctx, "owner_did")
	if byDID == "" {
		byDID = "platform"
	}

	if err := s.hashChecker.AddHash(ctx, req.HashSHA256, req.Reason, req.Source, byDID); err != nil {
		log.Printf("[admin] AddBlockedHash error: %v", err)
		http.Error(w, "Failed to add blocked hash", http.StatusInternalServerError)
		return
	}

	log.Printf("[admin] content hash blocked: sha256=%s reason=%s source=%s by=%s",
		req.HashSHA256[:16]+"...", req.Reason, req.Source, byDID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status":      "blocked",
		"hash_sha256": req.HashSHA256,
		"reason":      req.Reason,
	})
}

// ---------------------------------------------------------------------------
// handleAdminStatus — quick admin status summary
// ---------------------------------------------------------------------------

// handleAdminStatus serves GET /api/admin/status.
func (s *Server) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	status := map[string]interface{}{
		"safety_policy_enabled": s.safetyPolicy != nil,
		"hash_checker_enabled":  s.hashChecker != nil,
		"blocklist_enabled":     s.blocklist != nil,
	}

	if s.blocklist != nil {
		rows, _ := s.blocklist.ListBlocked(ctx, 1000)
		status["blocked_dids_count"] = len(rows)
	}
	if s.hashChecker != nil {
		hashes, _ := s.hashChecker.ListHashes(ctx, 10000)
		status["blocked_hashes_count"] = len(hashes)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status) //nolint:errcheck
}

// setupAdminRoutes wires all admin + safety API routes onto the given mux.
// Called from Server.Start() after all other routes are registered.
func (s *Server) setupAdminRoutes(mux *http.ServeMux) {
	// Admin blocklist — DID management
	mux.HandleFunc("/api/admin/blocklist/dids/", s.handleAdminBlocklistDIDByID)
	mux.HandleFunc("/api/admin/blocklist/dids", s.handleAdminBlocklistDIDs)

	// Admin blocklist — content hash management
	mux.HandleFunc("/api/admin/blocklist/hashes", s.handleAdminBlocklistHashes)

	// Admin status
	mux.HandleFunc("/api/admin/status", s.handleAdminStatus)

	// Federation pull — public, no auth (add to publicPaths in auth_middleware.go)
	mux.HandleFunc("/api/federation/blocklist", s.handleFederationBlocklist)
}

// Ensure the moderation import is used (compiler guard).
var _ *moderation.DIDBlocklist

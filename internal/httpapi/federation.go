package httpapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// coordinatorMeta holds static coordinator info returned by GET /api/federation/info.
// It is set once via Server.SetCoordinatorMeta before Start() is called.
type coordinatorMeta struct {
	DID        string  // coordinator's node DID (from node_info["owner_did"])
	FeePercent float64 // e.g. 1.0
	Regions    []string
}

// SetCoordinatorMeta configures the coordinator metadata returned by
// GET /api/federation/info. Call this before Start() when IsCoordinator=true.
func (s *Server) SetCoordinatorMeta(did string, feePct float64, regions []string) {
	s.coordMeta = &coordinatorMeta{DID: did, FeePercent: feePct, Regions: regions}
}

// ---------------------------------------------------------------------------
// GET /api/federation/info  — public, returns coordinator metadata
// ---------------------------------------------------------------------------

func (s *Server) handleFederationInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.coordMeta == nil {
		http.Error(w, "not a coordinator node", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"coordinator_did": s.coordMeta.DID,
		"fee_percent":     s.coordMeta.FeePercent,
		"regions":         s.coordMeta.Regions,
		"version":         "1",
	})
}

// ---------------------------------------------------------------------------
// GET /api/federation/peers  — public, returns active registered nodes
// ---------------------------------------------------------------------------

func (s *Server) handleFederationPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.store == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"count": 0, "peers": []interface{}{}}) // #nosec G104
		return
	}

	nodes, err := s.store.ListActiveFederationNodes(r.Context())
	if err != nil {
		log.Printf("[federation] ListActiveFederationNodes error: %v", err)
		http.Error(w, "registry query failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"count": len(nodes),
		"peers": nodes,
	})
}

// ---------------------------------------------------------------------------
// POST /api/federation/announce  — register or re-register a provider node
// ---------------------------------------------------------------------------

type announceRequest struct {
	NodeDID             string        `json:"node_did"`
	PublicKey           string        `json:"public_key"`  // base64-encoded Ed25519 public key
	Address             string        `json:"address"`
	Region              string        `json:"region"`
	Resources           nodeResources `json:"resources"`
	PricePerCPUHourSats int64         `json:"price_per_cpu_hour_sats"`
	Timestamp           string        `json:"timestamp"`
	Signature           string        `json:"signature"`   // base64-encoded Ed25519 signature
}

type nodeResources struct {
	TotalCPU     float64 `json:"total_cpu"`
	AvailableCPU float64 `json:"available_cpu"`
	TotalMemMB   int64   `json:"total_memory_mb"`
	AvailMemMB   int64   `json:"available_memory_mb"`
	TotalDiskGB  int64   `json:"total_disk_gb"`
	AvailDiskGB  int64   `json:"available_disk_gb"`
	GPUModel     string  `json:"gpu_model"`
}

func (s *Server) handleFederationAnnounce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Gap 3: rate-limit announce to 5 per IP per minute (slow/expensive operation).
	if !s.rateLimiter.Allow(r, 5) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	if s.store == nil {
		http.Error(w, "store not ready", http.StatusServiceUnavailable)
		return
	}

	var req announceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.NodeDID == "" || req.Address == "" {
		http.Error(w, "node_did and address are required", http.StatusBadRequest)
		return
	}
	if req.PublicKey == "" || req.Signature == "" {
		http.Error(w, "public_key and signature are required", http.StatusBadRequest)
		return
	}

	// Reject stale timestamps (> 5 minutes old).
	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil || time.Since(ts) > 5*time.Minute {
		http.Error(w, "timestamp missing or too old", http.StatusBadRequest)
		return
	}

	// Gap 1: verify Ed25519 signature.
	// Canonical message matches federation/announcer.go sign() implementation.
	if err := verifyAnnounceSignature(req); err != nil {
		log.Printf("[federation] announce signature invalid from %s: %v", req.NodeDID, err)
		http.Error(w, "signature verification failed", http.StatusUnauthorized)
		return
	}

	row := &store.FederationNodeRow{
		NodeDID:           req.NodeDID,
		Address:           req.Address,
		Region:            req.Region,
		TotalCPU:          req.Resources.TotalCPU,
		AvailableCPU:      req.Resources.AvailableCPU,
		TotalMemoryMB:     req.Resources.TotalMemMB,
		AvailableMemoryMB: req.Resources.AvailMemMB,
		TotalDiskGB:       req.Resources.TotalDiskGB,
		AvailableDiskGB:   req.Resources.AvailDiskGB,
		GPUModel:          req.Resources.GPUModel,
		PricePerCPUHour:   req.PricePerCPUHourSats,
		ReputationScore:   50, // neutral starting score
		UptimePercent:     0,
		FailureRate:       0,
		Status:            "online",
		LastHeartbeat:     time.Now().UTC(),
		PublicKey:         req.PublicKey,
	}

	if err := s.store.UpsertFederationNode(r.Context(), row); err != nil {
		log.Printf("[federation] UpsertFederationNode error: %v", err)
		http.Error(w, "registry update failed", http.StatusInternalServerError)
		return
	}

	log.Printf("[federation] node registered: %s @ %s (%s)", req.NodeDID, req.Address, req.Region)

	feePct := 1.0
	if s.coordMeta != nil {
		feePct = s.coordMeta.FeePercent
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"status":          "registered",
		"fee_percent":     feePct,
		"heartbeat_every": "30s",
	})
}

// verifyAnnounceSignature verifies the Ed25519 signature in an announce request.
// Canonical message: "{nodeDID}:{address}:{timestamp}" (matches announcer.go sign()).
func verifyAnnounceSignature(req announceRequest) error {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		return fmt.Errorf("decode public_key: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("public_key must be %d bytes, got %d", ed25519.PublicKeySize, len(pubKeyBytes))
	}
	sigBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	msg := []byte(req.NodeDID + ":" + req.Address + ":" + req.Timestamp)
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), msg, sigBytes) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// ---------------------------------------------------------------------------
// POST /api/federation/heartbeat  — keep-alive + resource update
// ---------------------------------------------------------------------------

type heartbeatRequest struct {
	NodeDID   string        `json:"node_did"`
	Resources nodeResources `json:"resources"`
	Timestamp string        `json:"timestamp"`
	Signature string        `json:"signature"`
}

func (s *Server) handleFederationHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Gap 3: rate-limit heartbeats to 10 per IP per minute (burst-tolerant).
	if !s.rateLimiter.Allow(r, 10) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	if s.store == nil {
		http.Error(w, "store not ready", http.StatusServiceUnavailable)
		return
	}

	var req heartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.NodeDID == "" {
		http.Error(w, "node_did required", http.StatusBadRequest)
		return
	}

	// Gap 1: verify heartbeat signature against the stored public key.
	// Canonical message: "{nodeDID}:{timestamp}" (matches announcer.go sign()).
	node, err := s.store.GetFederationNode(r.Context(), req.NodeDID)
	if err != nil || node == nil {
		http.Error(w, "unknown node — announce first", http.StatusUnauthorized)
		return
	}
	if node.PublicKey == "" {
		// Legacy node with no stored key — accept but log warning.
		log.Printf("[federation] heartbeat from %s has no stored public key; skipping sig check", req.NodeDID)
	} else if req.Signature == "" {
		http.Error(w, "signature required", http.StatusUnauthorized)
		return
	} else {
		if err := verifyHeartbeatSignature(req, node.PublicKey); err != nil {
			log.Printf("[federation] heartbeat signature invalid from %s: %v", req.NodeDID, err)
			http.Error(w, "signature verification failed", http.StatusUnauthorized)
			return
		}
	}

	if err := s.store.UpdateFederationHeartbeat(r.Context(),
		req.NodeDID,
		req.Resources.AvailableCPU,
		req.Resources.AvailMemMB,
		req.Resources.AvailDiskGB,
	); err != nil {
		log.Printf("[federation] UpdateFederationHeartbeat error: %v", err)
		http.Error(w, "heartbeat update failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) // #nosec G104
}

// verifyHeartbeatSignature verifies the Ed25519 signature in a heartbeat request.
// Canonical message: "{nodeDID}:{timestamp}" (matches announcer.go sign()).
func verifyHeartbeatSignature(req heartbeatRequest, storedPubKeyB64 string) error {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(storedPubKeyB64)
	if err != nil {
		return fmt.Errorf("decode stored public_key: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("stored public_key invalid size: %d", len(pubKeyBytes))
	}
	sigBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	msg := []byte(req.NodeDID + ":" + req.Timestamp)
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), msg, sigBytes) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// ---------------------------------------------------------------------------
// POST /api/federation/deregister  — clean offline notification
// ---------------------------------------------------------------------------

func (s *Server) handleFederationDeregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Gap 3: rate-limit deregister to 5 per IP per minute.
	if !s.rateLimiter.Allow(r, 5) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	if s.store == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req struct {
		NodeDID string `json:"node_did"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NodeDID == "" {
		http.Error(w, "node_did required", http.StatusBadRequest)
		return
	}

	_ = s.store.SetFederationNodeOffline(r.Context(), req.NodeDID)
	log.Printf("[federation] node deregistered: %s", req.NodeDID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "offline"}) // #nosec G104
}

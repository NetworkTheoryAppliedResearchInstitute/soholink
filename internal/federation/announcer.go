// Package federation implements the provider-side federation announcer and
// the coordinator-side registry client.
//
// Every SoHoLINK node that has a coordinator_url configured will:
//  1. Announce itself on startup with resource capacity + pricing.
//  2. Send a heartbeat every HeartbeatInterval (default 30 s) with
//     current available resource figures.
//  3. Send a deregister on clean shutdown (best-effort).
//
// Coordinators store the registry in their local SQLite federation_nodes
// table and expose it via GET /api/federation/peers.
package federation

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// NodeResources carries the current resource snapshot sent on announce/heartbeat.
type NodeResources struct {
	TotalCPU      float64 `json:"total_cpu"`
	AvailableCPU  float64 `json:"available_cpu"`
	TotalMemMB    int64   `json:"total_memory_mb"`
	AvailMemMB    int64   `json:"available_memory_mb"`
	TotalDiskGB   int64   `json:"total_disk_gb"`
	AvailDiskGB   int64   `json:"available_disk_gb"`
	GPUModel      string  `json:"gpu_model"`
}

// AnnounceRequest is the payload sent to POST /api/federation/announce.
type AnnounceRequest struct {
	NodeDID             string        `json:"node_did"`
	PublicKey           string        `json:"public_key"`   // base64 Ed25519
	Address             string        `json:"address"`      // host:port
	Region              string        `json:"region"`
	Resources           NodeResources `json:"resources"`
	PricePerCPUHourSats int64         `json:"price_per_cpu_hour_sats"`
	Timestamp           string        `json:"timestamp"`    // RFC3339 UTC
	Signature           string        `json:"signature"`    // base64 Ed25519 sig
}

// HeartbeatRequest is the payload sent to POST /api/federation/heartbeat.
type HeartbeatRequest struct {
	NodeDID   string        `json:"node_did"`
	Resources NodeResources `json:"resources"`
	Timestamp string        `json:"timestamp"`
	Signature string        `json:"signature"`
}

// Announcer manages the provider side of federation: announces the node to
// its configured coordinator and keeps the registration alive via heartbeats.
type Announcer struct {
	coordinatorURL      string
	nodeDID             string
	address             string
	region              string
	pricePerCPUHourSats int64
	privSeedHex         string // 64-char hex — node signing key seed
	pubKeyB64           string // base64 — node signing public key
	interval            time.Duration
	resourcesFn         func() NodeResources // called each heartbeat for live stats
	client              *http.Client
}

// Config holds the parameters needed to build an Announcer.
type Config struct {
	CoordinatorURL      string
	NodeDID             string
	Address             string
	Region              string
	PricePerCPUHourSats int64
	PrivSeedHex         string
	PubKeyB64           string
	HeartbeatInterval   time.Duration
	ResourcesFn         func() NodeResources // nil → reports zeros
}

// New creates a new Announcer from Config.
func New(cfg Config) *Announcer {
	interval := cfg.HeartbeatInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	rfn := cfg.ResourcesFn
	if rfn == nil {
		rfn = func() NodeResources { return NodeResources{} }
	}
	return &Announcer{
		coordinatorURL:      cfg.CoordinatorURL,
		nodeDID:             cfg.NodeDID,
		address:             cfg.Address,
		region:              cfg.Region,
		pricePerCPUHourSats: cfg.PricePerCPUHourSats,
		privSeedHex:         cfg.PrivSeedHex,
		pubKeyB64:           cfg.PubKeyB64,
		interval:            interval,
		resourcesFn:         rfn,
		client:              &http.Client{Timeout: 10 * time.Second},
	}
}

// Start announces the node immediately, then sends heartbeats on the
// configured interval until ctx is cancelled.
func (a *Announcer) Start(ctx context.Context) {
	log.Printf("[federation] announcing to coordinator %s (interval %s)",
		a.coordinatorURL, a.interval)

	if err := a.announce(ctx); err != nil {
		log.Printf("[federation] initial announce failed: %v", err)
	}

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := a.heartbeat(ctx); err != nil {
				log.Printf("[federation] heartbeat error: %v", err)
			}
		case <-ctx.Done():
			a.deregister() // best-effort, ignore error
			return
		}
	}
}

// announce sends a full registration to the coordinator.
func (a *Announcer) announce(ctx context.Context) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	res := a.resourcesFn()

	req := AnnounceRequest{
		NodeDID:             a.nodeDID,
		PublicKey:           a.pubKeyB64,
		Address:             a.address,
		Region:              a.region,
		Resources:           res,
		PricePerCPUHourSats: a.pricePerCPUHourSats,
		Timestamp:           ts,
	}

	// Sign the canonical message: "{nodeDID}:{address}:{timestamp}"
	msg := fmt.Sprintf("%s:%s:%s", a.nodeDID, a.address, ts)
	sig, err := a.sign(msg)
	if err != nil {
		return fmt.Errorf("announce sign: %w", err)
	}
	req.Signature = sig

	return a.post(ctx, "/api/federation/announce", req)
}

// heartbeat sends a lightweight resource update to the coordinator.
func (a *Announcer) heartbeat(ctx context.Context) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	res := a.resourcesFn()

	req := HeartbeatRequest{
		NodeDID:   a.nodeDID,
		Resources: res,
		Timestamp: ts,
	}
	msg := fmt.Sprintf("%s:%s", a.nodeDID, ts)
	sig, err := a.sign(msg)
	if err != nil {
		return fmt.Errorf("heartbeat sign: %w", err)
	}
	req.Signature = sig

	return a.post(ctx, "/api/federation/heartbeat", req)
}

// deregister notifies the coordinator that this node is going offline.
func (a *Announcer) deregister() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type deregReq struct {
		NodeDID string `json:"node_did"`
	}
	_ = a.post(ctx, "/api/federation/deregister", deregReq{NodeDID: a.nodeDID})
	log.Printf("[federation] deregistered from coordinator")
}

// sign returns a base64-encoded Ed25519 signature over msg using the node's
// federation signing key.
func (a *Announcer) sign(msg string) (string, error) {
	if a.privSeedHex == "" {
		return "", nil // unsigned (no key configured)
	}
	seed, err := hex.DecodeString(a.privSeedHex)
	if err != nil {
		return "", fmt.Errorf("decode signing key: %w", err)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, []byte(msg))
	return base64.StdEncoding.EncodeToString(sig), nil
}

// post marshals body to JSON and POSTs it to the coordinator.
func (a *Announcer) post(ctx context.Context, path string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.coordinatorURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("coordinator returned %d for %s", resp.StatusCode, path)
	}
	return nil
}

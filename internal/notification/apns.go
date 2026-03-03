// Package notification provides an Apple Push Notification Service (APNs)
// client for SoHoLINK.
//
// APNs uses HTTP/2 with JWT-based authentication (Provider Authentication
// Tokens).  This implementation requires no external dependencies beyond
// the Go standard library and golang.org/x/crypto (already vendored).
//
// Reference: https://developer.apple.com/documentation/usernotifications/
//            setting_up_a_remote_notification_server
package notification

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

// APNs gateway URLs.
const (
	apnsProductionHost = "https://api.push.apple.com"
	apnsSandboxHost    = "https://api.sandbox.push.apple.com"
)

// ErrDeviceTokenInvalid is returned (wrapped) when APNs responds with
// HTTP 410 Gone — the device token is no longer valid.  Callers should
// remove the token from storage and stop sending notifications to it.
//
// Use errors.Is(err, notification.ErrDeviceTokenInvalid) to detect this.
var ErrDeviceTokenInvalid = errors.New("apns: device token is no longer valid (410 Gone)")

// apnsJWTLifetime is how long a provider JWT token is valid.
// APNs tokens expire after one hour; we refresh proactively at 55 minutes.
const apnsJWTLifetime = 55 * time.Minute

// ---------------------------------------------------------------------------
// APNSConfig
// ---------------------------------------------------------------------------

// APNSConfig holds the APNs Provider Token credentials obtained from the
// Apple Developer portal.
type APNSConfig struct {
	// KeyID is the 10-character Key ID from the Apple Developer portal.
	KeyID string

	// TeamID is the 10-character Apple Developer Team ID.
	TeamID string

	// PrivateKeyPEM is the PEM-encoded ECDSA (ES256) private key (.p8 file).
	PrivateKeyPEM string

	// BundleID is the iOS app bundle identifier (e.g. "com.example.soholink").
	BundleID string

	// Sandbox indicates whether to use the APNs sandbox gateway.
	// Set to true during development; false for production.
	Sandbox bool
}

// ---------------------------------------------------------------------------
// APNSNotifier
// ---------------------------------------------------------------------------

// APNSNotifier sends APNs push notifications using provider JWT authentication.
// It caches and auto-refreshes the JWT token.  Safe for concurrent use.
type APNSNotifier struct {
	cfg    APNSConfig
	key    *ecdsa.PrivateKey
	client *http.Client

	mu         sync.Mutex
	token      string
	tokenExpAt time.Time
}

// NewAPNSNotifier creates an APNSNotifier.
// Returns an error if the private key PEM cannot be parsed.
func NewAPNSNotifier(cfg APNSConfig) (*APNSNotifier, error) {
	if cfg.KeyID == "" || cfg.TeamID == "" || cfg.PrivateKeyPEM == "" || cfg.BundleID == "" {
		return nil, errors.New("apns: KeyID, TeamID, PrivateKeyPEM, and BundleID are all required")
	}

	key, err := parseAPNSPrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("apns: parse private key: %w", err)
	}

	// Go's net/http automatically negotiates HTTP/2 for HTTPS connections.
	return &APNSNotifier{
		cfg:    cfg,
		key:    key,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// ---------------------------------------------------------------------------
// Public notification methods
// ---------------------------------------------------------------------------

// SendJobRequest pushes a "new job available" alert to a device.
func (n *APNSNotifier) SendJobRequest(ctx context.Context, deviceToken, taskID, workloadID string) error {
	payload := apnsPayload{
		Aps: apnsAps{
			Alert: apnsAlert{
				Title: "New compute job available",
				Body:  fmt.Sprintf("Task %s is ready for your device.", taskID),
			},
			Sound: "default",
		},
		TaskID:     taskID,
		WorkloadID: workloadID,
		EventType:  "job_request",
	}
	return n.send(ctx, deviceToken, payload, "com.soholink.job-request")
}

// SendPaymentReceived pushes a payment-received alert to a device.
func (n *APNSNotifier) SendPaymentReceived(ctx context.Context, deviceToken string, amountSats int64) error {
	payload := apnsPayload{
		Aps: apnsAps{
			Alert: apnsAlert{
				Title: "Payment received",
				Body:  fmt.Sprintf("You earned %d sats.", amountSats),
			},
			Sound: "default",
			Badge: 1,
		},
		AmountSats: amountSats,
		EventType:  "payment_received",
	}
	return n.send(ctx, deviceToken, payload, "com.soholink.payment")
}

// SendNodeOffline pushes a node-offline alert to a device.
func (n *APNSNotifier) SendNodeOffline(ctx context.Context, deviceToken, nodeDID string) error {
	short := nodeDID
	if len(short) > 16 {
		short = short[:16] + "…"
	}
	payload := apnsPayload{
		Aps: apnsAps{
			Alert: apnsAlert{
				Title: "Node offline",
				Body:  fmt.Sprintf("Node %s is no longer reachable.", short),
			},
			Sound: "default",
		},
		NodeDID:   nodeDID,
		EventType: "node_offline",
	}
	return n.send(ctx, deviceToken, payload, "com.soholink.node-alert")
}

// ---------------------------------------------------------------------------
// Internal send helper
// ---------------------------------------------------------------------------

// apnsPayload is the JSON body delivered to the device via APNs.
type apnsPayload struct {
	Aps        apnsAps `json:"aps"`
	EventType  string  `json:"event_type,omitempty"`
	TaskID     string  `json:"task_id,omitempty"`
	WorkloadID string  `json:"workload_id,omitempty"`
	NodeDID    string  `json:"node_did,omitempty"`
	AmountSats int64   `json:"amount_sats,omitempty"`
}

type apnsAps struct {
	Alert apnsAlert `json:"alert"`
	Sound string    `json:"sound,omitempty"`
	Badge int       `json:"badge,omitempty"`
	// ContentAvailable: 1 triggers a background fetch on iOS.
	ContentAvailable int `json:"content-available,omitempty"`
}

type apnsAlert struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// send serialises payload and posts it to the APNs HTTP/2 endpoint.
func (n *APNSNotifier) send(ctx context.Context, deviceToken string, payload apnsPayload, collapseID string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("apns: marshal payload: %w", err)
	}

	host := apnsProductionHost
	if n.cfg.Sandbox {
		host = apnsSandboxHost
	}
	url := fmt.Sprintf("%s/3/device/%s", host, deviceToken)

	token, err := n.providerToken()
	if err != nil {
		return fmt.Errorf("apns: get provider token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apns: create request: %w", err)
	}
	req.Header.Set("authorization", "bearer "+token)
	req.Header.Set("apns-topic", n.cfg.BundleID)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("content-type", "application/json")
	if collapseID != "" {
		req.Header.Set("apns-collapse-id", collapseID)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("apns: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("[apns] notification sent to device %.12s… (type=%s)",
			deviceToken, payload.EventType)
		return nil
	}

	var apnsErr struct {
		Reason    string `json:"reason"`
		Timestamp int64  `json:"timestamp"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&apnsErr)

	// A4 fix: HTTP 410 means the device token has been permanently invalidated
	// (user uninstalled the app, or APNs rotated the token).  Return a typed
	// sentinel so callers can distinguish this from transient errors and remove
	// the stale token from storage.
	if resp.StatusCode == http.StatusGone {
		return fmt.Errorf("%w: %s", ErrDeviceTokenInvalid, apnsErr.Reason)
	}

	return fmt.Errorf("apns: HTTP %d: %s", resp.StatusCode, apnsErr.Reason)
}

// ---------------------------------------------------------------------------
// JWT token management
// ---------------------------------------------------------------------------

// providerToken returns a valid JWT provider token, refreshing if expired.
func (n *APNSNotifier) providerToken() (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now()
	if n.token != "" && now.Before(n.tokenExpAt) {
		return n.token, nil
	}

	// A2 fix: capture the timestamp BEFORE mintJWT so that the expiry clock
	// starts at the moment the iat claim is stamped, not after the (possibly
	// slow) signing operation completes.
	token, err := n.mintJWT(now)
	if err != nil {
		return "", err
	}

	n.token = token
	n.tokenExpAt = now.Add(apnsJWTLifetime)
	return token, nil
}

// mintJWT creates a new ES256 JWT for APNs provider authentication.
// now is the timestamp used for the iat claim and must match the value used
// to calculate tokenExpAt in providerToken (A2 fix).
// Format: base64url(header).base64url(claims).base64url(signature)
func (n *APNSNotifier) mintJWT(now time.Time) (string, error) {
	headerJSON, err := json.Marshal(map[string]string{
		"alg": "ES256",
		"kid": n.cfg.KeyID,
	})
	if err != nil {
		return "", fmt.Errorf("apns: marshal JWT header: %w", err)
	}

	claimsJSON, err := json.Marshal(map[string]interface{}{
		"iss": n.cfg.TeamID,
		"iat": now.Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("apns: marshal JWT claims: %w", err)
	}

	signingInput := jwtBase64URL(headerJSON) + "." + jwtBase64URL(claimsJSON)

	sig, err := signES256(n.key, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("apns: sign JWT: %w", err)
	}

	return signingInput + "." + jwtBase64URL(sig), nil
}

// ---------------------------------------------------------------------------
// Crypto helpers (all stdlib; no external dependencies)
// ---------------------------------------------------------------------------

// parseAPNSPrivateKey parses a PEM-encoded ECDSA private key.
// Apple .p8 files use PKCS#8 DER; falls back to SEC 1 (EC PRIVATE KEY).
func parseAPNSPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("apns: no PEM block found in private key")
	}

	// Attempt PKCS#8 first (Apple .p8 format).
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		ecKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("apns: PKCS8 key is not ECDSA")
		}
		return ecKey, nil
	}

	// Fall back to SEC 1 (legacy EC PRIVATE KEY PEM type).
	ecKey, err2 := x509.ParseECPrivateKey(block.Bytes)
	if err2 != nil {
		return nil, fmt.Errorf("apns: parse key (PKCS8: %v; SEC1: %v)", err, err2)
	}
	return ecKey, nil
}

// signES256 signs data with ECDSA/SHA-256 and returns the raw 64-byte r‖s
// signature as required by JWS (RFC 7518 §3.4).
func signES256(key *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	digest := sha256.Sum256(data)

	// crypto/ecdsa.Sign returns a DER-encoded ASN.1 signature.
	derSig, err := ecdsaSign(key, digest[:])
	if err != nil {
		return nil, err
	}
	return derToRawSig(derSig)
}

// ecdsaSign calls ecdsa.SignASN1 (available since Go 1.15).
// Defined as a small wrapper so that the call site stays clean.
func ecdsaSign(key *ecdsa.PrivateKey, digest []byte) ([]byte, error) {
	return ecdsa.SignASN1(rand.Reader, key, digest)
}

// derToRawSig converts a DER-encoded ECDSA signature (r, s integers) to the
// fixed 64-byte r‖s format used in JWS.
func derToRawSig(der []byte) ([]byte, error) {
	var (
		inner cryptobyte.String
		r, s  big.Int
	)
	input := cryptobyte.String(der)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) {
		return nil, errors.New("apns: invalid DER sig: missing SEQUENCE")
	}
	if !inner.ReadASN1Integer(&r) {
		return nil, errors.New("apns: invalid DER sig: missing r")
	}
	if !inner.ReadASN1Integer(&s) {
		return nil, errors.New("apns: invalid DER sig: missing s")
	}

	const halfLen = 32 // P-256 curve order is 32 bytes
	rb := r.Bytes()
	sb := s.Bytes()
	if len(rb) > halfLen || len(sb) > halfLen {
		return nil, fmt.Errorf("apns: r or s too large (%d, %d)", len(rb), len(sb))
	}

	out := make([]byte, halfLen*2)
	copy(out[halfLen-len(rb):halfLen], rb)
	copy(out[halfLen*2-len(sb):], sb)
	return out, nil
}

// jwtBase64URL encodes b using base64url without padding, as required by JWT.
func jwtBase64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

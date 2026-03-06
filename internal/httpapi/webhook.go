package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxWebhookBytes caps the body read for webhook events.
// Stripe recommends 65 536 bytes; we allow a small margin.
const maxWebhookBytes = 131072 // 128 KB

// stripeEvent is a minimal representation of a Stripe webhook event.
type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID       string            `json:"id"`
			Metadata map[string]string `json:"metadata"`
		} `json:"object"`
	} `json:"data"`
}

// handleStripeWebhook handles POST /api/webhooks/stripe.
//
// Security: the raw request body is verified against the Stripe-Signature
// header using HMAC-SHA256 before any action is taken.  Requests with an
// invalid or missing signature are rejected with 400.
//
// Supported event types:
//   - payment_intent.succeeded  → ConfirmTopup
//   - checkout.session.completed → ConfirmTopup (via payment_intent metadata)
//   - payment_intent.payment_failed → FailTopup
func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the body before anything else — signature covers the raw bytes.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBytes))
	if err != nil {
		log.Printf("[webhook] failed to read body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify the Stripe-Signature header when a webhook secret is configured.
	if s.stripeWebhookSecret != "" {
		sigHeader := r.Header.Get("Stripe-Signature")
		if err := verifyStripeSignature(body, sigHeader, s.stripeWebhookSecret, 300); err != nil {
			log.Printf("[webhook] signature verification failed: %v", err)
			http.Error(w, "Invalid signature", http.StatusBadRequest)
			return
		}
	} else {
		// No secret configured: log a warning and allow through (dev/test only).
		log.Printf("[webhook] WARN: stripe_webhook_secret not set — skipping signature verification")
	}

	var event stripeEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("[webhook] failed to parse event JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[webhook] received stripe event: type=%s id=%s", event.Type, event.ID)

	// Stripe expects a 2xx response within 30 s; process synchronously.
	switch event.Type {
	case "payment_intent.succeeded", "checkout.session.completed":
		topupID := event.Data.Object.Metadata["topup_id"]
		if topupID == "" {
			// Not a wallet topup event — acknowledge and ignore.
			log.Printf("[webhook] %s: no topup_id in metadata (object=%s) — ignoring",
				event.Type, event.Data.Object.ID)
			break
		}
		if s.paymentLedger == nil {
			log.Printf("[webhook] paymentLedger not configured; cannot confirm topup %s", topupID)
			break
		}
		if err := s.paymentLedger.ConfirmTopup(r.Context(), topupID); err != nil {
			log.Printf("[webhook] ConfirmTopup(%s) error: %v", topupID, err)
			// Return 200 even on error — Stripe will not retry; operator must
			// resolve manually via the admin API.
		} else {
			log.Printf("[webhook] topup %s confirmed via %s event", topupID, event.Type)
		}

	case "payment_intent.payment_failed":
		topupID := event.Data.Object.Metadata["topup_id"]
		if topupID == "" {
			log.Printf("[webhook] payment_intent.payment_failed: no topup_id in metadata (object=%s) — ignoring",
				event.Data.Object.ID)
			break
		}
		if s.paymentLedger == nil {
			log.Printf("[webhook] paymentLedger not configured; cannot fail topup %s", topupID)
			break
		}
		if err := s.paymentLedger.FailTopup(r.Context(), topupID); err != nil {
			log.Printf("[webhook] FailTopup(%s) error: %v", topupID, err)
		} else {
			log.Printf("[webhook] topup %s marked failed via payment_intent.payment_failed", topupID)
		}

	default:
		// Unknown event type — acknowledge and ignore.
		log.Printf("[webhook] unhandled event type: %s", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}

// verifyStripeSignature checks the Stripe-Signature header against the raw
// webhook payload using HMAC-SHA256.
//
// The header has the form: t=<unix-timestamp>,v1=<hex-hmac>[,v1=<alt>...]
// Stripe may include multiple v1 signatures during secret rotation.
//
// toleranceSecs is the maximum allowed age of the timestamp (Stripe recommends 300).
func verifyStripeSignature(payload []byte, sigHeader, secret string, toleranceSecs int64) error {
	if sigHeader == "" {
		return fmt.Errorf("missing Stripe-Signature header")
	}

	var ts int64
	var signatures [][]byte

	for _, part := range strings.Split(sigHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			val, err := strconv.ParseInt(strings.TrimPrefix(part, "t="), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid timestamp in Stripe-Signature: %w", err)
			}
			ts = val
		} else if strings.HasPrefix(part, "v1=") {
			sig, err := hex.DecodeString(strings.TrimPrefix(part, "v1="))
			if err != nil {
				return fmt.Errorf("invalid hex in Stripe-Signature: %w", err)
			}
			signatures = append(signatures, sig)
		}
	}

	if ts == 0 {
		return fmt.Errorf("missing timestamp (t=) in Stripe-Signature")
	}
	if len(signatures) == 0 {
		return fmt.Errorf("no v1 signatures found in Stripe-Signature")
	}

	// Reject stale events.
	age := time.Now().Unix() - ts
	if age > toleranceSecs || age < -toleranceSecs {
		return fmt.Errorf("timestamp too old or in future: age=%ds tolerance=%ds", age, toleranceSecs)
	}

	// Compute expected HMAC: HMAC-SHA256(secret, "<timestamp>.<payload>")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d", ts)))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := mac.Sum(nil)

	for _, sig := range signatures {
		if hmac.Equal(expected, sig) {
			return nil // at least one signature matches
		}
	}
	return fmt.Errorf("no matching v1 signature found")
}

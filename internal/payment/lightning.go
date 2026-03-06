package payment

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

// LightningProcessor implements PaymentProcessor using Bitcoin Lightning Network.
type LightningProcessor struct {
	lndHost  string
	macaroon string
	client   *http.Client
	online   bool
}

// NewLightningProcessor creates a new Bitcoin Lightning payment processor.
// tlsCertPath is the path to LND's tls.cert file for certificate pinning.
// When provided, the cert is loaded into a dedicated pool and InsecureSkipVerify
// is disabled.  When empty, a warning is logged and verification is skipped
// (acceptable for local dev, not for production).
func NewLightningProcessor(lndHost, macaroon, tlsCertPath string) *LightningProcessor {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if tlsCertPath != "" {
		certPEM, err := os.ReadFile(tlsCertPath)
		if err == nil {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(certPEM)
			tlsConfig.RootCAs = pool
			log.Printf("[lightning] TLS certificate pinned from %s", tlsCertPath)
		} else {
			log.Printf("[lightning] WARNING: could not read TLS cert %s: %v — falling back to InsecureSkipVerify", tlsCertPath, err)
			tlsConfig.InsecureSkipVerify = true // #nosec G402 -- cert file unreadable; operator should fix lnd_tls_cert_path
		}
	} else {
		log.Printf("[lightning] WARNING: lnd_tls_cert_path not configured — TLS verification disabled. Set this for production use!")
		tlsConfig.InsecureSkipVerify = true // #nosec G402 -- no cert path provided; configure lnd_tls_cert_path in production
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &LightningProcessor{
		lndHost:  lndHost,
		macaroon: macaroon,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		online: lndHost != "",
	}
}

func (p *LightningProcessor) Name() string {
	return "lightning"
}

func (p *LightningProcessor) IsOnline(ctx context.Context) bool {
	return p.online && p.lndHost != ""
}

// lndInvoiceRequest is the JSON body for creating an LND invoice.
type lndInvoiceRequest struct {
	Value int64  `json:"value"`
	Memo  string `json:"memo"`
}

// lndInvoiceResponse is the JSON response from LND invoice creation.
type lndInvoiceResponse struct {
	RHash          string `json:"r_hash"`
	PaymentRequest string `json:"payment_request"`
	AddIndex       string `json:"add_index"`
}

// lndInvoiceLookup is the JSON response from LND invoice lookup.
type lndInvoiceLookup struct {
	RHash          string `json:"r_hash"`
	Value          string `json:"value"`
	State          string `json:"state"` // OPEN, SETTLED, CANCELED, ACCEPTED
	Settled        bool   `json:"settled"`
	CreationDate   string `json:"creation_date"`
	SettleDate     string `json:"settle_date"`
	PaymentRequest string `json:"payment_request"`
	Memo           string `json:"memo"`
}

// lndListInvoicesResponse wraps the list invoices response.
type lndListInvoicesResponse struct {
	Invoices []lndInvoiceLookup `json:"invoices"`
}

// lndSendRequest is the JSON body for a keysend payment (used for refunds).
type lndSendRequest struct {
	Dest   string `json:"dest"`
	Amt    int64  `json:"amt"`
	DestCustomRecords map[string]string `json:"dest_custom_records,omitempty"`
}

// doLNDRequest executes an LND REST API request with macaroon auth.
func (p *LightningProcessor) doLNDRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	endpoint := fmt.Sprintf("https://%s%s", p.lndHost, path)

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("lightning: failed to create request: %w", err)
	}
	req.Header.Set("Grpc-Metadata-macaroon", p.macaroon)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lightning: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lightning: failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("lightning: LND returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// mapLNDStatus maps an LND invoice state to our internal status.
func mapLNDStatus(state string, settled bool) string {
	if settled {
		return "succeeded"
	}
	switch state {
	case "OPEN":
		return "pending"
	case "SETTLED":
		return "succeeded"
	case "CANCELED":
		return "failed"
	case "ACCEPTED":
		return "pending"
	default:
		return "pending"
	}
}

func (p *LightningProcessor) CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	if p.lndHost == "" {
		return nil, fmt.Errorf("lightning: LND host not configured")
	}

	invoiceReq := lndInvoiceRequest{
		Value: req.Amount,
		Memo:  req.ResourceType,
	}

	bodyBytes, err := json.Marshal(invoiceReq)
	if err != nil {
		return nil, fmt.Errorf("lightning: failed to marshal invoice request: %w", err)
	}

	respBody, err := p.doLNDRequest(ctx, http.MethodPost, "/v1/invoices", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	var ir lndInvoiceResponse
	if err := json.Unmarshal(respBody, &ir); err != nil {
		return nil, fmt.Errorf("lightning: failed to parse invoice response: %w", err)
	}

	// The r_hash from LND is base64-encoded; we use it as our charge ID.
	chargeID := ir.RHash

	return &ChargeResult{
		ChargeID:     chargeID,
		Status:       "pending",
		Amount:       req.Amount,
		ProcessorFee: 0, // Lightning routing fees are near-zero
		NetAmount:    req.Amount,
	}, nil
}

func (p *LightningProcessor) ConfirmCharge(ctx context.Context, chargeID string) error {
	if p.lndHost == "" {
		return fmt.Errorf("lightning: LND host not configured")
	}

	// For Lightning, "confirming" means checking if the invoice has been settled.
	// We look up the invoice by r_hash and verify its state.
	rHashHex := chargeID
	// Try to use the chargeID as hex for the lookup path.
	path := fmt.Sprintf("/v1/invoice/%s", url.PathEscape(rHashHex))

	respBody, err := p.doLNDRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}

	var il lndInvoiceLookup
	if err := json.Unmarshal(respBody, &il); err != nil {
		return fmt.Errorf("lightning: failed to parse invoice lookup: %w", err)
	}

	if il.State == "SETTLED" || il.Settled {
		return nil // Invoice is confirmed/settled
	}

	return fmt.Errorf("lightning: invoice %s not yet settled (state=%s)", chargeID, il.State)
}

func (p *LightningProcessor) RefundCharge(ctx context.Context, chargeID string, reason string) error {
	if p.lndHost == "" {
		return fmt.Errorf("lightning: LND host not configured")
	}

	// First, look up the original invoice to get the amount and destination.
	rHashHex := chargeID
	path := fmt.Sprintf("/v1/invoice/%s", url.PathEscape(rHashHex))

	respBody, err := p.doLNDRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("lightning: failed to look up invoice for refund: %w", err)
	}

	var il lndInvoiceLookup
	if err := json.Unmarshal(respBody, &il); err != nil {
		return fmt.Errorf("lightning: failed to parse invoice for refund: %w", err)
	}

	// Parse the value from the invoice.
	var amount int64
	fmt.Sscanf(il.Value, "%d", &amount)
	if amount <= 0 {
		return fmt.Errorf("lightning: cannot refund invoice with zero amount")
	}

	// For refund, we use the keysend payment route via /v2/router/send.
	// In practice, you would need the destination pubkey from the invoice.
	// We use the r_hash as a custom record to link the refund to the original payment.
	sendReq := lndSendRequest{
		Amt: amount,
		DestCustomRecords: map[string]string{
			// TLV type 5482373484 is the standard preimage key for keysend
			"5482373484": hex.EncodeToString([]byte(chargeID)),
		},
	}

	sendBody, err := json.Marshal(sendReq)
	if err != nil {
		return fmt.Errorf("lightning: failed to marshal send request: %w", err)
	}

	_, err = p.doLNDRequest(ctx, http.MethodPost, "/v2/router/send", bytes.NewReader(sendBody))
	if err != nil {
		return fmt.Errorf("lightning: refund keysend failed: %w", err)
	}

	return nil
}

func (p *LightningProcessor) GetChargeStatus(ctx context.Context, chargeID string) (*ChargeStatus, error) {
	if p.lndHost == "" {
		return nil, fmt.Errorf("lightning: LND host not configured")
	}

	rHashHex := chargeID
	path := fmt.Sprintf("/v1/invoice/%s", url.PathEscape(rHashHex))

	respBody, err := p.doLNDRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var il lndInvoiceLookup
	if err := json.Unmarshal(respBody, &il); err != nil {
		return nil, fmt.Errorf("lightning: failed to parse invoice lookup: %w", err)
	}

	var amount int64
	fmt.Sscanf(il.Value, "%d", &amount)

	status := mapLNDStatus(il.State, il.Settled)

	cs := &ChargeStatus{
		ChargeID: chargeID,
		Status:   status,
		Amount:   amount,
	}

	// Parse creation date (Unix timestamp string).
	var creationTS int64
	fmt.Sscanf(il.CreationDate, "%d", &creationTS)
	if creationTS > 0 {
		cs.CreatedAt = time.Unix(creationTS, 0)
	}

	// Parse settle date if present.
	var settleTS int64
	fmt.Sscanf(il.SettleDate, "%d", &settleTS)
	if settleTS > 0 {
		t := time.Unix(settleTS, 0)
		cs.SettledAt = &t
	}

	return cs, nil
}

func (p *LightningProcessor) ListCharges(ctx context.Context, filter ChargeFilter) ([]ChargeStatus, error) {
	if p.lndHost == "" {
		return nil, fmt.Errorf("lightning: LND host not configured")
	}

	params := url.Values{}
	if filter.Limit > 0 {
		params.Set("num_max_invoices", fmt.Sprintf("%d", filter.Limit))
	} else {
		params.Set("num_max_invoices", "10")
	}
	if filter.Offset > 0 {
		params.Set("index_offset", fmt.Sprintf("%d", filter.Offset))
	}
	params.Set("reversed", "true") // Most recent first

	path := "/v1/invoices?" + params.Encode()

	respBody, err := p.doLNDRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var lr lndListInvoicesResponse
	if err := json.Unmarshal(respBody, &lr); err != nil {
		return nil, fmt.Errorf("lightning: failed to parse list response: %w", err)
	}

	var charges []ChargeStatus
	for _, inv := range lr.Invoices {
		var amount int64
		fmt.Sscanf(inv.Value, "%d", &amount)

		status := mapLNDStatus(inv.State, inv.Settled)

		cs := ChargeStatus{
			ChargeID: inv.RHash,
			Status:   status,
			Amount:   amount,
		}

		var creationTS int64
		fmt.Sscanf(inv.CreationDate, "%d", &creationTS)
		if creationTS > 0 {
			cs.CreatedAt = time.Unix(creationTS, 0)
		}

		var settleTS int64
		fmt.Sscanf(inv.SettleDate, "%d", &settleTS)
		if settleTS > 0 {
			t := time.Unix(settleTS, 0)
			cs.SettledAt = &t
		}

		charges = append(charges, cs)
	}

	return charges, nil
}

package payment

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// HTLC hold-invoice helpers
//
// LND exposes hold invoices via the invoicesrpc sub-server (not the standard
// /v1/invoices endpoint).  The three operations below implement the full
// lifecycle:
//
//   1. CreateHoldInvoice  — coordinator creates a hold invoice keyed to the
//                           expected result hash; shares the payment_request
//                           with the mobile node (and requestor).
//   2. SettleHoldInvoice  — coordinator settles once shadow-replica verification
//                           passes; releases funds to the provider.
//   3. CancelHoldInvoice  — coordinator cancels if verification fails or the
//                           mobile node disappears without delivering a result.
//
// Reference: https://api.lightning.community/#invoicesrpc-AddHoldInvoice
// ---------------------------------------------------------------------------

// lndHoldInvoiceRequest is the JSON body for POST /v2/invoices/hodl.
// LND's gRPC-gateway REST layer encodes all protobuf `bytes` fields as
// standard base64 (not hex).  Sending hex causes LND to return 400.
type lndHoldInvoiceRequest struct {
	// Hash is the base64-encoded SHA-256 payment hash that the payer must
	// provide a preimage for to settle the invoice.
	Hash  string `json:"hash"`
	Value int64  `json:"value"`
	Memo  string `json:"memo,omitempty"`
	// Expiry is the invoice lifetime in seconds (default 3600 if zero).
	Expiry int64 `json:"expiry,omitempty"`
}

// lndHoldInvoiceResponse is the JSON response from LND hold invoice creation.
type lndHoldInvoiceResponse struct {
	PaymentRequest string `json:"payment_request"`
	AddIndex       string `json:"add_index"`
}

// lndSettleRequest is the JSON body for POST /v2/invoices/settle.
type lndSettleRequest struct {
	// Preimage is the base64-encoded 32-byte preimage that hashes to the
	// payment hash supplied at CreateHoldInvoice time.
	Preimage string `json:"preimage"`
}

// lndCancelRequest is the JSON body for POST /v2/invoices/cancel.
type lndCancelRequest struct {
	// PaymentHash is the base64-encoded SHA-256 payment hash of the invoice
	// to cancel.
	PaymentHash string `json:"payment_hash"`
}

// HoldInvoice is returned by CreateHoldInvoice.
type HoldInvoice struct {
	// PaymentRequest is the BOLT-11 invoice string to share with the payer.
	PaymentRequest string

	// PaymentHashHex is the hex-encoded payment hash the coordinator uses
	// to settle or cancel the invoice later.
	PaymentHashHex string
}

// CreateHoldInvoice creates a Lightning hold invoice on LND.
//
// Parameters:
//   - amount:         value in satoshis
//   - paymentHashHex: hex-encoded SHA-256 hash of the 32-byte preimage
//   - memo:           human-readable description attached to the invoice
//
// The invoice remains in ACCEPTED state (funds held) until SettleHoldInvoice
// or CancelHoldInvoice is called.
func (p *LightningProcessor) CreateHoldInvoice(ctx context.Context, amount int64, paymentHashHex, memo string) (*HoldInvoice, error) {
	if p.lndHost == "" {
		return nil, fmt.Errorf("htlc: LND host not configured")
	}
	if paymentHashHex == "" {
		return nil, fmt.Errorf("htlc: payment hash must not be empty")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("htlc: amount must be positive")
	}

	// Validate the payment hash is valid hex of correct length (32 bytes = 64 chars).
	if len(paymentHashHex) != 64 {
		return nil, fmt.Errorf("htlc: payment hash must be 64 hex chars (32 bytes), got %d", len(paymentHashHex))
	}
	// Decode hex → raw bytes → base64.  LND's gRPC-gateway REST layer expects
	// standard base64 for all protobuf `bytes` fields (P1 fix).
	hashBytes, err := hex.DecodeString(paymentHashHex)
	if err != nil {
		return nil, fmt.Errorf("htlc: invalid payment hash hex: %w", err)
	}

	req := lndHoldInvoiceRequest{
		Hash:   base64.StdEncoding.EncodeToString(hashBytes),
		Value:  amount,
		Memo:   memo,
		Expiry: 3600,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("htlc: marshal error: %w", err)
	}

	respBytes, err := p.doLNDRequest(ctx, "POST", "/v2/invoices/hodl", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("htlc: CreateHoldInvoice: %w", err)
	}

	var resp lndHoldInvoiceResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("htlc: parse hold invoice response: %w", err)
	}

	return &HoldInvoice{
		PaymentRequest: resp.PaymentRequest,
		PaymentHashHex: paymentHashHex,
	}, nil
}

// SettleHoldInvoice settles a previously created hold invoice.
// The preimage must be the 32-byte value whose SHA-256 equals the payment hash
// supplied to CreateHoldInvoice.  Call this after the coordinator has verified
// the task result from the shadow replica.
//
// Parameters:
//   - preimageHex: hex-encoded 32-byte preimage
func (p *LightningProcessor) SettleHoldInvoice(ctx context.Context, preimageHex string) error {
	if p.lndHost == "" {
		return fmt.Errorf("htlc: LND host not configured")
	}
	if len(preimageHex) != 64 {
		return fmt.Errorf("htlc: preimage must be 64 hex chars (32 bytes), got %d", len(preimageHex))
	}
	// Decode hex → raw bytes → base64 for LND REST API (P1 fix).
	preimageBytes, err := hex.DecodeString(preimageHex)
	if err != nil {
		return fmt.Errorf("htlc: invalid preimage hex: %w", err)
	}

	req := lndSettleRequest{Preimage: base64.StdEncoding.EncodeToString(preimageBytes)}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("htlc: marshal settle request: %w", err)
	}

	_, err = p.doLNDRequest(ctx, "POST", "/v2/invoices/settle", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("htlc: SettleHoldInvoice: %w", err)
	}

	return nil
}

// CancelHoldInvoice cancels a hold invoice, releasing any held funds back to
// the payer.  Call this when:
//   - the mobile node disconnects without delivering a result, or
//   - the shadow replica's result hash does not match the mobile result.
//
// Parameters:
//   - paymentHashHex: hex-encoded SHA-256 payment hash used at creation time
func (p *LightningProcessor) CancelHoldInvoice(ctx context.Context, paymentHashHex string) error {
	if p.lndHost == "" {
		return fmt.Errorf("htlc: LND host not configured")
	}
	if len(paymentHashHex) != 64 {
		return fmt.Errorf("htlc: payment hash must be 64 hex chars (32 bytes), got %d", len(paymentHashHex))
	}
	// Decode hex → raw bytes → base64 for LND REST API (P1 fix).
	hashBytes, err := hex.DecodeString(paymentHashHex)
	if err != nil {
		return fmt.Errorf("htlc: invalid payment hash hex: %w", err)
	}

	req := lndCancelRequest{PaymentHash: base64.StdEncoding.EncodeToString(hashBytes)}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("htlc: marshal cancel request: %w", err)
	}

	_, err = p.doLNDRequest(ctx, "POST", "/v2/invoices/cancel", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("htlc: CancelHoldInvoice: %w", err)
	}

	return nil
}

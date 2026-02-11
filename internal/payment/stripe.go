package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StripeProcessor implements PaymentProcessor using the Stripe REST API directly.
type StripeProcessor struct {
	secretKey     string
	webhookSecret string
	client        *http.Client
	offline       bool
}

// NewStripeProcessor creates a new Stripe payment processor.
// Uses direct REST API calls, no SDK dependency.
func NewStripeProcessor(secretKey, webhookSecret string) *StripeProcessor {
	return &StripeProcessor{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *StripeProcessor) Name() string {
	return "stripe"
}

func (p *StripeProcessor) IsOnline(ctx context.Context) bool {
	if p.offline || p.secretKey == "" {
		return false
	}
	return true
}

// stripeResponse is a minimal representation of a Stripe PaymentIntent response.
type stripeResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Error    *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// stripeRefundResponse is a minimal representation of a Stripe Refund response.
type stripeRefundResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Amount int64  `json:"amount"`
	Error  *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// stripeListResponse wraps a Stripe list endpoint response.
type stripeListResponse struct {
	Data []stripeResponse `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// doRequest executes a Stripe API request with Basic Auth.
func (p *StripeProcessor) doRequest(ctx context.Context, method, endpoint string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to create request: %w", err)
	}
	req.SetBasicAuth(p.secretKey, "")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("stripe: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// mapStripeStatus maps a Stripe PaymentIntent status to our internal status.
func mapStripeStatus(stripeStatus string) string {
	switch stripeStatus {
	case "requires_payment_method", "requires_confirmation", "requires_action":
		return "pending"
	case "processing":
		return "pending"
	case "succeeded":
		return "succeeded"
	case "canceled":
		return "failed"
	case "requires_capture":
		return "pending"
	default:
		return "pending"
	}
}

func (p *StripeProcessor) CreateCharge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	if p.secretKey == "" {
		return nil, fmt.Errorf("stripe: secret key not configured")
	}

	form := url.Values{}
	form.Set("amount", fmt.Sprintf("%d", req.Amount))
	form.Set("currency", strings.ToLower(req.Currency))
	if req.Currency == "" {
		form.Set("currency", "usd")
	}
	form.Set("metadata[user_did]", req.UserDID)
	form.Set("metadata[provider_did]", req.ProviderDID)
	form.Set("metadata[resource_type]", req.ResourceType)
	form.Set("metadata[usage_record_id]", req.UsageRecordID)
	for k, v := range req.Metadata {
		form.Set("metadata["+k+"]", v)
	}
	if req.IdempotencyKey != "" {
		form.Set("metadata[idempotency_key]", req.IdempotencyKey)
	}

	respBody, err := p.doRequest(ctx, http.MethodPost,
		"https://api.stripe.com/v1/payment_intents",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	var sr stripeResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return nil, fmt.Errorf("stripe: failed to parse response: %w", err)
	}

	if sr.Error != nil {
		return nil, fmt.Errorf("stripe: %s: %s", sr.Error.Type, sr.Error.Message)
	}

	status := mapStripeStatus(sr.Status)

	return &ChargeResult{
		ChargeID:     sr.ID,
		Status:       status,
		Amount:       sr.Amount,
		ProcessorFee: 0, // Stripe fees are deducted at payout, not visible here
		NetAmount:    sr.Amount,
	}, nil
}

func (p *StripeProcessor) ConfirmCharge(ctx context.Context, chargeID string) error {
	if p.secretKey == "" {
		return fmt.Errorf("stripe: secret key not configured")
	}

	endpoint := fmt.Sprintf("https://api.stripe.com/v1/payment_intents/%s/confirm",
		url.PathEscape(chargeID))

	respBody, err := p.doRequest(ctx, http.MethodPost, endpoint, strings.NewReader(""))
	if err != nil {
		return err
	}

	var sr stripeResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return fmt.Errorf("stripe: failed to parse confirm response: %w", err)
	}

	if sr.Error != nil {
		return fmt.Errorf("stripe: %s: %s", sr.Error.Type, sr.Error.Message)
	}

	return nil
}

func (p *StripeProcessor) RefundCharge(ctx context.Context, chargeID string, reason string) error {
	if p.secretKey == "" {
		return fmt.Errorf("stripe: secret key not configured")
	}

	form := url.Values{}
	form.Set("payment_intent", chargeID)
	form.Set("metadata[reason]", reason)

	respBody, err := p.doRequest(ctx, http.MethodPost,
		"https://api.stripe.com/v1/refunds",
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	var rr stripeRefundResponse
	if err := json.Unmarshal(respBody, &rr); err != nil {
		return fmt.Errorf("stripe: failed to parse refund response: %w", err)
	}

	if rr.Error != nil {
		return fmt.Errorf("stripe: %s: %s", rr.Error.Type, rr.Error.Message)
	}

	return nil
}

func (p *StripeProcessor) GetChargeStatus(ctx context.Context, chargeID string) (*ChargeStatus, error) {
	if p.secretKey == "" {
		return nil, fmt.Errorf("stripe: secret key not configured")
	}

	endpoint := fmt.Sprintf("https://api.stripe.com/v1/payment_intents/%s",
		url.PathEscape(chargeID))

	respBody, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var sr stripeResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return nil, fmt.Errorf("stripe: failed to parse status response: %w", err)
	}

	if sr.Error != nil {
		return nil, fmt.Errorf("stripe: %s: %s", sr.Error.Type, sr.Error.Message)
	}

	return &ChargeStatus{
		ChargeID: sr.ID,
		Status:   mapStripeStatus(sr.Status),
		Amount:   sr.Amount,
	}, nil
}

func (p *StripeProcessor) ListCharges(ctx context.Context, filter ChargeFilter) ([]ChargeStatus, error) {
	if p.secretKey == "" {
		return nil, fmt.Errorf("stripe: secret key not configured")
	}

	params := url.Values{}
	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	params.Set("limit", fmt.Sprintf("%d", limit))

	endpoint := "https://api.stripe.com/v1/payment_intents?" + params.Encode()

	respBody, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var lr stripeListResponse
	if err := json.Unmarshal(respBody, &lr); err != nil {
		return nil, fmt.Errorf("stripe: failed to parse list response: %w", err)
	}

	if lr.Error != nil {
		return nil, fmt.Errorf("stripe: %s: %s", lr.Error.Type, lr.Error.Message)
	}

	var charges []ChargeStatus
	for _, item := range lr.Data {
		charges = append(charges, ChargeStatus{
			ChargeID: item.ID,
			Status:   mapStripeStatus(item.Status),
			Amount:   item.Amount,
		})
	}

	return charges, nil
}

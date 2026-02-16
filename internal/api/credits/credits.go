package credits

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vfrog-ai/vfrog-cli/internal/auth"
	"github.com/vfrog-ai/vfrog-cli/internal/config"
)

// ErrInsufficientCredits is returned when the organisation doesn't have enough credits.
var ErrInsufficientCredits = errors.New("insufficient credits")

// Client calls Supabase Edge Functions for credit operations.
type Client struct {
	supabaseURL string
	apiKey      string
	cfg         *config.Config
	httpClient  *http.Client
}

// ReserveParams contains the parameters for reserving credits.
type ReserveParams struct {
	OrganisationID string
	ToolType       string // "ssat", "run_inference", "training", or "inference"
	ProcessID      string
	Units          int
}

// ReserveResult contains the result of a credit reservation.
type ReserveResult struct {
	ReservationID string  `json:"reservation_id"`
	Amount        float64 `json:"amount"`
}

// NewClient creates a new credits client from config.
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.SupabaseURL == "" {
		return nil, fmt.Errorf("supabase_url not configured")
	}
	if cfg.SupabasePublishableKey == "" {
		return nil, fmt.Errorf("supabase_publishable_key not configured")
	}

	return &Client{
		supabaseURL: cfg.SupabaseURL,
		apiKey:      cfg.SupabasePublishableKey,
		cfg:         cfg,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// ReserveCredits reserves credits before processing. Returns ErrInsufficientCredits on 402.
func (c *Client) ReserveCredits(params ReserveParams) (*ReserveResult, error) {
	accessToken, err := auth.GetValidToken(c.cfg)
	if err != nil {
		return nil, fmt.Errorf("authentication required: %w", err)
	}

	reqURL := fmt.Sprintf("%s/functions/v1/reserve-credits", c.supabaseURL)

	payload := map[string]interface{}{
		"organisation_id": params.OrganisationID,
		"tool_type":       params.ToolType,
		"process_id":      params.ProcessID,
		"units":           params.Units,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusPaymentRequired {
		return nil, ErrInsufficientCredits
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("reserve-credits failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result ReserveResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// GetUsageParams contains optional parameters for cost estimation.
type GetUsageParams struct {
	OrganisationID string
	ActionType     string // optional: "ssat", "run_inference", "training", "inference"
	Units          int    // optional: number of units for cost estimation
}

// Usage contains the organisation's usage and balance information.
type Usage struct {
	Allowed            bool    `json:"allowed"`
	Reason             string  `json:"reason,omitempty"`
	CreditsBalance     float64 `json:"credits_balance"`
	ExpiringSoon       float64 `json:"expiring_soon"`
	TotalAllocated     float64 `json:"total_allocated"`
	TotalConsumed      float64 `json:"total_consumed"`
	EstimatedActionCost float64 `json:"estimated_action_cost,omitempty"`
	PricePerUnit       float64 `json:"price_per_unit,omitempty"`
}

// GetUsage returns the organisation's usage, balance, and optional cost estimate.
func (c *Client) GetUsage(params GetUsageParams) (*Usage, error) {
	accessToken, err := auth.GetValidToken(c.cfg)
	if err != nil {
		return nil, fmt.Errorf("authentication required: %w", err)
	}

	reqURL := fmt.Sprintf("%s/functions/v1/get-usage", c.supabaseURL)

	payload := map[string]interface{}{
		"organisation_id": params.OrganisationID,
	}
	if params.ActionType != "" {
		payload["action_type"] = params.ActionType
	}
	if params.Units > 0 {
		payload["units"] = params.Units
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get-usage failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result Usage
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

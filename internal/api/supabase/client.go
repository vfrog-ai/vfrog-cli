package supabase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/vfrog/vfrog-cli/internal/auth"
	"github.com/vfrog/vfrog-cli/internal/config"
)

// Client represents a Supabase PostgREST client
type Client struct {
	baseURL    string
	apiKey     string
	cfg        *config.Config
	httpClient *http.Client
}

// NewClient creates a new Supabase client
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.SupabaseURL == "" {
		return nil, fmt.Errorf("supabase_url not configured")
	}

	if cfg.SupabasePublishableKey == "" {
		return nil, fmt.Errorf("supabase_publishable_key not configured")
	}

	return &Client{
		baseURL:    cfg.SupabaseURL,
		apiKey:     cfg.SupabasePublishableKey,
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Get performs a GET request to a Supabase table
func (c *Client) Get(table string, params map[string]string) ([]map[string]interface{}, error) {
	accessToken, err := auth.GetValidToken(c.cfg)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/rest/v1/%s", c.baseURL, table)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// Post performs a POST request to create a record
func (c *Client) Post(table string, data map[string]interface{}) (map[string]interface{}, error) {
	accessToken, err := auth.GetValidToken(c.cfg)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/rest/v1/%s", c.baseURL, table)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no data returned")
	}

	return result[0], nil
}

// Patch performs a PATCH request to update a record
func (c *Client) Patch(table string, id string, data map[string]interface{}) error {
	accessToken, err := auth.GetValidToken(c.cfg)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/rest/v1/%s?id=eq.%s", c.baseURL, table, url.QueryEscape(id))

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	req, err := http.NewRequest("PATCH", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Delete performs a DELETE request
func (c *Client) Delete(table string, id string) error {
	accessToken, err := auth.GetValidToken(c.cfg)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/rest/v1/%s?id=eq.%s", c.baseURL, table, url.QueryEscape(id))

	req, err := http.NewRequest("DELETE", reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

package vfrogapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vfrog/vfrog-cli/internal/config"
)

// Client represents a vfrog API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new vfrog API client
func NewClient(cfg *config.Config, apiKeyOverride string) (*Client, error) {
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = "https://api.vfrog.ai"
	}

	apiKey := apiKeyOverride
	if apiKey == "" {
		apiKey = os.Getenv("VFROG_API_KEY")
	}
	if apiKey == "" {
		apiKey = cfg.APIKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key not configured. Set it via --api-key flag, VFROG_API_KEY env var, or config file")
	}

	return &Client{
		baseURL:    apiURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// InferenceRequest represents a CV inference request
type InferenceRequest struct {
	ImageURL    string `json:"image_url,omitempty"`
	ImageBase64 string `json:"image_base64,omitempty"`
	ExternalID  string `json:"external_id,omitempty"`
}

// InferenceResponse represents a CV inference response
type InferenceResponse struct {
	Success   bool                     `json:"success"`
	RequestID string                   `json:"request_id"`
	ImageURL  string                   `json:"image_url"`
	Status    string                   `json:"status"`
	Error     string                   `json:"error,omitempty"`
	Results   []map[string]interface{} `json:"results,omitempty"`
}

// RunInference runs a CV inference request
func (c *Client) RunInference(req InferenceRequest) (*InferenceResponse, error) {
	url := fmt.Sprintf("%s/v1/cv/requests/sync", c.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
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

	var result InferenceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// BatchInferenceRequest represents a batch CV inference request
type BatchInferenceRequest struct {
	ImageURLs  []string `json:"image_urls"`
	ExternalID string   `json:"external_id,omitempty"`
}

// BatchInferenceResponse represents a batch CV inference response
type BatchInferenceResponse struct {
	Success bool                `json:"success"`
	Results []InferenceResponse `json:"results,omitempty"`
	Error   string              `json:"error,omitempty"`
}

// RunBatchInference runs a batch CV inference request (max 10 URLs)
func (c *Client) RunBatchInference(req BatchInferenceRequest) (*BatchInferenceResponse, error) {
	reqURL := fmt.Sprintf("%s/v1/cv/requests/batch", c.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
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

	var result BatchInferenceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// GetRequestStatus gets the status of a CV inference request
func (c *Client) GetRequestStatus(requestID string) (*InferenceResponse, error) {
	reqURL := fmt.Sprintf("%s/v1/cv/requests/%s", c.baseURL, requestID)

	httpReq, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
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

	var result InferenceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// FeedbackRequest represents a CV feedback request
type FeedbackRequest struct {
	RequestID string `json:"request_id"`
	Rating    int    `json:"rating"`
}

// SubmitFeedback submits feedback for a CV inference request
func (c *Client) SubmitFeedback(req FeedbackRequest) (map[string]interface{}, error) {
	reqURL := fmt.Sprintf("%s/v1/cv/feedback", c.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
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

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]interface{}{"status": "success"}, nil
	}

	return result, nil
}

// EncodeImageFile reads a local image file and returns base64 encoded data URL
func EncodeImageFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	mimeType := "image/jpeg"
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".png") {
		mimeType = "image/png"
	} else if strings.HasSuffix(lower, ".webp") {
		mimeType = "image/webp"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
}

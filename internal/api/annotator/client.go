package annotator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/vfrog/vfrog-cli/internal/config"
)

// Client represents an annotator API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new annotator client
func NewClient(cfg *config.Config) (*Client, error) {
	annotatorURL := cfg.AnnotatorURL
	if annotatorURL == "" {
		annotatorURL = "https://annotator.vfrog.ai"
	}

	apiKey := os.Getenv("VFROG_ANNOTATOR_API_KEY")
	if apiKey == "" {
		apiKey = cfg.AnnotatorAPIKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("annotator API key not configured. Set VFROG_ANNOTATOR_API_KEY env var or config file")
	}

	return &Client{
		baseURL:    annotatorURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

// ImageRef represents an image reference for batch submission
type ImageRef struct {
	ID       string `json:"id"`
	ImageURL string `json:"image_url"`
}

// ProductImageRef represents a product image reference with label
type ProductImageRef struct {
	ID       string `json:"id"`
	ImageURL string `json:"image_url"`
	Label    string `json:"label"`
}

// BatchSubmitParams represents the parameters for batch submission
type BatchSubmitParams struct {
	ProjectIterationID string          `json:"project_iteration_id"`
	ProductImage       ProductImageRef `json:"product_image"`
	DatasetImages      []ImageRef      `json:"dataset_images"`
	CallbackURL        string          `json:"callback_url,omitempty"`
}

// SubmitBatch submits a batch annotation task
func (c *Client) SubmitBatch(params BatchSubmitParams) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/batch-submit", c.baseURL)

	jsonData, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// Response may be empty, which is fine
		return map[string]interface{}{}, nil
	}

	return result, nil
}


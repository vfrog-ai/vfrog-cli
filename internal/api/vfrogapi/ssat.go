package vfrogapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vfrog/vfrog-cli/internal/config"
)

// SSATClient represents a client for SSAT operations via the API project
type SSATClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewSSATClient creates a new SSAT client
func NewSSATClient(cfg *config.Config, apiKey string) (*SSATClient, error) {
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = cfg.APIProjectBaseURL
	}
	if apiURL == "" {
		return nil, fmt.Errorf("API URL not configured")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	return &SSATClient{
		baseURL:    apiURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

// ProductImageRef represents a product image reference
type ProductImageRef struct {
	ID       string `json:"id"`
	ImageURL string `json:"image_url"`
	Label    string `json:"label"`
}

// DatasetImageRef represents a dataset image reference for batch submit
type DatasetImageRef struct {
	ID       string `json:"id"`
	ImageURL string `json:"image_url"`
}

// BatchSubmitParams represents the parameters for SSAT batch submission
type BatchSubmitParams struct {
	ProjectIterationID string            `json:"project_iteration_id"`
	ProductImage       ProductImageRef   `json:"product_image"`
	DatasetImages      []DatasetImageRef `json:"dataset_images"`
	CallbackURL        string            `json:"callback_url,omitempty"`
}

// BatchSubmit submits an SSAT batch to the annotator via the API project
func (c *SSATClient) BatchSubmit(params BatchSubmitParams) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/ssat/batch-submit", c.baseURL)

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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// Response may be empty
		return map[string]interface{}{}, nil
	}

	return result, nil
}

// InferenceImageRef represents a dataset image for inference
type InferenceImageRef struct {
	ID      string `json:"id"`
	FileURL string `json:"file_url"`
}

// AnnotatedImageRef represents an annotated image for inference
type AnnotatedImageRef struct {
	DatasetImagesID string        `json:"dataset_images_id"`
	Annotation      []interface{} `json:"annotation"`
}

// RunInferenceParams represents the parameters for SSAT run inference
type RunInferenceParams struct {
	ProjectIterationID string              `json:"project_iteration_id"`
	ModelPath          string              `json:"model_path"`
	DatasetImages      []InferenceImageRef `json:"dataset_images"`
	AnnotatedImages    []AnnotatedImageRef `json:"annotated_images"`
	CallbackURL        string              `json:"callback_url,omitempty"`
}

// RunInference submits an inference task via the API project
func (c *SSATClient) RunInference(params RunInferenceParams) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/ssat/run-inference", c.baseURL)

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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// TrainParams represents the parameters for model training
type TrainParams struct {
	ProjectIterationID string              `json:"project_iteration_id"`
	OrganisationID     string              `json:"organisation_id"`
	ProjectName        string              `json:"project_name"`
	ProductID          string              `json:"product_id"`
	DatasetImages      []InferenceImageRef `json:"dataset_images"`
	AnnotatedImages    []AnnotatedImageRef `json:"annotated_images"`
	CallbackURL        string              `json:"callback_url,omitempty"`
}

// Train submits a training task via the API project
func (c *SSATClient) Train(params TrainParams) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/train", c.baseURL)

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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}


package inference

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

// Client represents an inference server client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new inference client
func NewClient(cfg *config.Config, apiKeyOverride string) (*Client, error) {
	inferenceURL := cfg.InferenceURL
	if inferenceURL == "" {
		inferenceURL = "https://inference.vfrog.ai"
	}

	apiKey := apiKeyOverride
	if apiKey == "" {
		apiKey = os.Getenv("VFROG_INFERENCE_API_KEY")
	}
	if apiKey == "" {
		apiKey = cfg.InferenceAPIKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("inference API key not configured. Set it via --api-key flag, VFROG_INFERENCE_API_KEY env var, or config file")
	}

	return &Client{
		baseURL:    inferenceURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

// SubmitTrainingTask submits a training task to the inference server
func (c *Client) SubmitTrainingTask(payload map[string]interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/train", c.baseURL)

	jsonData, err := json.Marshal(payload)
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
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

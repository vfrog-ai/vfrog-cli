package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vfrog/vfrog-cli/internal/config"
)

// SignedURLResponse represents the response from the s3-storage-proxy edge function
type SignedURLResponse struct {
	SignedURL string `json:"signedUrl"`
	FileKey  string `json:"fileKey"`
	FileURL  string `json:"fileUrl"`
}

// GetSignedUploadURL requests a signed upload URL from Supabase edge function
func GetSignedUploadURL(cfg *config.Config, accessToken, bucket, filename, contentType string) (*SignedURLResponse, error) {
	if cfg.SupabaseURL == "" {
		return nil, fmt.Errorf("supabase_url not configured")
	}

	edgeFunctionURL := fmt.Sprintf("%s/functions/v1/s3-storage-proxy", cfg.SupabaseURL)

	payload := map[string]interface{}{
		"action":      "getSignedUploadUrl",
		"bucket":      bucket,
		"filename":    filename,
		"contentType": contentType,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", edgeFunctionURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("apikey", cfg.SupabasePublishableKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to get signed URL (status %d): %s", resp.StatusCode, string(body))
	}

	var result SignedURLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// UploadToS3 uploads file data to S3 using a signed URL
func UploadToS3(signedURL string, data []byte, contentType string) error {
	req, err := http.NewRequest("PUT", signedURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// UploadFile uploads a local file to S3 and returns the file URL
func UploadFile(cfg *config.Config, accessToken, bucket, filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	filename := filepath.Base(filePath)
	contentType := DetectContentType(filename)

	signedResp, err := GetSignedUploadURL(cfg, accessToken, bucket, filename, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to get signed URL: %w", err)
	}

	if err := UploadToS3(signedResp.SignedURL, data, contentType); err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return signedResp.FileURL, nil
}

// DetectContentType detects MIME type from filename
func DetectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	// Try standard MIME detection
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		return mimeType
	}

	// Fallback for common image types
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}

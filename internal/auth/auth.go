package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vfrog/vfrog-cli/internal/config"
)

// Login performs Supabase password authentication
func Login(email, password string, cfg *config.Config) (*config.Auth, error) {
	if cfg.SupabaseURL == "" {
		return nil, fmt.Errorf("supabase_url not configured. This binary may not have been built with credentials")
	}

	url := fmt.Sprintf("%s/auth/v1/token?grant_type=password", cfg.SupabaseURL)

	payload := map[string]string{
		"email":    email,
		"password": password,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return &config.Auth{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// RefreshToken refreshes the access token using the refresh token
func RefreshToken(refreshToken string, cfg *config.Config) (*config.Auth, error) {
	if cfg.SupabaseURL == "" {
		return nil, fmt.Errorf("supabase_url not configured")
	}

	url := fmt.Sprintf("%s/auth/v1/token?grant_type=refresh_token", cfg.SupabaseURL)

	payload := map[string]string{
		"refresh_token": refreshToken,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refresh payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return &config.Auth{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// GetValidToken returns a valid access token, refreshing if necessary
func GetValidToken(cfg *config.Config) (string, error) {
	if cfg.Auth == nil {
		return "", fmt.Errorf("not authenticated. Run: vfrog login")
	}

	// Check if token is expired (with 5 minute buffer)
	if time.Now().Add(5 * time.Minute).After(cfg.Auth.ExpiresAt) {
		newAuth, err := RefreshToken(cfg.Auth.RefreshToken, cfg)
		if err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}

		cfg.Auth = newAuth
		if err := config.Save(cfg); err != nil {
			return "", fmt.Errorf("failed to save refreshed token: %w", err)
		}
	}

	return cfg.Auth.AccessToken, nil
}

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Build-time variables (set via ldflags)
var (
	// DefaultSupabaseURL is set at build time for each environment binary
	DefaultSupabaseURL = ""
	// DefaultSupabasePublishableKey is set at build time for each environment binary
	DefaultSupabasePublishableKey = ""
	// DefaultAPIURL is set at build time for each environment binary
	DefaultAPIURL = ""
	// DefaultInferenceURL is set at build time for each environment binary
	DefaultInferenceURL = ""
	// DefaultAPIProjectBaseURL is set at build time for each environment binary
	DefaultAPIProjectBaseURL = ""
	// DefaultPlatformHost is set at build time for each environment binary
	DefaultPlatformHost = ""
	// Environment name (dev, staging, production) - informational
	Environment = "production"
	// Version is set at build time
	Version = "dev"
)

var (
	configDir  = filepath.Join(os.Getenv("HOME"), ".vfrog")
	configPath = filepath.Join(configDir, "config.json")
	mu         sync.RWMutex
)

// Config represents the CLI configuration
type Config struct {
	SupabaseURL          string    `json:"supabase_url,omitempty"`
	SupabasePublishableKey string  `json:"supabase_publishable_key,omitempty"`
	OrganisationID       string    `json:"organisation_id,omitempty"`
	ProjectID            string    `json:"project_id,omitempty"`
	APIURL               string    `json:"api_url,omitempty"`
	InferenceURL         string    `json:"inference_url,omitempty"`
	InferenceAPIKey      string    `json:"inference_api_key,omitempty"`
	APIProjectBaseURL    string    `json:"api_project_base_url,omitempty"`
	PlatformHost         string    `json:"platform_host,omitempty"`
	APIKey               string    `json:"api_key,omitempty"`
	Auth                 *Auth    `json:"auth,omitempty"`
}

// Auth represents authentication tokens
type Auth struct {
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// Load reads the configuration from disk and applies build-time defaults
func Load() (*Config, error) {
	mu.RLock()
	defer mu.RUnlock()

	cfg := &Config{}
	
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Apply build-time defaults
		applyDefaults(cfg)
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if len(data) == 0 {
		applyDefaults(cfg)
		return cfg, nil
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults for any missing values
	applyDefaults(cfg)
	return cfg, nil
}

// applyDefaults fills in build-time default values for empty fields
func applyDefaults(cfg *Config) {
	if cfg.SupabaseURL == "" {
		cfg.SupabaseURL = DefaultSupabaseURL
	}
	if cfg.SupabasePublishableKey == "" {
		cfg.SupabasePublishableKey = DefaultSupabasePublishableKey
	}
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	if cfg.InferenceURL == "" {
		cfg.InferenceURL = DefaultInferenceURL
	}
	if cfg.APIProjectBaseURL == "" {
		cfg.APIProjectBaseURL = DefaultAPIProjectBaseURL
	}
	if cfg.PlatformHost == "" {
		cfg.PlatformHost = DefaultPlatformHost
	}
}

// Save writes the configuration to disk
func Save(cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// SetOrganisationID sets the organisation ID and clears project ID if it changes
func (c *Config) SetOrganisationID(orgID string) error {
	oldOrgID := c.OrganisationID
	c.OrganisationID = orgID
	
	// If organisation changed, clear project_id
	if oldOrgID != "" && oldOrgID != orgID {
		c.ProjectID = ""
	}
	
	return Save(c)
}

// SetProjectID sets the project ID
func (c *Config) SetProjectID(projectID string) error {
	c.ProjectID = projectID
	return Save(c)
}

// RequireOrganisationID returns an error if organisation_id is not set
func (c *Config) RequireOrganisationID() error {
	if c.OrganisationID == "" {
		return fmt.Errorf("organisation_id is required. Run: vfrog config set organisation --organisation_id <id>")
	}
	return nil
}

// RequireProjectID returns an error if project_id is not set
func (c *Config) RequireProjectID() error {
	if err := c.RequireOrganisationID(); err != nil {
		return err
	}
	if c.ProjectID == "" {
		return fmt.Errorf("project_id is required. Run: vfrog config set project --project_id <id>")
	}
	return nil
}


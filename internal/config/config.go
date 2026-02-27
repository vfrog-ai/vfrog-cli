package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// DefaultAPIProjectBaseURL is set at build time for each environment binary
	DefaultAPIProjectBaseURL = ""
	// DefaultPlatformHost is set at build time for each environment binary
	DefaultPlatformHost = ""
	// DefaultInferenceURL is set at build time for each environment binary
	DefaultInferenceURL = ""
	// DefaultAnnotatorURL is set at build time for each environment binary
	DefaultAnnotatorURL = ""
	// Environment name (dev, staging, production) - set at build time
	Environment = ""
	// Version is set at build time (e.g., v0.1.0)
	Version = ""
)

var (
	configDir  = filepath.Join(os.Getenv("HOME"), ".vfrog")
	mu         sync.RWMutex
)

// getConfigPath returns the config file path based on the environment
func getConfigPath() string {
	env := Environment
	if env == "" {
		env = "default"
	}
	configFile := fmt.Sprintf("config-%s.json", env)
	return filepath.Join(configDir, configFile)
}

// Config represents the CLI configuration
type Config struct {
	SupabaseURL            string `json:"supabase_url,omitempty"`
	SupabasePublishableKey string `json:"supabase_publishable_key,omitempty"`
	OrganisationID         string `json:"organisation_id,omitempty"`
	ProjectID              string `json:"project_id,omitempty"`
	ObjectID               string `json:"object_id,omitempty"`
	APIURL            string `json:"api_url,omitempty"`
	APIProjectBaseURL string `json:"api_project_base_url,omitempty"`
	PlatformHost           string `json:"platform_host,omitempty"`
	PlanType               string `json:"plan_type,omitempty"`
	APIKey                 string `json:"api_key,omitempty"`
	Auth                   *Auth  `json:"auth,omitempty"`
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
	configPath := getConfigPath()
	
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
// For local environment, always use build-time defaults for API URLs to ensure correct localhost endpoints
func applyDefaults(cfg *Config) {
	if cfg.SupabaseURL == "" {
		cfg.SupabaseURL = DefaultSupabaseURL
	}
	if cfg.SupabasePublishableKey == "" {
		cfg.SupabasePublishableKey = DefaultSupabasePublishableKey
	}
	// For local environment, always use build-time defaults for API URLs
	if Environment == "local" {
		if DefaultAPIURL != "" {
			cfg.APIURL = DefaultAPIURL
		}
		if DefaultAPIProjectBaseURL != "" {
			cfg.APIProjectBaseURL = DefaultAPIProjectBaseURL
		}
	} else {
		// For other environments, only set if empty
		if cfg.APIURL == "" {
			cfg.APIURL = DefaultAPIURL
		}
		if cfg.APIProjectBaseURL == "" {
			cfg.APIProjectBaseURL = DefaultAPIProjectBaseURL
		}
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

	configPath := getConfigPath()
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

// SetOrganisationIDWithPlan sets the organisation ID and plan type, clearing project ID if org changes
func (c *Config) SetOrganisationIDWithPlan(orgID, planType string) error {
	oldOrgID := c.OrganisationID
	c.OrganisationID = orgID
	c.PlanType = planType

	// If organisation changed, clear project_id
	if oldOrgID != "" && oldOrgID != orgID {
		c.ProjectID = ""
	}

	return Save(c)
}

// IsFreePlan returns true if the cached plan type is FREE
func (c *Config) IsFreePlan() bool {
	return strings.EqualFold(c.PlanType, "FREE")
}

// FreePlanError returns an error indicating a feature requires a paid plan
func (c *Config) FreePlanError(feature string) error {
	host := c.PlatformHost
	if host == "" {
		host = DefaultPlatformHost
	}
	return fmt.Errorf("%s requires a paid plan. You are on the FREE plan.\nUpgrade at: %s/billing", feature, host)
}

// SetProjectID sets the project ID and clears object_id if project changes
func (c *Config) SetProjectID(projectID string) error {
	oldProjectID := c.ProjectID
	c.ProjectID = projectID

	// If project changed, clear object_id
	if oldProjectID != "" && oldProjectID != projectID {
		c.ObjectID = ""
	}

	return Save(c)
}

// SetObjectID sets the object (product image) ID
func (c *Config) SetObjectID(objectID string) error {
	c.ObjectID = objectID
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

// RequireObjectID returns an error if object_id is not set
func (c *Config) RequireObjectID() error {
	if err := c.RequireProjectID(); err != nil {
		return err
	}
	if c.ObjectID == "" {
		return fmt.Errorf("object_id is required. Run: vfrog config set object --object_id <id>")
	}
	return nil
}


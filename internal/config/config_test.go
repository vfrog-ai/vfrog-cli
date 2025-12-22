package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSave(t *testing.T) {
	// Use a temporary directory for testing
	originalDir := configDir
	originalPath := configPath
	defer func() {
		configDir = originalDir
		configPath = originalPath
	}()

	tmpDir := t.TempDir()
	configDir = tmpDir
	configPath = filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		SupabaseURL:          "https://test.supabase.co",
		SupabasePublishableKey: "test-key",
		OrganisationID:      "org-123",
		ProjectID:           "proj-456",
		Auth: &Auth{
			AccessToken:  "token-123",
			RefreshToken: "refresh-123",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.SupabaseURL != cfg.SupabaseURL {
		t.Errorf("SupabaseURL mismatch: got %v, want %v", loaded.SupabaseURL, cfg.SupabaseURL)
	}

	if loaded.OrganisationID != cfg.OrganisationID {
		t.Errorf("OrganisationID mismatch: got %v, want %v", loaded.OrganisationID, cfg.OrganisationID)
	}
}

func TestSetOrganisationIDClearsProjectID(t *testing.T) {
	originalDir := configDir
	originalPath := configPath
	defer func() {
		configDir = originalDir
		configPath = originalPath
	}()

	tmpDir := t.TempDir()
	configDir = tmpDir
	configPath = filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		OrganisationID: "org-1",
		ProjectID:      "proj-1",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := cfg.SetOrganisationID("org-2"); err != nil {
		t.Fatalf("SetOrganisationID failed: %v", err)
	}

	if cfg.ProjectID != "" {
		t.Errorf("ProjectID should be cleared when organisation changes, got %v", cfg.ProjectID)
	}
}

func TestRequireOrganisationID(t *testing.T) {
	cfg := &Config{}
	if err := cfg.RequireOrganisationID(); err == nil {
		t.Error("RequireOrganisationID should fail when organisation_id is not set")
	}

	cfg.OrganisationID = "org-1"
	if err := cfg.RequireOrganisationID(); err != nil {
		t.Errorf("RequireOrganisationID should succeed when organisation_id is set: %v", err)
	}
}

func TestRequireProjectID(t *testing.T) {
	cfg := &Config{}
	if err := cfg.RequireProjectID(); err == nil {
		t.Error("RequireProjectID should fail when project_id is not set")
	}

	cfg.OrganisationID = "org-1"
	if err := cfg.RequireProjectID(); err == nil {
		t.Error("RequireProjectID should fail when only organisation_id is set")
	}

	cfg.ProjectID = "proj-1"
	if err := cfg.RequireProjectID(); err != nil {
		t.Errorf("RequireProjectID should succeed when both are set: %v", err)
	}
}


package cmd

import (
	"fmt"
	"regexp"
	"time"

	"github.com/spf13/cobra"

	"github.com/vfrog-ai/vfrog-cli/internal/api/supabase"
	"github.com/vfrog-ai/vfrog-cli/internal/config"
	"github.com/vfrog-ai/vfrog-cli/internal/output"
)

// UUID regex pattern
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateUUID checks if a string is a valid UUID format
func validateUUID(id string) error {
	if !uuidPattern.MatchString(id) {
		return fmt.Errorf("invalid UUID format: %s", id)
	}
	return nil
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  `Manage CLI configuration settings such as organisation and project IDs.`,
}

// configSetCmd represents the config set command
var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a configuration value",
	Long:  `Set a configuration value such as organisation_id or project_id.`,
}

// configShowCmd represents the config show command
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Show current CLI configuration including environment and credentials status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		version := config.Version
		if version == "" {
			version = "local-build"
		}
		env := config.Environment
		if env == "" {
			env = "local"
		}

		info := map[string]interface{}{
			"version":         version,
			"environment":     env,
			"organisation_id": cfg.OrganisationID,
			"project_id":      cfg.ProjectID,
			"object_id":       cfg.ObjectID,
			"supabase_url":    cfg.SupabaseURL,
			"api_url":         cfg.APIURL,
			"platform_host":   cfg.PlatformHost,
			"authenticated":   cfg.Auth != nil && cfg.Auth.AccessToken != "",
		}

		if jsonOutput {
			return output.PrintJSON(info)
		}

		fmt.Printf("Version:         %s\n", version)
		fmt.Printf("Environment:     %s\n", env)
		fmt.Printf("Organisation ID: %s\n", cfg.OrganisationID)
		fmt.Printf("Project ID:      %s\n", cfg.ProjectID)
		fmt.Printf("Object ID:       %s\n", cfg.ObjectID)
		fmt.Printf("Supabase URL:    %s\n", cfg.SupabaseURL)
		fmt.Printf("API URL:         %s\n", cfg.APIURL)
		fmt.Printf("Platform Host:   %s\n", cfg.PlatformHost)
		if cfg.Auth != nil && cfg.Auth.AccessToken != "" {
			fmt.Printf("Authenticated:   yes (expires: %s)\n", cfg.Auth.ExpiresAt.Format(time.RFC3339))
		} else {
			fmt.Printf("Authenticated:   no\n")
		}

		return nil
	},
}

// configSetOrganisationCmd represents the config set organisation command
var configSetOrganisationCmd = &cobra.Command{
	Use:   "organisation",
	Short: "Set the default organisation ID",
	Long:  `Set the default organisation ID for subsequent commands. This will clear the project_id if it changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID, _ := cmd.Flags().GetString("organisation_id")
		if orgID == "" {
			return fmt.Errorf("organisation_id is required")
		}

		// Validate UUID format
		if err := validateUUID(orgID); err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Verify organisation exists and user has access
		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		// Check if user has access to this organisation via organisation_user
		orgUsers, err := client.Get("organisation_user", map[string]string{
			"organisation_id": fmt.Sprintf("eq.%s", orgID),
			"select":          "organisation_id,organisation:organisation_id(id,name)",
		})
		if err != nil {
			return fmt.Errorf("failed to verify organisation: %w", err)
		}

		if len(orgUsers) == 0 {
			return fmt.Errorf("organisation not found or you don't have access: %s", orgID)
		}

		// Get the organisation name for confirmation
		var orgName string
		if org, ok := orgUsers[0]["organisation"].(map[string]interface{}); ok {
			if name, ok := org["name"].(string); ok {
				orgName = name
			}
		}

		if err := cfg.SetOrganisationID(orgID); err != nil {
			return fmt.Errorf("failed to set organisation_id: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{
				"organisation_id":   orgID,
				"organisation_name": orgName,
				"status":            "success",
			})
		}

		output.PrintSuccess(fmt.Sprintf("Set organisation to: %s (%s)", orgName, orgID))
		return nil
	},
}

// configSetProjectCmd represents the config set project command
var configSetProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Set the default project ID",
	Long:  `Set the default project ID for subsequent commands. Requires organisation_id to be set first.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID, _ := cmd.Flags().GetString("project_id")
		if projectID == "" {
			return fmt.Errorf("project_id is required")
		}

		// Validate UUID format
		if err := validateUUID(projectID); err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		// Verify project exists in the current organisation
		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		projects, err := client.Get("projects", map[string]string{
			"id":              fmt.Sprintf("eq.%s", projectID),
			"organisation_id": fmt.Sprintf("eq.%s", cfg.OrganisationID),
			"select":          "id,title",
		})
		if err != nil {
			return fmt.Errorf("failed to verify project: %w", err)
		}

		if len(projects) == 0 {
			return fmt.Errorf("project not found in current organisation: %s", projectID)
		}

		// Get the project title for confirmation
		var projectTitle string
		if title, ok := projects[0]["title"].(string); ok {
			projectTitle = title
		}

		if err := cfg.SetProjectID(projectID); err != nil {
			return fmt.Errorf("failed to set project_id: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{
				"project_id":    projectID,
				"project_title": projectTitle,
				"status":        "success",
			})
		}

		output.PrintSuccess(fmt.Sprintf("Set project to: %s (%s)", projectTitle, projectID))
		return nil
	},
}

// configSetObjectCmd represents the config set object command
var configSetObjectCmd = &cobra.Command{
	Use:   "object",
	Short: "Set the default object (product image) ID",
	Long:  `Set the default object ID for subsequent commands. Requires project_id to be set first.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		objectID, _ := cmd.Flags().GetString("object_id")
		if objectID == "" {
			return fmt.Errorf("object_id is required")
		}

		// Validate UUID format
		if err := validateUUID(objectID); err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireProjectID(); err != nil {
			return err
		}

		// Verify object exists in the current project
		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		objects, err := client.Get("product_images", map[string]string{
			"id":         fmt.Sprintf("eq.%s", objectID),
			"project_id": fmt.Sprintf("eq.%s", cfg.ProjectID),
			"select":     "id,label,filename",
		})
		if err != nil {
			return fmt.Errorf("failed to verify object: %w", err)
		}

		if len(objects) == 0 {
			return fmt.Errorf("object not found in current project: %s", objectID)
		}

		// Get the object label for confirmation
		var objectLabel string
		if label, ok := objects[0]["label"].(string); ok && label != "" {
			objectLabel = label
		} else if filename, ok := objects[0]["filename"].(string); ok {
			objectLabel = filename
		}

		if err := cfg.SetObjectID(objectID); err != nil {
			return fmt.Errorf("failed to set object_id: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{
				"object_id":    objectID,
				"object_label": objectLabel,
				"status":       "success",
			})
		}

		output.PrintSuccess(fmt.Sprintf("Set object to: %s (%s)", objectLabel, objectID))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
	configSetCmd.AddCommand(configSetOrganisationCmd)
	configSetCmd.AddCommand(configSetProjectCmd)
	configSetCmd.AddCommand(configSetObjectCmd)

	configSetOrganisationCmd.Flags().String("organisation_id", "", "Organisation ID")
	configSetProjectCmd.Flags().String("project_id", "", "Project ID")
	configSetObjectCmd.Flags().String("object_id", "", "Object (product image) ID")
}

package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/config"
	"github.com/vfrog/vfrog-cli/internal/output"
)

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

		info := map[string]interface{}{
			"version":          config.Version,
			"environment":      config.Environment,
			"organisation_id":  cfg.OrganisationID,
			"project_id":       cfg.ProjectID,
			"supabase_url":     cfg.SupabaseURL,
			"api_url":          cfg.APIURL,
			"inference_url":    cfg.InferenceURL,
			"platform_host":    cfg.PlatformHost,
			"authenticated":    cfg.Auth != nil && cfg.Auth.AccessToken != "",
		}

		if jsonOutput {
			return output.PrintJSON(info)
		}

		fmt.Printf("Version:         %s\n", config.Version)
		fmt.Printf("Environment:     %s\n", config.Environment)
		fmt.Printf("Organisation ID: %s\n", cfg.OrganisationID)
		fmt.Printf("Project ID:      %s\n", cfg.ProjectID)
		fmt.Printf("Supabase URL:    %s\n", cfg.SupabaseURL)
		fmt.Printf("API URL:         %s\n", cfg.APIURL)
		fmt.Printf("Inference URL:   %s\n", cfg.InferenceURL)
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

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.SetOrganisationID(orgID); err != nil {
			return fmt.Errorf("failed to set organisation_id: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{"organisation_id": orgID, "status": "success"})
		}

		output.PrintSuccess(fmt.Sprintf("Set organisation_id to %s", orgID))
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

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		if err := cfg.SetProjectID(projectID); err != nil {
			return fmt.Errorf("failed to set project_id: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{"project_id": projectID, "status": "success"})
		}

		output.PrintSuccess(fmt.Sprintf("Set project_id to %s", projectID))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
	configSetCmd.AddCommand(configSetOrganisationCmd)
	configSetCmd.AddCommand(configSetProjectCmd)

	configSetOrganisationCmd.Flags().String("organisation_id", "", "Organisation ID")
	configSetProjectCmd.Flags().String("project_id", "", "Project ID")
}

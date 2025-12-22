package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/api/supabase"
	"github.com/vfrog/vfrog-cli/internal/auth"
	"github.com/vfrog/vfrog-cli/internal/config"
	"github.com/vfrog/vfrog-cli/internal/output"
)

// projectsCmd represents the projects command
var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage projects",
	Long:  `Manage projects in your organisation.`,
}

// projectsListCmd represents the projects list command
var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Long:  `List all projects in the configured organisation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		projects, err := client.Get("projects", map[string]string{
			"organisation_id": fmt.Sprintf("eq.%s", cfg.OrganisationID),
			"select":          "id,title,description,created_at",
		})
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(projects)
		}

		fmt.Println("Projects:")
		for _, p := range projects {
			fmt.Printf("  - %s (ID: %v)\n", p["title"], p["id"])
		}

		return nil
	},
}

// projectsCreateCmd represents the projects create command
var projectsCreateCmd = &cobra.Command{
	Use:   "create [project_name]",
	Short: "Create a new project",
	Long:  `Create a new project in the configured organisation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		// Get current user ID from JWT token
		accessToken, err := auth.GetValidToken(cfg)
		if err != nil {
			return fmt.Errorf("failed to get access token: %w", err)
		}
		userID, err := supabase.DecodeJWT(accessToken)
		if err != nil {
			return fmt.Errorf("failed to get user ID: %w", err)
		}

		projectData := map[string]interface{}{
			"title":           projectName,
			"organisation_id": cfg.OrganisationID,
			"user_id":         userID,
		}

		project, err := client.Post("projects", projectData)
		if err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(project)
		}

		output.PrintSuccess(fmt.Sprintf("Created project: %s (ID: %v)", projectName, project["id"]))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectsCmd)
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsCreateCmd)
}


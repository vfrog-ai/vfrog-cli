package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vfrog-ai/vfrog-cli/internal/api/supabase"
	"github.com/vfrog-ai/vfrog-cli/internal/auth"
	"github.com/vfrog-ai/vfrog-cli/internal/config"
	"github.com/vfrog-ai/vfrog-cli/internal/output"
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
			"select":          "id,title,created_at",
		})
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		if jsonOutput {
			// Add selected flag to JSON output
			for i := range projects {
				if id, ok := projects[i]["id"].(string); ok && id == cfg.ProjectID {
					projects[i]["selected"] = true
				} else {
					projects[i]["selected"] = false
				}
			}
			return output.PrintJSON(projects)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return nil
		}

		table := output.NewTable("TITLE", "ID")
		for _, p := range projects {
			title := fmt.Sprintf("%v", p["title"])
			id := p["id"].(string)
			selected := id == cfg.ProjectID
			table.AddRowWithMarker(selected, title, id)
		}
		table.Print()

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

// projectsDeleteCmd represents the projects delete command
var projectsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a project",
	Long: `Delete a project by ID. This will permanently remove the project and all associated data.

Example:
  vfrog projects delete --project_id <id>
  vfrog projects delete --project_id <id> --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID, _ := cmd.Flags().GetString("project_id")
		force, _ := cmd.Flags().GetBool("force")

		if projectID == "" {
			return fmt.Errorf("--project_id is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		if !force {
			fmt.Printf("Are you sure you want to delete project %s? This cannot be undone. [y/N] ", projectID)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		if err := client.Delete("projects", projectID); err != nil {
			return fmt.Errorf("failed to delete project: %w", err)
		}

		// Clear config if deleting the active project
		if cfg.ProjectID == projectID {
			cfg.ProjectID = ""
			cfg.ObjectID = ""
			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update config: %v\n", err)
			}
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{"status": "success", "project_id": projectID})
		}

		output.PrintSuccess(fmt.Sprintf("Deleted project %s", projectID))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectsCmd)
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsCreateCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)

	projectsDeleteCmd.Flags().String("project_id", "", "Project ID to delete")
	projectsDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
}

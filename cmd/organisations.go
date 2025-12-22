package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/api/supabase"
	"github.com/vfrog/vfrog-cli/internal/config"
	"github.com/vfrog/vfrog-cli/internal/output"
)

// organisationsCmd represents the organisations command
var organisationsCmd = &cobra.Command{
	Use:   "organisations",
	Short: "List organisations",
	Long:  `List all organisations the authenticated user belongs to.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		// Get user's organisations via organisation_user join
		// RLS will automatically filter by authenticated user
		orgUsers, err := client.Get("organisation_user", map[string]string{
			"select": "organisation_id,organisation(*)",
		})
		if err != nil {
			return fmt.Errorf("failed to get organisations: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(orgUsers)
		}

		// Simple table output
		fmt.Println("Organisations:")
		for _, ou := range orgUsers {
			if org, ok := ou["organisation"].(map[string]interface{}); ok {
				if name, ok := org["name"].(string); ok {
					fmt.Printf("  - %s (ID: %v)\n", name, ou["organisation_id"])
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(organisationsCmd)
}


package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vfrog-ai/vfrog-cli/internal/api/supabase"
	"github.com/vfrog-ai/vfrog-cli/internal/config"
	"github.com/vfrog-ai/vfrog-cli/internal/output"
)

// organisationsCmd represents the organisations command
var organisationsCmd = &cobra.Command{
	Use:     "organisations",
	Aliases: []string{"orgs"},
	Short:   "Manage organisations",
	Long:    `Manage organisations the authenticated user belongs to.`,
}

// organisationsListCmd represents the organisations list command
var organisationsListCmd = &cobra.Command{
	Use:   "list",
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

		// Query with plan join: organisation.plan:plan_id(type)
		orgUsers, err := client.Get("organisation_user", map[string]string{
			"select": "organisation_id,organisation:organisation_id(id,name,plan:plan_id(type))",
		})
		if err != nil {
			return fmt.Errorf("failed to get organisations: %w", err)
		}

		// Deduplicate by organisation_id
		seen := make(map[string]bool)
		var uniqueOrgs []map[string]interface{}

		for _, ou := range orgUsers {
			orgID, ok := ou["organisation_id"].(string)
			if !ok {
				continue
			}
			if seen[orgID] {
				continue
			}
			seen[orgID] = true

			if org, ok := ou["organisation"].(map[string]interface{}); ok {
				// Extract plan type from nested plan object
				var planType interface{}
				if plan, ok := org["plan"].(map[string]interface{}); ok {
					planType = plan["type"]
				}

				uniqueOrgs = append(uniqueOrgs, map[string]interface{}{
					"id":        org["id"],
					"name":      org["name"],
					"plan_type": planType,
				})
			}
		}

		if jsonOutput {
			// Add selected flag to JSON output
			for i := range uniqueOrgs {
				if id, ok := uniqueOrgs[i]["id"].(string); ok && id == cfg.OrganisationID {
					uniqueOrgs[i]["selected"] = true
				} else {
					uniqueOrgs[i]["selected"] = false
				}
			}
			return output.PrintJSON(uniqueOrgs)
		}

		if len(uniqueOrgs) == 0 {
			fmt.Println("No organisations found.")
			return nil
		}

		table := output.NewTable("NAME", "ID", "PLAN")
		for _, org := range uniqueOrgs {
			name := fmt.Sprintf("%v", org["name"])
			id := org["id"].(string)
			planType := "-"
			if pt := org["plan_type"]; pt != nil {
				planType = fmt.Sprintf("%v", pt)
			}
			selected := id == cfg.OrganisationID
			table.AddRowWithMarker(selected, name, id, planType)
		}
		table.Print()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(organisationsCmd)
	organisationsCmd.AddCommand(organisationsListCmd)
}

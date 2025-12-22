package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/config"
	"github.com/vfrog/vfrog-cli/internal/output"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print CLI version and environment",
	Long:  `Print the vfrog CLI version and the environment it was built for.`,
	Run: func(cmd *cobra.Command, args []string) {
		info := map[string]string{
			"version":     config.Version,
			"environment": config.Environment,
		}

		if jsonOutput {
			output.PrintJSON(info)
			return
		}

		fmt.Printf("vfrog CLI %s (%s)\n", config.Version, config.Environment)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}


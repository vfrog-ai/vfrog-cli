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
		version := config.Version
		if version == "" {
			version = "local-build"
		}

		env := config.Environment
		if env == "" {
			env = "local"
		}

		info := map[string]string{
			"version":     version,
			"environment": env,
		}

		if jsonOutput {
			output.PrintJSON(info)
			return
		}

		fmt.Printf("vfrog %s (%s)\n", version, env)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

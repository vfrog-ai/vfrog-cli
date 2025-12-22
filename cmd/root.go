package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	cfgFile    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "vfrog",
	Short: "vfrog CLI - Command-line interface for the vfrog platform",
	Long: `vfrog CLI provides a fast, reliable, scriptable interface to interact
with the vfrog platform without using the web UI.

It supports both interactive login and API-key usage, and is designed for
local developer usage and CI/CD automation.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.vfrog/config.json)")
}


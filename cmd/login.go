package cmd

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/vfrog/vfrog-cli/internal/auth"
	"github.com/vfrog/vfrog-cli/internal/config"
	"github.com/vfrog/vfrog-cli/internal/output"
)

var (
	loginEmail    string
	loginPassword string
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with vfrog platform",
	Long: `Login to the vfrog platform using your email and password.
This will store your authentication tokens locally.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cfg.SupabaseURL == "" {
			return fmt.Errorf("supabase_url not configured. Please set it in your config file")
		}

		email := loginEmail
		password := loginPassword

		// Prompt for email if not provided
		if email == "" {
			fmt.Print("Email: ")
			fmt.Scanln(&email)
		}

		// Prompt for password if not provided
		if password == "" {
			fmt.Print("Password: ")
			bytePassword, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			password = string(bytePassword)
			fmt.Println()
		}

		authData, err := auth.Login(email, password, cfg.SupabaseURL)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		cfg.Auth = authData
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{"status": "success", "message": "Logged in successfully"})
		}

		output.PrintSuccess("Logged in successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "Email address")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password")
}


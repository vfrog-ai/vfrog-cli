package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/api/supabase"
	"github.com/vfrog/vfrog-cli/internal/auth"
	"github.com/vfrog/vfrog-cli/internal/config"
	"github.com/vfrog/vfrog-cli/internal/output"
)

// objectsCmd represents the objects command
var objectsCmd = &cobra.Command{
	Use:   "objects",
	Short: "Manage objects (product images)",
	Long:  `Manage objects (product images) in your project.`,
}

// objectsCreateCmd represents the objects create command
var objectsCreateCmd = &cobra.Command{
	Use:   "create [url]",
	Short: "Create a new object",
	Long:  `Create a new object (product image) from a URL. In v0.1, only URLs are supported.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageURL := args[0]
		label, _ := cmd.Flags().GetString("label")
		externalID, _ := cmd.Flags().GetString("external_id")

		if _, err := url.Parse(imageURL); err != nil {
			return fmt.Errorf("invalid URL: %s", imageURL)
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireProjectID(); err != nil {
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

		filename := imageURL
		if parsedURL, err := url.Parse(imageURL); err == nil {
			pathParts := strings.Split(parsedURL.Path, "/")
			if len(pathParts) > 0 {
				filename = pathParts[len(pathParts)-1]
			}
		}

		mimeType := "image/jpeg"
		if strings.HasSuffix(strings.ToLower(filename), ".png") {
			mimeType = "image/png"
		} else if strings.HasSuffix(strings.ToLower(filename), ".webp") {
			mimeType = "image/webp"
		}

		objectData := map[string]interface{}{
			"project_id":  cfg.ProjectID,
			"user_id":     userID,
			"file_name":   filename,
			"file_path":   imageURL,
			"file_size":   0,
			"mime_type":   mimeType,
			"processed":   false,
		}

		if label != "" {
			objectData["label"] = label
		}
		if externalID != "" {
			objectData["external_id"] = externalID
		}

		result, err := client.Post("product_images", objectData)
		if err != nil {
			return fmt.Errorf("failed to create object: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(result)
		}

		output.PrintSuccess(fmt.Sprintf("Created object: %s (ID: %v)", filename, result["id"]))
		return nil
	},
}

// objectsListCmd represents the objects list command
var objectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List objects",
	Long:  `List all objects (product images) in the configured project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireProjectID(); err != nil {
			return err
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		objects, err := client.Get("product_images", map[string]string{
			"project_id": fmt.Sprintf("eq.%s", cfg.ProjectID),
			"select":     "id,file_name,file_path,label,external_id,created_at",
		})
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(objects)
		}

		fmt.Println("Objects:")
		for _, obj := range objects {
			name := obj["file_name"]
			if label, ok := obj["label"].(string); ok && label != "" {
				name = label
			}
			fmt.Printf("  - %s (ID: %v)\n", name, obj["id"])
		}

		return nil
	},
}

// objectsDeleteCmd represents the objects delete command
var objectsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an object",
	Long:  `Delete an object (product image) by ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		objectID, _ := cmd.Flags().GetString("object_id")
		if objectID == "" {
			return fmt.Errorf("object_id is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireProjectID(); err != nil {
			return err
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		if err := client.Delete("product_images", objectID); err != nil {
			return fmt.Errorf("failed to delete object: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{"status": "success", "object_id": objectID})
		}

		output.PrintSuccess(fmt.Sprintf("Deleted object %s", objectID))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(objectsCmd)
	objectsCmd.AddCommand(objectsCreateCmd)
	objectsCmd.AddCommand(objectsListCmd)
	objectsCmd.AddCommand(objectsDeleteCmd)

	objectsCreateCmd.Flags().String("label", "", "Label for the object")
	objectsCreateCmd.Flags().String("external_id", "", "External ID for the object")
	objectsDeleteCmd.Flags().String("object_id", "", "Object ID to delete")
}

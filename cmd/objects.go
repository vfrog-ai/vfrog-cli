package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vfrog-ai/vfrog-cli/internal/api/storage"
	"github.com/vfrog-ai/vfrog-cli/internal/api/supabase"
	"github.com/vfrog-ai/vfrog-cli/internal/auth"
	"github.com/vfrog-ai/vfrog-cli/internal/config"
	"github.com/vfrog-ai/vfrog-cli/internal/output"
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
	Long: `Create a new object (product image) from a URL or local file.

Examples:
  vfrog objects create https://example.com/product.jpg --label "My Product"
  vfrog objects create --file ./product.jpg --label "My Product"`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		label, _ := cmd.Flags().GetString("label")
		externalID, _ := cmd.Flags().GetString("external_id")
		filePath, _ := cmd.Flags().GetString("file")

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

		var imageURL string
		var imageFilePath string
		var filename string

		if filePath != "" {
			// Upload local file
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s", filePath)
			}

			uploadResult, err := storage.UploadFile(cfg, accessToken, "product-images", filePath)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}

			imageURL = uploadResult.FileURL
			imageFilePath = uploadResult.FilePath
			filename = filepath.Base(filePath)
		} else if len(args) == 1 {
			imageURL = args[0]
			if _, err := url.Parse(imageURL); err != nil {
				return fmt.Errorf("invalid URL: %s", imageURL)
			}

			filename = imageURL
			if parsedURL, err := url.Parse(imageURL); err == nil {
				pathParts := strings.Split(parsedURL.Path, "/")
				if len(pathParts) > 0 {
					filename = pathParts[len(pathParts)-1]
				}
			}
			imageFilePath = imageURL
		} else {
			return fmt.Errorf("provide a URL as argument or use --file for local file upload")
		}

		mimeType := storage.DetectContentType(filename)

		objectData := map[string]interface{}{
			"project_id": cfg.ProjectID,
			"user_id":    userID,
			"filename":   filename,
			"file_path":  imageFilePath,
			"file_url":   imageURL,
			"file_size":  0,
			"mime_type":  mimeType,
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
			"select":     "id,filename,file_path,label,external_id,created_at",
		})
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		if jsonOutput {
			// Add selected flag to JSON output
			for i := range objects {
				if id, ok := objects[i]["id"].(string); ok && id == cfg.ObjectID {
					objects[i]["selected"] = true
				} else {
					objects[i]["selected"] = false
				}
			}
			return output.PrintJSON(objects)
		}

		if len(objects) == 0 {
			fmt.Println("No objects found.")
			return nil
		}

		table := output.NewTable("LABEL", "ID", "EXTERNAL_ID")
		for _, obj := range objects {
			label := ""
			if l, ok := obj["label"].(string); ok && l != "" {
				label = l
			} else if f, ok := obj["filename"].(string); ok {
				label = f
			}
			id := fmt.Sprintf("%v", obj["id"])
			externalID := "-"
			if eid, ok := obj["external_id"].(string); ok && eid != "" {
				externalID = eid
			}
			selected := id == cfg.ObjectID
			table.AddRowWithMarker(selected, label, id, externalID)
		}
		table.Print()

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
	objectsCreateCmd.Flags().String("file", "", "Local image file to upload")
	objectsDeleteCmd.Flags().String("object_id", "", "Object ID to delete")
}

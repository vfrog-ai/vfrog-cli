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

// datasetImagesCmd represents the dataset_images command
var datasetImagesCmd = &cobra.Command{
	Use:   "dataset_images",
	Short: "Manage dataset images",
	Long:  `Manage dataset images in your project.`,
}

// datasetImagesUploadCmd represents the dataset_images upload command
var datasetImagesUploadCmd = &cobra.Command{
	Use:   "upload [url1] [url2] ...",
	Short: "Upload dataset images from URLs",
	Long:  `Upload dataset images to your project from URLs. In v0.1, only URLs are supported.`,
	Args:  cobra.MinimumNArgs(1),
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

		accessToken, err := auth.GetValidToken(cfg)
		if err != nil {
			return fmt.Errorf("failed to get access token: %w", err)
		}
		userID, err := supabase.DecodeJWT(accessToken)
		if err != nil {
			return fmt.Errorf("failed to get user ID: %w", err)
		}

		var results []map[string]interface{}

		for _, imageURL := range args {
			if _, err := url.Parse(imageURL); err != nil {
				return fmt.Errorf("invalid URL: %s", imageURL)
			}

			filename := imageURL
			if parsedURL, err := url.Parse(imageURL); err == nil {
				pathParts := strings.Split(parsedURL.Path, "/")
				if len(pathParts) > 0 {
					filename = pathParts[len(pathParts)-1]
				}
			}

			imageData := map[string]interface{}{
				"project_id": cfg.ProjectID,
				"user_id":    userID,
				"filename":   filename,
				"file_path":  imageURL,
				"file_url":   imageURL,
			}

			result, err := client.Post("dataset_images", imageData)
			if err != nil {
				return fmt.Errorf("failed to upload %s: %w", imageURL, err)
			}

			results = append(results, result)
		}

		if jsonOutput {
			return output.PrintJSON(results)
		}

		output.PrintSuccess(fmt.Sprintf("Uploaded %d dataset image(s)", len(results)))
		for _, r := range results {
			fmt.Printf("  - %s (ID: %v)\n", r["filename"], r["id"])
		}

		return nil
	},
}

// datasetImagesListCmd represents the dataset_images list command
var datasetImagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List dataset images",
	Long:  `List all dataset images in the configured project.`,
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

		images, err := client.Get("dataset_images", map[string]string{
			"project_id": fmt.Sprintf("eq.%s", cfg.ProjectID),
			"select":     "id,filename,file_url,created_at",
		})
		if err != nil {
			return fmt.Errorf("failed to list dataset images: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(images)
		}

		if len(images) == 0 {
			fmt.Println("No dataset images found.")
			return nil
		}

		table := output.NewTable("FILENAME", "ID", "URL")
		for _, img := range images {
			filename := fmt.Sprintf("%v", img["filename"])
			id := fmt.Sprintf("%v", img["id"])
			fileURL := fmt.Sprintf("%v", img["file_url"])
			table.AddRow(filename, id, fileURL)
		}
		table.Print()

		return nil
	},
}

// datasetImagesDeleteCmd represents the dataset_images delete command
var datasetImagesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a dataset image",
	Long:  `Delete a dataset image by ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		datasetImageID, _ := cmd.Flags().GetString("dataset_image_id")
		if datasetImageID == "" {
			return fmt.Errorf("dataset_image_id is required")
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

		if err := client.Delete("dataset_images", datasetImageID); err != nil {
			return fmt.Errorf("failed to delete dataset image: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{"status": "success", "dataset_image_id": datasetImageID})
		}

		output.PrintSuccess(fmt.Sprintf("Deleted dataset image %s", datasetImageID))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(datasetImagesCmd)
	datasetImagesCmd.AddCommand(datasetImagesUploadCmd)
	datasetImagesCmd.AddCommand(datasetImagesListCmd)
	datasetImagesCmd.AddCommand(datasetImagesDeleteCmd)

	datasetImagesDeleteCmd.Flags().String("dataset_image_id", "", "Dataset image ID to delete")
}

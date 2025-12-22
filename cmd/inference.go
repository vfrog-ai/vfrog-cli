package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/api/vfrogapi"
	"github.com/vfrog/vfrog-cli/internal/config"
	"github.com/vfrog/vfrog-cli/internal/output"
)

var (
	inferenceAPIKey    string
	inferenceImageURL  string
	inferenceImagePath string
)

// inferenceCmd represents the inference command
var inferenceCmd = &cobra.Command{
	Use:   "inference",
	Short: "Run CV inference",
	Long:  `Run a computer vision inference request on an image. Supports both URLs and local files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := vfrogapi.NewClient(cfg, inferenceAPIKey)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		req := vfrogapi.InferenceRequest{}

		if inferenceImagePath != "" {
			imageBase64, err := vfrogapi.EncodeImageFile(inferenceImagePath)
			if err != nil {
				return fmt.Errorf("failed to read image file: %w", err)
			}
			req.ImageBase64 = imageBase64
		} else if inferenceImageURL != "" {
			req.ImageURL = inferenceImageURL
		} else {
			return fmt.Errorf("either --image_url or --image must be provided")
		}

		result, err := client.RunInference(req)
		if err != nil {
			return fmt.Errorf("inference failed: %w", err)
		}

		if !result.Success {
			return fmt.Errorf("inference error: %s", result.Error)
		}

		if jsonOutput {
			return output.PrintJSON(result)
		}

		fmt.Printf("Request ID: %s\n", result.RequestID)
		fmt.Printf("Status: %s\n", result.Status)
		if len(result.Results) > 0 {
			fmt.Printf("Results: %d detection(s)\n", len(result.Results))
			for i, r := range result.Results {
				fmt.Printf("  %d. %v\n", i+1, r)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(inferenceCmd)
	inferenceCmd.Flags().StringVar(&inferenceAPIKey, "api-key", "", "API key for inference (overrides env and config)")
	inferenceCmd.Flags().StringVar(&inferenceImageURL, "image_url", "", "Image URL to process")
	inferenceCmd.Flags().StringVar(&inferenceImagePath, "image", "", "Local image file path to process")
}

package cmd

import (
	"fmt"
	"strings"

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
	Long: `Run a computer vision inference request on an image. Supports both URLs and local files.

Use subcommands for batch inference, status checks, and feedback:
  vfrog inference batch    Run batch inference on multiple images
  vfrog inference status   Check the status of an inference request
  vfrog inference feedback Submit feedback for an inference request

Or run directly for single image inference:
  vfrog inference --image_url <url> --api-key <key>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no flags provided and subcommand not matched, show help
		if inferenceImageURL == "" && inferenceImagePath == "" {
			return cmd.Help()
		}

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

// inferenceBatchCmd represents the inference batch command
var inferenceBatchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Run batch inference on multiple images",
	Long: `Run batch CV inference on up to 10 image URLs.

Example:
  vfrog inference batch --api-key <key> --image_url url1,url2,url3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		imageURLs, _ := cmd.Flags().GetString("image_url")
		externalID, _ := cmd.Flags().GetString("external_id")
		apiKey, _ := cmd.Flags().GetString("api-key")

		if imageURLs == "" {
			return fmt.Errorf("--image_url is required (comma-separated list of URLs)")
		}

		urls := strings.Split(imageURLs, ",")
		for i := range urls {
			urls[i] = strings.TrimSpace(urls[i])
		}

		if len(urls) > 10 {
			return fmt.Errorf("maximum 10 URLs allowed, got %d", len(urls))
		}

		if len(urls) == 0 {
			return fmt.Errorf("at least one URL is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := vfrogapi.NewClient(cfg, apiKey)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		req := vfrogapi.BatchInferenceRequest{
			ImageURLs:  urls,
			ExternalID: externalID,
		}

		result, err := client.RunBatchInference(req)
		if err != nil {
			return fmt.Errorf("batch inference failed: %w", err)
		}

		if !result.Success {
			return fmt.Errorf("batch inference error: %s", result.Error)
		}

		if jsonOutput {
			return output.PrintJSON(result)
		}

		table := output.NewTable("IMAGE_URL", "REQUEST_ID", "STATUS", "DETECTIONS")
		for _, r := range result.Results {
			imgURL := r.ImageURL
			if len(imgURL) > 60 {
				imgURL = imgURL[:57] + "..."
			}
			detections := fmt.Sprintf("%d", len(r.Results))
			table.AddRow(imgURL, r.RequestID, r.Status, detections)
		}
		table.Print()

		return nil
	},
}

// inferenceStatusCmd represents the inference status command
var inferenceStatusCmd = &cobra.Command{
	Use:   "status <request_id>",
	Short: "Check inference request status",
	Long: `Check the status of a CV inference request by its request ID.

Example:
  vfrog inference status abc123 --api-key <key>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		requestID := args[0]
		apiKey, _ := cmd.Flags().GetString("api-key")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := vfrogapi.NewClient(cfg, apiKey)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		result, err := client.GetRequestStatus(requestID)
		if err != nil {
			return fmt.Errorf("failed to get request status: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(result)
		}

		fmt.Printf("Request ID: %s\n", result.RequestID)
		fmt.Printf("Status: %s\n", result.Status)
		fmt.Printf("Image URL: %s\n", result.ImageURL)
		if len(result.Results) > 0 {
			fmt.Printf("Results: %d detection(s)\n", len(result.Results))
			for i, r := range result.Results {
				fmt.Printf("  %d. %v\n", i+1, r)
			}
		}

		return nil
	},
}

// inferenceFeedbackCmd represents the inference feedback command
var inferenceFeedbackCmd = &cobra.Command{
	Use:   "feedback",
	Short: "Submit feedback for an inference request",
	Long: `Submit feedback for a CV inference request.

Rating values:
  -1  Negative (bad result)
   0  Neutral
   1  Positive (good result)

Example:
  vfrog inference feedback --request_id abc123 --rating 1 --api-key <key>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		requestID, _ := cmd.Flags().GetString("request_id")
		rating, _ := cmd.Flags().GetInt("rating")
		apiKey, _ := cmd.Flags().GetString("api-key")

		if requestID == "" {
			return fmt.Errorf("--request_id is required")
		}

		if rating != -1 && rating != 0 && rating != 1 {
			return fmt.Errorf("--rating must be -1, 0, or 1")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := vfrogapi.NewClient(cfg, apiKey)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		req := vfrogapi.FeedbackRequest{
			RequestID: requestID,
			Rating:    rating,
		}

		result, err := client.SubmitFeedback(req)
		if err != nil {
			return fmt.Errorf("failed to submit feedback: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(result)
		}

		output.PrintSuccess(fmt.Sprintf("Feedback submitted for request %s (rating: %d)", requestID, rating))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(inferenceCmd)

	// Root inference flags (backward compat)
	inferenceCmd.Flags().StringVar(&inferenceAPIKey, "api-key", "", "API key for inference (overrides env and config)")
	inferenceCmd.Flags().StringVar(&inferenceImageURL, "image_url", "", "Image URL to process")
	inferenceCmd.Flags().StringVar(&inferenceImagePath, "image", "", "Local image file path to process")

	// Subcommands
	inferenceCmd.AddCommand(inferenceBatchCmd)
	inferenceCmd.AddCommand(inferenceStatusCmd)
	inferenceCmd.AddCommand(inferenceFeedbackCmd)

	// Batch flags
	inferenceBatchCmd.Flags().String("api-key", "", "API key for inference")
	inferenceBatchCmd.Flags().String("image_url", "", "Comma-separated image URLs (max 10)")
	inferenceBatchCmd.Flags().String("external_id", "", "External ID for the batch request")

	// Status flags
	inferenceStatusCmd.Flags().String("api-key", "", "API key for inference")

	// Feedback flags
	inferenceFeedbackCmd.Flags().String("api-key", "", "API key for inference")
	inferenceFeedbackCmd.Flags().String("request_id", "", "Request ID to submit feedback for")
	inferenceFeedbackCmd.Flags().Int("rating", 0, "Rating (-1, 0, or 1)")
}

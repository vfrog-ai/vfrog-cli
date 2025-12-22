package cmd

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/api/inference"
	"github.com/vfrog/vfrog-cli/internal/api/supabase"
	"github.com/vfrog/vfrog-cli/internal/config"
	"github.com/vfrog/vfrog-cli/internal/output"
)

// iterationsCmd represents the iterations command
var iterationsCmd = &cobra.Command{
	Use:   "iterations",
	Short: "Manage iterations",
	Long:  `Manage project iterations.`,
}

// iterationsListCmd represents the iterations list command
var iterationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List iterations",
	Long:  `List all iterations for a specific object (product image).`,
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

		iterations, err := client.Get("project_iteration", map[string]string{
			"product_image_id": fmt.Sprintf("eq.%s", objectID),
			"select":           "id,iteration_number,status,trained_status,created_at",
			"order":            "iteration_number.desc",
		})
		if err != nil {
			return fmt.Errorf("failed to list iterations: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(iterations)
		}

		fmt.Println("Iterations:")
		for _, iter := range iterations {
			fmt.Printf("  - Iteration #%v (ID: %v, Status: %v)\n", iter["iteration_number"], iter["id"], iter["status"])
		}

		return nil
	},
}

// iterationsCreateCmd represents the iterations create command
var iterationsCreateCmd = &cobra.Command{
	Use:   "create [object_id]",
	Short: "Create a new iteration",
	Long:  `Create a new iteration for an object. Randomly selects N (default 20) dataset images from the project.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		objectID := args[0]
		randomN, _ := cmd.Flags().GetInt("random")
		if randomN <= 0 {
			randomN = 20
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

		existingIterations, err := client.Get("project_iteration", map[string]string{
			"product_image_id": fmt.Sprintf("eq.%s", objectID),
			"select":           "iteration_number",
		})
		if err != nil {
			return fmt.Errorf("failed to get existing iterations: %w", err)
		}

		nextIterationNumber := 1
		for _, iter := range existingIterations {
			if num, ok := iter["iteration_number"].(float64); ok {
				if int(num) >= nextIterationNumber {
					nextIterationNumber = int(num) + 1
				}
			}
		}

		datasetImages, err := client.Get("dataset_images", map[string]string{
			"project_id": fmt.Sprintf("eq.%s", cfg.ProjectID),
			"select":     "id",
		})
		if err != nil {
			return fmt.Errorf("failed to get dataset images: %w", err)
		}

		if len(datasetImages) == 0 {
			return fmt.Errorf("no dataset images found in project")
		}

		rand.Seed(time.Now().UnixNano())
		selectedCount := randomN
		if selectedCount > len(datasetImages) {
			selectedCount = len(datasetImages)
		}

		selectedIndices := rand.Perm(len(datasetImages))[:selectedCount]
		selectedImages := make([]map[string]interface{}, selectedCount)
		for i, idx := range selectedIndices {
			selectedImages[i] = datasetImages[idx]
		}

		iterationData := map[string]interface{}{
			"project_id":       cfg.ProjectID,
			"product_image_id": objectID,
			"iteration_number": nextIterationNumber,
			"status":           "created",
		}

		iteration, err := client.Post("project_iteration", iterationData)
		if err != nil {
			return fmt.Errorf("failed to create iteration: %w", err)
		}

		iterationID := iteration["id"].(string)

		for _, img := range selectedImages {
			linkData := map[string]interface{}{
				"project_iteration_id": iterationID,
				"dataset_image_id":     img["id"],
			}
			_, err := client.Post("project_iteration_dataset_images", linkData)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to link dataset image %v: %v\n", img["id"], err)
			}
		}

		if jsonOutput {
			return output.PrintJSON(iteration)
		}

		output.PrintSuccess(fmt.Sprintf("Created iteration #%d (ID: %s) with %d dataset images", nextIterationNumber, iterationID, selectedCount))
		return nil
	},
}

// iterationsDeleteCmd represents the iterations delete command
var iterationsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an iteration",
	Long:  `Delete an iteration by ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		iterationID, _ := cmd.Flags().GetString("iteration_id")
		if iterationID == "" {
			return fmt.Errorf("iteration_id is required")
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

		if err := client.Delete("project_iteration", iterationID); err != nil {
			return fmt.Errorf("failed to delete iteration: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]string{"status": "success", "iteration_id": iterationID})
		}

		output.PrintSuccess(fmt.Sprintf("Deleted iteration %s", iterationID))
		return nil
	},
}

// iterationsSSATCmd represents the iterations ssat command
var iterationsSSATCmd = &cobra.Command{
	Use:   "ssat",
	Short: "Get SSAT URL for an iteration",
	Long:  `Print the Platform URL for SSAT (Semi-Supervised Active Training) workflow for an iteration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		iterationID, _ := cmd.Flags().GetString("iteration_id")
		if iterationID == "" {
			return fmt.Errorf("iteration_id is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterations, err := client.Get("project_iteration", map[string]string{
			"id":     fmt.Sprintf("eq.%s", iterationID),
			"select": "project_id,product_image_id",
		})
		if err != nil {
			return fmt.Errorf("failed to get iteration: %w", err)
		}

		if len(iterations) == 0 {
			return fmt.Errorf("iteration not found")
		}

		iter := iterations[0]
		projectID := iter["project_id"].(string)
		productID := iter["product_image_id"].(string)

		platformHost := cfg.PlatformHost
		if platformHost == "" {
			platformHost = "https://platform.vfrog.ai"
		}

		url := fmt.Sprintf("%s/org/%s/proj/%s/prod/%s/iter/%s", platformHost, cfg.OrganisationID, projectID, productID, iterationID)

		if jsonOutput {
			return output.PrintJSON(map[string]string{"url": url, "iteration_id": iterationID})
		}

		fmt.Println(url)
		return nil
	},
}

// iterationsHaloCmd represents the iterations halo command
var iterationsHaloCmd = &cobra.Command{
	Use:   "halo",
	Short: "Get HALO URL for an iteration",
	Long:  `Print the Platform URL for HALO (Human Assisted Labelling of Objects) workflow for an iteration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		iterationID, _ := cmd.Flags().GetString("iteration_id")
		if iterationID == "" {
			return fmt.Errorf("iteration_id is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterations, err := client.Get("project_iteration", map[string]string{
			"id":     fmt.Sprintf("eq.%s", iterationID),
			"select": "project_id",
		})
		if err != nil {
			return fmt.Errorf("failed to get iteration: %w", err)
		}

		if len(iterations) == 0 {
			return fmt.Errorf("iteration not found")
		}

		iter := iterations[0]
		projectID := iter["project_id"].(string)

		platformHost := cfg.PlatformHost
		if platformHost == "" {
			platformHost = "https://platform.vfrog.ai"
		}

		url := fmt.Sprintf("%s/org/%s/proj/%s/halo?iteration=%s", platformHost, cfg.OrganisationID, projectID, iterationID)

		if jsonOutput {
			return output.PrintJSON(map[string]string{"url": url, "iteration_id": iterationID})
		}

		fmt.Println(url)
		return nil
	},
}

// iterationTrainCmd represents the iteration train command
var iterationTrainCmd = &cobra.Command{
	Use:   "train",
	Short: "Train a model for an iteration",
	Long:  `Train a model for an iteration using the inference server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		iterationID, _ := cmd.Flags().GetString("iteration_id")
		if iterationID == "" {
			return fmt.Errorf("iteration_id is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireProjectID(); err != nil {
			return err
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterations, err := client.Get("project_iteration", map[string]string{
			"id":     fmt.Sprintf("eq.%s", iterationID),
			"select": "project_id,product_image_id,iteration_number",
		})
		if err != nil {
			return fmt.Errorf("failed to get iteration: %w", err)
		}

		if len(iterations) == 0 {
			return fmt.Errorf("iteration not found")
		}

		iter := iterations[0]
		projectID := iter["project_id"].(string)
		productID := iter["product_image_id"].(string)

		projects, err := client.Get("projects", map[string]string{
			"id":     fmt.Sprintf("eq.%s", projectID),
			"select": "title",
		})
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}

		if len(projects) == 0 {
			return fmt.Errorf("project not found")
		}

		projectName := projects[0]["title"].(string)

		datasetImageLinks, err := client.Get("project_iteration_dataset_images", map[string]string{
			"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
			"select":               "dataset_image_id,dataset_images(file_url)",
		})
		if err != nil {
			return fmt.Errorf("failed to get dataset images: %w", err)
		}

		datasetImages := make([]map[string]interface{}, 0, len(datasetImageLinks))
		for _, link := range datasetImageLinks {
			if datasetImg, ok := link["dataset_images"].(map[string]interface{}); ok {
				datasetImages = append(datasetImages, map[string]interface{}{
					"id":       link["dataset_image_id"],
					"file_url": datasetImg["file_url"],
				})
			}
		}

		annotatedImages, err := client.Get("project_iteration_annotated_images", map[string]string{
			"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
			"select":               "dataset_images_id,annotation",
		})
		if err != nil {
			return fmt.Errorf("failed to get annotated images: %w", err)
		}

		annotatedImagesList := make([]map[string]interface{}, 0, len(annotatedImages))
		for _, annImg := range annotatedImages {
			annotatedImagesList = append(annotatedImagesList, map[string]interface{}{
				"dataset_images_id": annImg["dataset_images_id"],
				"annotation":        annImg["annotation"],
			})
		}

		callbackURL := ""
		if cfg.APIProjectBaseURL != "" {
			callbackURL = fmt.Sprintf("%s/api/v1/callback/project-status-update", cfg.APIProjectBaseURL)
		}

		inferenceClient, err := inference.NewClient(cfg, "")
		if err != nil {
			return fmt.Errorf("failed to create inference client: %w", err)
		}

		taskResponse, err := inferenceClient.SubmitTrainingTask(map[string]interface{}{
			"project_iteration_id": iterationID,
			"organisation_id":      cfg.OrganisationID,
			"project_name":         projectName,
			"product_id":           productID,
			"dataset_images":       datasetImages,
			"annotated_images":     annotatedImagesList,
			"callback_url":         callbackURL,
		})
		if err != nil {
			return fmt.Errorf("failed to submit training task: %w", err)
		}

		updateData := map[string]interface{}{
			"task_id":        taskResponse["task_id"],
			"trained_status": "training",
		}
		if err := client.Patch("project_iteration", iterationID, updateData); err != nil {
			return fmt.Errorf("failed to update iteration: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(taskResponse)
		}

		output.PrintSuccess(fmt.Sprintf("Training started for iteration %s (task_id: %v)", iterationID, taskResponse["task_id"]))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(iterationsCmd)
	iterationsCmd.AddCommand(iterationsListCmd)
	iterationsCmd.AddCommand(iterationsCreateCmd)
	iterationsCmd.AddCommand(iterationsDeleteCmd)
	iterationsCmd.AddCommand(iterationsSSATCmd)
	iterationsCmd.AddCommand(iterationsHaloCmd)

	rootCmd.AddCommand(iterationTrainCmd)

	iterationsListCmd.Flags().String("object_id", "", "Object (product image) ID")
	iterationsCreateCmd.Flags().Int("random", 20, "Number of random dataset images to select")
	iterationsDeleteCmd.Flags().String("iteration_id", "", "Iteration ID to delete")
	iterationsSSATCmd.Flags().String("iteration_id", "", "Iteration ID")
	iterationsHaloCmd.Flags().String("iteration_id", "", "Iteration ID")
	iterationTrainCmd.Flags().String("iteration_id", "", "Iteration ID to train")
}

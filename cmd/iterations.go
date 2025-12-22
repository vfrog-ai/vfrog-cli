package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/api/supabase"
	"github.com/vfrog/vfrog-cli/internal/api/vfrogapi"
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
	Short: "Start SSAT auto-annotation for an iteration",
	Long: `Start the SSAT (Semi-Supervised Active Training) annotation workflow for an iteration.
	
For iteration 1: Uses the annotator service with cutout extraction and matching.
For iteration 2+: Uses inference with a trained model from the previous iteration.

The number of dataset images processed depends on the iteration number:
- Iteration 1: 20 images
- Iteration 2: 40 images
- Iteration 3+: 80 images

The iteration must be in 'created' status to start SSAT.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		iterationID, _ := cmd.Flags().GetString("iteration_id")
		if iterationID == "" {
			return fmt.Errorf("iteration_id is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		// Get iteration details
		iterations, err := client.Get("project_iteration", map[string]string{
			"id":     fmt.Sprintf("eq.%s", iterationID),
			"select": "id,project_id,product_image_id,iteration_number,status,model_id,ssat_model_id",
		})
		if err != nil {
			return fmt.Errorf("failed to get iteration: %w", err)
		}

		if len(iterations) == 0 {
			return fmt.Errorf("iteration not found: %s", iterationID)
		}

		iter := iterations[0]
		status := iter["status"].(string)
		
		// Check if iteration is in 'created' status
		if status != "created" {
			return fmt.Errorf("iteration status must be 'created' to start SSAT (current: %s)", status)
		}

		productImageID := iter["product_image_id"].(string)
		iterationNumber := int(iter["iteration_number"].(float64))

		// Determine number of images based on iteration number
		var imageCount int
		switch iterationNumber {
		case 1:
			imageCount = 20
		case 2:
			imageCount = 40
		default:
			imageCount = 80
		}

		// Get linked dataset images for this iteration
		datasetImageLinks, err := client.Get("project_iteration_dataset_images", map[string]string{
			"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
			"select":               "id,dataset_image_id,dataset_images(id,file_url)",
		})
		if err != nil {
			return fmt.Errorf("failed to get dataset images: %w", err)
		}

		if len(datasetImageLinks) == 0 {
			return fmt.Errorf("no dataset images linked to this iteration. Create the iteration with dataset images first")
		}

		// Limit to imageCount
		if len(datasetImageLinks) > imageCount {
			// Randomly select imageCount images
			rand.Seed(time.Now().UnixNano())
			selectedIndices := rand.Perm(len(datasetImageLinks))[:imageCount]
			selected := make([]map[string]interface{}, imageCount)
			for i, idx := range selectedIndices {
				selected[i] = datasetImageLinks[idx]
			}
			datasetImageLinks = selected
		}

		// Check if we have a model (iteration 2+)
		var modelID string
		if iter["model_id"] != nil {
			modelID = iter["model_id"].(string)
		} else if iter["ssat_model_id"] != nil {
			modelID = iter["ssat_model_id"].(string)
		}

		// For iteration 2+, require a model
		if iterationNumber >= 2 && modelID == "" {
			return fmt.Errorf("no model available. Train a model on iteration %d before starting iteration %d", iterationNumber-1, iterationNumber)
		}

		// Build callback URL
		callbackURL := ""
		if cfg.APIProjectBaseURL != "" {
			callbackURL = fmt.Sprintf("%s/api/v1/callback/project-status-update?include_annotations=true", cfg.APIProjectBaseURL)
		}

		if iterationNumber >= 2 && modelID != "" {
			// Use inference for iteration 2+
			return runInferenceSSAT(cmd, cfg, client, iterationID, modelID, datasetImageLinks, callbackURL)
		}

		// Use annotator for iteration 1
		return runAnnotatorSSAT(cmd, cfg, client, iterationID, productImageID, datasetImageLinks, callbackURL)
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
	Long:  `Train a model for an iteration using the API project.`,
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

		// Get API key
		apiKey := getAPIKey(cfg)
		if apiKey == "" {
			return fmt.Errorf("API key is required. Set VFROG_API_KEY env var or use 'vfrog config set api_key'")
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

		datasetImages := make([]vfrogapi.InferenceImageRef, 0, len(datasetImageLinks))
		for _, link := range datasetImageLinks {
			if datasetImg, ok := link["dataset_images"].(map[string]interface{}); ok {
				imgID := ""
				if id, ok := link["dataset_image_id"].(string); ok {
					imgID = id
				}
				imgURL := ""
				if url, ok := datasetImg["file_url"].(string); ok {
					imgURL = url
				}
				if imgID != "" && imgURL != "" {
					datasetImages = append(datasetImages, vfrogapi.InferenceImageRef{
						ID:      imgID,
						FileURL: imgURL,
					})
				}
			}
		}

		annotatedImagesData, err := client.Get("project_iteration_annotated_images", map[string]string{
			"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
			"select":               "dataset_images_id,annotation",
		})
		if err != nil {
			return fmt.Errorf("failed to get annotated images: %w", err)
		}

		annotatedImages := make([]vfrogapi.AnnotatedImageRef, 0, len(annotatedImagesData))
		for _, annImg := range annotatedImagesData {
			dsID := ""
			if id, ok := annImg["dataset_images_id"].(string); ok {
				dsID = id
			}
			var annotations []interface{}
			if ann, ok := annImg["annotation"].([]interface{}); ok {
				annotations = ann
			}
			if dsID != "" {
				annotatedImages = append(annotatedImages, vfrogapi.AnnotatedImageRef{
					DatasetImagesID: dsID,
					Annotation:      annotations,
				})
			}
		}

		callbackURL := ""
		if cfg.APIProjectBaseURL != "" {
			callbackURL = fmt.Sprintf("%s/api/v1/callback/project-status-update", cfg.APIProjectBaseURL)
		}

		// Create API client
		ssatClient, err := vfrogapi.NewSSATClient(cfg, apiKey)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Submit training task via API project
		params := vfrogapi.TrainParams{
			ProjectIterationID: iterationID,
			OrganisationID:     cfg.OrganisationID,
			ProjectName:        projectName,
			ProductID:          productID,
			DatasetImages:      datasetImages,
			AnnotatedImages:    annotatedImages,
			CallbackURL:        callbackURL,
		}

		taskResponse, err := ssatClient.Train(params)
		if err != nil {
			return fmt.Errorf("failed to submit training task: %w", err)
		}

		updateData := map[string]interface{}{
			"trained_status": "training",
		}
		if taskID, ok := taskResponse["task_id"]; ok {
			updateData["task_id"] = taskID
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

// getAPIKey gets the API key from flag, env var, or config
func getAPIKey(cfg *config.Config) string {
	// Check flag (set by parent command)
	apiKey := os.Getenv("VFROG_API_KEY")
	if apiKey != "" {
		return apiKey
	}
	// Check config
	if cfg.APIKey != "" {
		return cfg.APIKey
	}
	return ""
}

// runAnnotatorSSAT runs SSAT using the API project (for iteration 1)
func runAnnotatorSSAT(cmd *cobra.Command, cfg *config.Config, client *supabase.Client, iterationID, productImageID string, datasetImageLinks []map[string]interface{}, callbackURL string) error {
	// Get API key
	apiKey := getAPIKey(cfg)
	if apiKey == "" {
		return fmt.Errorf("API key is required. Set VFROG_API_KEY env var or use 'vfrog config set api_key'")
	}

	// Get product image details
	productImages, err := client.Get("product_images", map[string]string{
		"id":     fmt.Sprintf("eq.%s", productImageID),
		"select": "id,label,file_url,file_path",
	})
	if err != nil {
		return fmt.Errorf("failed to get product image: %w", err)
	}

	if len(productImages) == 0 {
		return fmt.Errorf("product image not found: %s", productImageID)
	}

	productImage := productImages[0]
	productImageURL := ""
	if url, ok := productImage["file_url"].(string); ok && url != "" {
		productImageURL = url
	} else if path, ok := productImage["file_path"].(string); ok {
		productImageURL = path
	}

	if productImageURL == "" {
		return fmt.Errorf("product image has no URL")
	}

	label := "product"
	if l, ok := productImage["label"].(string); ok && l != "" {
		label = l
	}

	// Build dataset images list
	datasetImages := make([]vfrogapi.DatasetImageRef, 0, len(datasetImageLinks))
	for _, link := range datasetImageLinks {
		if dsImg, ok := link["dataset_images"].(map[string]interface{}); ok {
			imgID := link["dataset_image_id"].(string)
			imgURL := ""
			if url, ok := dsImg["file_url"].(string); ok && url != "" {
				imgURL = url
			}
			if imgURL != "" {
				datasetImages = append(datasetImages, vfrogapi.DatasetImageRef{
					ID:       imgID,
					ImageURL: imgURL,
				})
			}
		}
	}

	if len(datasetImages) == 0 {
		return fmt.Errorf("no valid dataset images found")
	}

	// Create API client
	ssatClient, err := vfrogapi.NewSSATClient(cfg, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Submit batch via API project
	params := vfrogapi.BatchSubmitParams{
		ProjectIterationID: iterationID,
		ProductImage: vfrogapi.ProductImageRef{
			ID:       productImageID,
			ImageURL: productImageURL,
			Label:    label,
		},
		DatasetImages: datasetImages,
		CallbackURL:   callbackURL,
	}

	_, err = ssatClient.BatchSubmit(params)
	if err != nil {
		return fmt.Errorf("failed to submit SSAT batch: %w", err)
	}

	// Update iteration status to annotating
	if err := client.Patch("project_iteration", iterationID, map[string]interface{}{
		"status": "annotating",
	}); err != nil {
		return fmt.Errorf("failed to update iteration status: %w", err)
	}

	if jsonOutput {
		return output.PrintJSON(map[string]interface{}{
			"iteration_id":   iterationID,
			"status":         "annotating",
			"dataset_images": len(datasetImages),
			"method":         "ssat-batch",
		})
	}

	output.PrintSuccess(fmt.Sprintf("SSAT started for iteration %s with %d dataset images", iterationID, len(datasetImages)))
	return nil
}

// runInferenceSSAT runs SSAT using the API project (for iteration 2+)
func runInferenceSSAT(cmd *cobra.Command, cfg *config.Config, client *supabase.Client, iterationID, modelID string, datasetImageLinks []map[string]interface{}, callbackURL string) error {
	// Get API key
	apiKey := getAPIKey(cfg)
	if apiKey == "" {
		return fmt.Errorf("API key is required. Set VFROG_API_KEY env var or use 'vfrog config set api_key'")
	}

	// Get model details
	models, err := client.Get("models", map[string]string{
		"id":     fmt.Sprintf("eq.%s", modelID),
		"select": "id,model_path",
	})
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	if len(models) == 0 {
		return fmt.Errorf("model not found: %s", modelID)
	}

	modelPath := ""
	if path, ok := models[0]["model_path"].(string); ok {
		modelPath = path
	}

	if modelPath == "" {
		return fmt.Errorf("model has no model_path")
	}

	// Build dataset images list (use project_iteration_dataset_image.id, not dataset_image_id)
	datasetImages := make([]vfrogapi.InferenceImageRef, 0, len(datasetImageLinks))
	for _, link := range datasetImageLinks {
		if dsImg, ok := link["dataset_images"].(map[string]interface{}); ok {
			// Use the project_iteration_dataset_images.id as the id (not dataset_image_id)
			imgID := link["id"].(string)
			imgURL := ""
			if url, ok := dsImg["file_url"].(string); ok && url != "" {
				imgURL = url
			}
			if imgURL != "" {
				datasetImages = append(datasetImages, vfrogapi.InferenceImageRef{
					ID:      imgID,
					FileURL: imgURL,
				})
			}
		}
	}

	if len(datasetImages) == 0 {
		return fmt.Errorf("no valid dataset images found")
	}

	// Get existing annotated images for this iteration
	annotatedImagesData, err := client.Get("project_iteration_annotated_images", map[string]string{
		"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
		"select":               "project_iteration_dataset_image_id,annotation,project_iteration_dataset_images!inner(dataset_image_id)",
	})
	if err != nil {
		return fmt.Errorf("failed to get annotated images: %w", err)
	}

	// Build annotated images list
	annotatedImages := make([]vfrogapi.AnnotatedImageRef, 0, len(annotatedImagesData))
	for _, annImg := range annotatedImagesData {
		var datasetImageID string
		if pidsiData, ok := annImg["project_iteration_dataset_images"].(map[string]interface{}); ok {
			if dsID, ok := pidsiData["dataset_image_id"].(string); ok {
				datasetImageID = dsID
			}
		}

		if datasetImageID == "" {
			continue
		}

		// Parse annotation as []interface{} for the API
		var annotations []interface{}
		if ann, ok := annImg["annotation"].([]interface{}); ok {
			annotations = ann
		}

		annotatedImages = append(annotatedImages, vfrogapi.AnnotatedImageRef{
			DatasetImagesID: datasetImageID,
			Annotation:      annotations,
		})
	}

	// Create API client
	ssatClient, err := vfrogapi.NewSSATClient(cfg, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Submit inference task via API project
	params := vfrogapi.RunInferenceParams{
		ProjectIterationID: iterationID,
		ModelPath:          modelPath,
		DatasetImages:      datasetImages,
		AnnotatedImages:    annotatedImages,
		CallbackURL:        callbackURL,
	}

	result, err := ssatClient.RunInference(params)
	if err != nil {
		return fmt.Errorf("failed to submit inference task: %w", err)
	}

	// Update iteration status to annotating and store task_id
	updateData := map[string]interface{}{
		"status": "annotating",
	}
	if taskID, ok := result["task_id"]; ok {
		updateData["task_id"] = taskID
	}
	if err := client.Patch("project_iteration", iterationID, updateData); err != nil {
		return fmt.Errorf("failed to update iteration status: %w", err)
	}

	if jsonOutput {
		return output.PrintJSON(map[string]interface{}{
			"iteration_id":     iterationID,
			"task_id":          result["task_id"],
			"status":           "annotating",
			"dataset_images":   len(datasetImages),
			"annotated_images": len(annotatedImages),
			"method":           "inference",
			"model_id":         modelID,
		})
	}

	output.PrintSuccess(fmt.Sprintf("SSAT started for iteration %s with %d dataset images (model: %s)", iterationID, len(datasetImages), modelID))
	return nil
}

// Helper functions for parsing map values
func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
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

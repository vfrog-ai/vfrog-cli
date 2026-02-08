package cmd

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"

	"github.com/vfrog/vfrog-cli/internal/api/supabase"
	"github.com/vfrog/vfrog-cli/internal/api/vfrogapi"
	"github.com/vfrog/vfrog-cli/internal/auth"
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
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List iterations",
	Long:    `List all iterations for the configured object (product image). Use --object_id to override.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireProjectID(); err != nil {
			return err
		}

		// Use flag or config
		objectID, _ := cmd.Flags().GetString("object_id")
		if objectID == "" {
			objectID = cfg.ObjectID
		}
		if objectID == "" {
			return fmt.Errorf("object_id is required. Set it with: vfrog config set object --object_id <id>")
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterations, err := client.Get("project_iteration", map[string]string{
			"product_image_id": fmt.Sprintf("eq.%s", objectID),
			"select":           "id,iteration_number,status,trained_status,created_at",
			"order":            "iteration_number.asc",
		})
		if err != nil {
			return fmt.Errorf("failed to list iterations: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(iterations)
		}

		if len(iterations) == 0 {
			fmt.Println("No iterations found.")
			return nil
		}

		table := output.NewTable("ITERATION", "ID", "STATUS", "TRAINED")
		for _, iter := range iterations {
			iterNum := fmt.Sprintf("#%v", iter["iteration_number"])
			id := fmt.Sprintf("%v", iter["id"])
			status := fmt.Sprintf("%v", iter["status"])
			trainedStatus := "-"
			if ts, ok := iter["trained_status"].(string); ok && ts != "" {
				trainedStatus = ts
			}
			table.AddRow(iterNum, id, status, trainedStatus)
		}
		table.Print()

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

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
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

By default, ALL dataset images linked to the iteration are used.

Use --random X to randomly select X dataset images from the project's dataset_images instead
of using the images linked to the iteration.

The iteration must be in 'created' status to start SSAT.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		randomCount, _ := cmd.Flags().GetInt("random")
		restartFlag, _ := cmd.Flags().GetBool("restart")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		if err := cfg.RequireProjectID(); err != nil {
			return err
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		// Resolve iteration ID
		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
		}

		// If --restart flag is set, restart the iteration first
		if restartFlag {
			// Get access token for restart
			accessToken, err := getAccessToken(cfg)
			if err != nil {
				return fmt.Errorf("authentication required: %w. Run 'vfrog login' first", err)
			}

			// Create API client
			ssatClient, err := vfrogapi.NewSSATClient(cfg, accessToken)
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Restart the iteration
			params := vfrogapi.RestartIterationParams{
				IterationID: iterationID,
			}

			result, err := ssatClient.RestartIteration(params)
			if err != nil {
				return fmt.Errorf("failed to restart iteration: %w", err)
			}

			// Use the new iteration ID
			newIterationID, ok := result["iteration_id"].(string)
			if !ok {
				return fmt.Errorf("invalid response from restart: missing iteration_id")
			}
			iterationID = newIterationID

			if !jsonOutput {
				fmt.Printf("Restarted iteration (new ID: %s)\n", iterationID)
			}
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
		projectID := iter["project_id"].(string)
		iterationNumber := int(iter["iteration_number"].(float64))

		var datasetImageLinks []map[string]interface{}

		if randomCount > 0 {
			// Randomly select from all project dataset images
			allDatasetImages, err := client.Get("dataset_images", map[string]string{
				"project_id": fmt.Sprintf("eq.%s", projectID),
				"select":     "id,file_url",
			})
			if err != nil {
				return fmt.Errorf("failed to get dataset images: %w", err)
			}

			if len(allDatasetImages) == 0 {
				return fmt.Errorf("no dataset images found in project")
			}

			// Randomly select randomCount images
			rand.Seed(time.Now().UnixNano())
			selectedCount := randomCount
			if selectedCount > len(allDatasetImages) {
				selectedCount = len(allDatasetImages)
			}

			selectedIndices := rand.Perm(len(allDatasetImages))[:selectedCount]
			datasetImageLinks = make([]map[string]interface{}, selectedCount)
			for i, idx := range selectedIndices {
				img := allDatasetImages[idx]
				// Format to match the structure expected by runAnnotatorSSAT/runInferenceSSAT
				datasetImageLinks[i] = map[string]interface{}{
					"dataset_image_id": img["id"],
					"dataset_images": map[string]interface{}{
						"id":       img["id"],
						"file_url": img["file_url"],
					},
				}
			}
		} else {
			// Use all linked dataset images for this iteration
			linkedImages, err := client.Get("project_iteration_dataset_images", map[string]string{
				"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
				"select":               "id,dataset_image_id,dataset_images(id,file_url)",
			})
			if err != nil {
				return fmt.Errorf("failed to get dataset images: %w", err)
			}

			if len(linkedImages) == 0 {
				return fmt.Errorf("no dataset images linked to this iteration. Create the iteration with dataset images first, or use --random X to select from project dataset images")
			}

			datasetImageLinks = linkedImages
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
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
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

		// Get access token (requires login)
		accessToken, err := getAccessToken(cfg)
		if err != nil {
			return fmt.Errorf("authentication required: %w. Run 'vfrog login' first", err)
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
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

		// Get access token (requires login)
		accessToken, authErr := getAccessToken(cfg)
		if authErr != nil {
			return fmt.Errorf("authentication required: %w. Run 'vfrog login' first", authErr)
		}

		// Create API client
		ssatClient, err := vfrogapi.NewSSATClient(cfg, accessToken)
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

// getAccessToken gets the access token from auth (requires login)
func getAccessToken(cfg *config.Config) (string, error) {
	return auth.GetValidToken(cfg)
}

// getIterationID resolves iteration ID from either --iteration_id or --iteration_number flag
// If iteration_number is provided, it uses object_id from flag or config
func getIterationID(cmd *cobra.Command, cfg *config.Config, client *supabase.Client) (string, error) {
	iterationID, _ := cmd.Flags().GetString("iteration_id")
	iterationNumber, _ := cmd.Flags().GetInt("iteration_number")

	if iterationID != "" && iterationNumber > 0 {
		return "", fmt.Errorf("cannot specify both --iteration_id and --iteration_number")
	}

	if iterationID != "" {
		return iterationID, nil
	}

	if iterationNumber > 0 {
		// Need object_id to find iteration by number
		objectID, _ := cmd.Flags().GetString("object_id")
		if objectID == "" {
			objectID = cfg.ObjectID
		}
		if objectID == "" {
			return "", fmt.Errorf("object_id is required when using --iteration_number. Set it with: vfrog config set object --object_id <id>")
		}

		// Find iteration by number
		iterations, err := client.Get("project_iteration", map[string]string{
			"product_image_id":  fmt.Sprintf("eq.%s", objectID),
			"iteration_number":   fmt.Sprintf("eq.%d", iterationNumber),
			"select":            "id",
		})
		if err != nil {
			return "", fmt.Errorf("failed to find iteration #%d: %w", iterationNumber, err)
		}

		if len(iterations) == 0 {
			return "", fmt.Errorf("iteration #%d not found for object %s", iterationNumber, objectID)
		}

		iterID, ok := iterations[0]["id"].(string)
		if !ok {
			return "", fmt.Errorf("invalid iteration ID format")
		}

		return iterID, nil
	}

	return "", fmt.Errorf("either --iteration_id or --iteration_number is required")
}

// runAnnotatorSSAT runs SSAT using the API project (for iteration 1)
func runAnnotatorSSAT(cmd *cobra.Command, cfg *config.Config, client *supabase.Client, iterationID, productImageID string, datasetImageLinks []map[string]interface{}, callbackURL string) error {
	// Get access token (requires login)
	accessToken, err := getAccessToken(cfg)
	if err != nil {
		return fmt.Errorf("authentication required: %w. Run 'vfrog login' first", err)
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
	ssatClient, err := vfrogapi.NewSSATClient(cfg, accessToken)
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
	// Get access token (requires login)
	accessToken, err := getAccessToken(cfg)
	if err != nil {
		return fmt.Errorf("authentication required: %w. Run 'vfrog login' first", err)
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
	ssatClient, err := vfrogapi.NewSSATClient(cfg, accessToken)
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

// iterationsAnnotationsCmd represents the iterations annotations command
var iterationsAnnotationsCmd = &cobra.Command{
	Use:   "annotations",
	Short: "List annotations for an iteration",
	Long: `List annotated images and their annotations for an iteration.

Example:
  vfrog iterations annotations --iteration_id <id>
  vfrog iterations annotations --iteration_number 1 --object_id <id>`,
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

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
		}

		annotatedImages, err := client.Get("project_iteration_annotated_images", map[string]string{
			"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
			"select":               "id,dataset_images_id,annotation,created_at",
		})
		if err != nil {
			return fmt.Errorf("failed to get annotations: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(annotatedImages)
		}

		if len(annotatedImages) == 0 {
			fmt.Println("No annotations found.")
			return nil
		}

		table := output.NewTable("DATASET_IMAGE_ID", "ANNOTATIONS", "CREATED_AT")
		for _, img := range annotatedImages {
			dsID := fmt.Sprintf("%v", img["dataset_images_id"])
			annotationCount := 0
			if ann, ok := img["annotation"].([]interface{}); ok {
				annotationCount = len(ann)
			}
			createdAt := "-"
			if ca, ok := img["created_at"].(string); ok {
				createdAt = ca
			}
			table.AddRow(dsID, fmt.Sprintf("%d", annotationCount), createdAt)
		}
		table.Print()

		return nil
	},
}

// iterationsStatusCmd represents the iterations status command
var iterationsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check iteration status",
	Long: `Check the current status of an iteration. Use --watch to poll until completion.

Example:
  vfrog iterations status --iteration_id <id>
  vfrog iterations status --iteration_id <id> --watch --interval 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		watch, _ := cmd.Flags().GetBool("watch")
		interval, _ := cmd.Flags().GetInt("interval")
		if interval <= 0 {
			interval = 5
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

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
		}

		for {
			iterations, err := client.Get("project_iteration", map[string]string{
				"id":     fmt.Sprintf("eq.%s", iterationID),
				"select": "id,status,trained_status,task_id,created_at,updated_at",
			})
			if err != nil {
				return fmt.Errorf("failed to get iteration status: %w", err)
			}

			if len(iterations) == 0 {
				return fmt.Errorf("iteration not found: %s", iterationID)
			}

			iter := iterations[0]

			if jsonOutput {
				return output.PrintJSON(iter)
			}

			status := fmt.Sprintf("%v", iter["status"])
			trainedStatus := "-"
			if ts, ok := iter["trained_status"].(string); ok && ts != "" {
				trainedStatus = ts
			}
			taskID := "-"
			if tid, ok := iter["task_id"].(string); ok && tid != "" {
				taskID = tid
			}

			if watch {
				fmt.Printf("\rStatus: %-15s Trained: %-15s Task: %s", status, trainedStatus, taskID)
			} else {
				fmt.Printf("Iteration ID: %s\n", iterationID)
				fmt.Printf("Status: %s\n", status)
				fmt.Printf("Trained Status: %s\n", trainedStatus)
				fmt.Printf("Task ID: %s\n", taskID)
				if ca, ok := iter["created_at"].(string); ok {
					fmt.Printf("Created: %s\n", ca)
				}
				if ua, ok := iter["updated_at"].(string); ok {
					fmt.Printf("Updated: %s\n", ua)
				}
			}

			if !watch {
				return nil
			}

			// Check for terminal status
			if status == "completed" || status == "failed" {
				fmt.Println() // newline after watch mode
				return nil
			}

			time.Sleep(time.Duration(interval) * time.Second)
		}
	},
}

// iterationsControlCmd represents the iterations control command
var iterationsControlCmd = &cobra.Command{
	Use:   "control",
	Short: "Run SSAT control for an iteration",
	Long: `Submit SSAT control for an iteration. Fetches linked dataset images and submits
them for control quality assessment.

Example:
  vfrog iterations control --iteration_id <id>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.RequireOrganisationID(); err != nil {
			return err
		}

		if err := cfg.RequireProjectID(); err != nil {
			return err
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
		}

		// Get linked dataset images
		linkedImages, err := client.Get("project_iteration_dataset_images", map[string]string{
			"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
			"select":               "id,dataset_image_id,dataset_images(id,file_url)",
		})
		if err != nil {
			return fmt.Errorf("failed to get dataset images: %w", err)
		}

		if len(linkedImages) == 0 {
			return fmt.Errorf("no dataset images linked to this iteration")
		}

		// Build dataset images list
		datasetImages := make([]vfrogapi.InferenceImageRef, 0, len(linkedImages))
		for _, link := range linkedImages {
			if dsImg, ok := link["dataset_images"].(map[string]interface{}); ok {
				imgID := ""
				if id, ok := link["dataset_image_id"].(string); ok {
					imgID = id
				}
				imgURL := ""
				if fileURL, ok := dsImg["file_url"].(string); ok && fileURL != "" {
					imgURL = fileURL
				}
				if imgID != "" && imgURL != "" {
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

		accessToken, err := getAccessToken(cfg)
		if err != nil {
			return fmt.Errorf("authentication required: %w. Run 'vfrog login' first", err)
		}

		ssatClient, err := vfrogapi.NewSSATClient(cfg, accessToken)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		params := vfrogapi.ControlParams{
			ProjectIterationID: iterationID,
			DatasetImages:      datasetImages,
			OrganisationID:     cfg.OrganisationID,
		}

		result, err := ssatClient.Control(params)
		if err != nil {
			return fmt.Errorf("failed to submit control: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(result)
		}

		output.PrintSuccess(fmt.Sprintf("Control submitted for iteration %s with %d dataset images", iterationID, len(datasetImages)))
		return nil
	},
}

// iterationsNextCmd represents the iterations next command
var iterationsNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Create the next iteration from the current one",
	Long: `Create the next iteration from the current iteration via the API project.

The given iteration must be the latest for this object. For iteration 2+,
the current iteration must have a trained model (model_id).

The new iteration will have:
- iteration_number = current + 1
- ssat_model_id = current iteration's model_id (for SSAT inference)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
		}

		// Get API key
		// Get access token (requires login)
		accessToken, err := getAccessToken(cfg)
		if err != nil {
			return fmt.Errorf("authentication required: %w. Run 'vfrog login' first", err)
		}

		// Create API client
		ssatClient, err := vfrogapi.NewSSATClient(cfg, accessToken)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Call API endpoint
		params := vfrogapi.NextIterationParams{
			IterationID: iterationID,
		}

		result, err := ssatClient.NextIteration(params)
		if err != nil {
			return fmt.Errorf("failed to create next iteration: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(result)
		}

		newIterationID := fmt.Sprintf("%v", result["iteration_id"])
		newIterationNumber := result["iteration_number"]
		output.PrintSuccess(fmt.Sprintf("Created iteration #%v (ID: %s)", newIterationNumber, newIterationID))
		return nil
	},
}

// iterationsRestartCmd represents the iterations restart command
var iterationsRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart an iteration (delete and recreate)",
	Long: `Restart an iteration by deleting it and recreating it via the API project.

This is only allowed if the iteration is the latest for this object
(no iteration with a higher iteration_number exists).

For iteration #1: Recreates with status "created".
For iteration #2+: Recreates with ssat_model_id from the previous iteration's model_id.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
		}

		// Get API key
		// Get access token (requires login)
		accessToken, err := getAccessToken(cfg)
		if err != nil {
			return fmt.Errorf("authentication required: %w. Run 'vfrog login' first", err)
		}

		// Create API client
		ssatClient, err := vfrogapi.NewSSATClient(cfg, accessToken)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Call API endpoint
		params := vfrogapi.RestartIterationParams{
			IterationID: iterationID,
		}

		result, err := ssatClient.RestartIteration(params)
		if err != nil {
			return fmt.Errorf("failed to restart iteration: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(result)
		}

		// Format as table similar to iterations list
		table := output.NewTable("ITERATION", "ID", "STATUS", "SSAT_MODEL_ID", "PREVIOUS_ID")
		iterationNumber := fmt.Sprintf("#%v", result["iteration_number"])
		newIterationID := fmt.Sprintf("%v", result["iteration_id"])
		status := "created"
		ssatModelID := "-"
		if modelID, ok := result["ssat_model_id"].(string); ok && modelID != "" {
			ssatModelID = modelID
		} else if modelID, ok := result["ssat_model_id"]; ok && modelID != nil {
			ssatModelID = fmt.Sprintf("%v", modelID)
		}
		previousID := fmt.Sprintf("%v", result["previous_id"])
		table.AddRow(iterationNumber, newIterationID, status, ssatModelID, previousID)
		table.Print()

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
	iterationsCmd.AddCommand(iterationsNextCmd)
	iterationsCmd.AddCommand(iterationsRestartCmd)
	iterationsCmd.AddCommand(iterationTrainCmd)
	iterationsCmd.AddCommand(iterationsAnnotationsCmd)
	iterationsCmd.AddCommand(iterationsStatusCmd)
	iterationsCmd.AddCommand(iterationsControlCmd)

	// Hidden alias: "vfrog iteration train" still works for backward compat
	iterationAliasCmd := &cobra.Command{Use: "iteration", Hidden: true}
	iterationAliasTrainCmd := &cobra.Command{
		Use:   iterationTrainCmd.Use,
		Short: iterationTrainCmd.Short,
		Long:  iterationTrainCmd.Long,
		RunE:  iterationTrainCmd.RunE,
	}
	iterationAliasTrainCmd.Flags().String("iteration_id", "", "Iteration ID to train")
	iterationAliasTrainCmd.Flags().Int("iteration_number", 0, "Iteration number")
	iterationAliasTrainCmd.Flags().String("object_id", "", "Object (product image) ID")
	iterationAliasCmd.AddCommand(iterationAliasTrainCmd)
	rootCmd.AddCommand(iterationAliasCmd)

	iterationsListCmd.Flags().String("object_id", "", "Object (product image) ID")
	iterationsCreateCmd.Flags().Int("random", 20, "Number of random dataset images to select")
	iterationsDeleteCmd.Flags().String("iteration_id", "", "Iteration ID to delete")
	iterationsDeleteCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsDeleteCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationsSSATCmd.Flags().String("iteration_id", "", "Iteration ID")
	iterationsSSATCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsSSATCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationsSSATCmd.Flags().Int("random", 0, "Randomly select N dataset images from the project (overrides default behavior)")
	iterationsSSATCmd.Flags().Bool("restart", false, "Restart the iteration before running SSAT")
	iterationsHaloCmd.Flags().String("iteration_id", "", "Iteration ID")
	iterationsHaloCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsHaloCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationTrainCmd.Flags().String("iteration_id", "", "Iteration ID to train")
	iterationTrainCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationTrainCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationsNextCmd.Flags().String("iteration_id", "", "Current iteration ID to create next from")
	iterationsNextCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsNextCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationsRestartCmd.Flags().String("iteration_id", "", "Iteration ID to restart")
	iterationsRestartCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsRestartCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationsAnnotationsCmd.Flags().String("iteration_id", "", "Iteration ID")
	iterationsAnnotationsCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsAnnotationsCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationsStatusCmd.Flags().String("iteration_id", "", "Iteration ID")
	iterationsStatusCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsStatusCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationsStatusCmd.Flags().Bool("watch", false, "Poll until iteration reaches terminal status")
	iterationsStatusCmd.Flags().Int("interval", 5, "Polling interval in seconds (used with --watch)")
	iterationsControlCmd.Flags().String("iteration_id", "", "Iteration ID")
	iterationsControlCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsControlCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
}

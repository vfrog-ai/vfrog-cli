package cmd

import (
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/vfrog-ai/vfrog-cli/internal/api/credits"
	"github.com/vfrog-ai/vfrog-cli/internal/api/supabase"
	"github.com/vfrog-ai/vfrog-cli/internal/api/vfrogapi"
	"github.com/vfrog-ai/vfrog-cli/internal/auth"
	"github.com/vfrog-ai/vfrog-cli/internal/config"
	"github.com/vfrog-ai/vfrog-cli/internal/output"
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

		if cfg.IsFreePlan() {
			return cfg.FreePlanError("SSAT annotation")
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

		// Reserve credits before processing
		creditsClient, err := credits.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create credits client: %w", err)
		}

		imageCount := len(datasetImageLinks)
		var toolType string
		var processID string
		if iterationNumber >= 2 {
			toolType = "run_inference"
			processID = fmt.Sprintf("run_inference_%s", iterationID)
		} else {
			toolType = "ssat"
			processID = fmt.Sprintf("ssat_%s", iterationID)
		}

		// Pre-check usage (validates billing, subscription, and balance)
		usage, err := creditsClient.GetUsage(credits.GetUsageParams{
			OrganisationID: cfg.OrganisationID,
			ActionType:     toolType,
			Units:          imageCount,
		})
		if err != nil {
			return fmt.Errorf("failed to check usage: %w", err)
		}
		if !usage.Allowed {
			if usage.Reason != "" {
				return fmt.Errorf("cannot proceed: %s. Estimated cost: %.2f credits (%d images). Current balance: %.2f credits",
					usage.Reason, usage.EstimatedActionCost, imageCount, usage.CreditsBalance)
			}
			return fmt.Errorf("cannot proceed: usage not allowed. Current balance: %.2f credits", usage.CreditsBalance)
		}

		if !jsonOutput {
			fmt.Printf("Estimated cost: %.2f credits (%d images x %.2f per image). Balance: %.2f credits\n",
				usage.EstimatedActionCost, imageCount, usage.PricePerUnit, usage.CreditsBalance)
		}

		// Reserve credits
		reservation, err := creditsClient.ReserveCredits(credits.ReserveParams{
			OrganisationID: cfg.OrganisationID,
			ToolType:       toolType,
			ProcessID:      processID,
			Units:          imageCount,
		})
		if err != nil {
			if errors.Is(err, credits.ErrInsufficientCredits) {
				return fmt.Errorf("insufficient credits. Estimated cost: %.2f credits (%d images x %.2f per image). Current balance: %.2f credits",
					usage.EstimatedActionCost, imageCount, usage.PricePerUnit, usage.CreditsBalance)
			}
			return fmt.Errorf("failed to reserve credits: %w", err)
		}

		if !jsonOutput {
			fmt.Printf("Credits reserved: %.2f (reservation: %s)\n", reservation.Amount, reservation.ReservationID)
		}

		// Save credit reservation ID to iteration
		if err := client.Patch("project_iteration", iterationID, map[string]interface{}{
			"credit_reservation_id": reservation.ReservationID,
		}); err != nil {
			if !jsonOutput {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save credit reservation ID: %v\n", err)
			}
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
		productImageID := iter["product_image_id"].(string)

		platformHost := cfg.PlatformHost
		if platformHost == "" {
			platformHost = "https://platform.vfrog.ai"
		}

		url := fmt.Sprintf("%s/org/%s/proj/%s/prod/%s/iter/%s/halo", platformHost, cfg.OrganisationID, projectID, productImageID, iterationID)

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

		if cfg.IsFreePlan() {
			return cfg.FreePlanError("Model training")
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
			"select":               "project_iteration_dataset_image_id,annotation,project_iteration_dataset_images!inner(dataset_image_id)",
		})
		if err != nil {
			return fmt.Errorf("failed to get annotated images: %w", err)
		}

		annotatedImages := make([]vfrogapi.AnnotatedImageRef, 0, len(annotatedImagesData))
		for _, annImg := range annotatedImagesData {
			var dsID string
			if pidsiData, ok := annImg["project_iteration_dataset_images"].(map[string]interface{}); ok {
				if id, ok := pidsiData["dataset_image_id"].(string); ok {
					dsID = id
				}
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

		// Reserve credits before training
		creditsClient, err := credits.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create credits client: %w", err)
		}

		trainImageCount := len(annotatedImages)

		// Pre-check usage (validates billing, subscription, and balance)
		usage, err := creditsClient.GetUsage(credits.GetUsageParams{
			OrganisationID: cfg.OrganisationID,
			ActionType:     "training",
			Units:          trainImageCount,
		})
		if err != nil {
			return fmt.Errorf("failed to check usage: %w", err)
		}
		if !usage.Allowed {
			if usage.Reason != "" {
				return fmt.Errorf("cannot proceed: %s. Estimated cost: %.2f credits (%d images). Current balance: %.2f credits",
					usage.Reason, usage.EstimatedActionCost, trainImageCount, usage.CreditsBalance)
			}
			return fmt.Errorf("cannot proceed: usage not allowed. Current balance: %.2f credits", usage.CreditsBalance)
		}

		if !jsonOutput {
			fmt.Printf("Estimated cost: %.2f credits (%d images x %.2f per image). Balance: %.2f credits\n",
				usage.EstimatedActionCost, trainImageCount, usage.PricePerUnit, usage.CreditsBalance)
		}

		// Reserve credits
		reservation, err := creditsClient.ReserveCredits(credits.ReserveParams{
			OrganisationID: cfg.OrganisationID,
			ToolType:       "training",
			ProcessID:      fmt.Sprintf("training_%s", iterationID),
			Units:          trainImageCount,
		})
		if err != nil {
			if errors.Is(err, credits.ErrInsufficientCredits) {
				return fmt.Errorf("insufficient credits. Estimated cost: %.2f credits (%d images x %.2f per image). Current balance: %.2f credits",
					usage.EstimatedActionCost, trainImageCount, usage.PricePerUnit, usage.CreditsBalance)
			}
			return fmt.Errorf("failed to reserve credits: %w", err)
		}

		if !jsonOutput {
			fmt.Printf("Credits reserved: %.2f (reservation: %s)\n", reservation.Amount, reservation.ReservationID)
		}

		// Save credit reservation ID to iteration
		if err := client.Patch("project_iteration", iterationID, map[string]interface{}{
			"credit_reservation_id": reservation.ReservationID,
		}); err != nil {
			if !jsonOutput {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save credit reservation ID: %v\n", err)
			}
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

	// Determine industry: flag > control check
	industry, _ := cmd.Flags().GetString("industry")
	if industry == "" {
		// Run control check to determine industry (like the platform does)
		controlDatasetImages := make([]vfrogapi.ControlImageRef, 0, len(datasetImages))
		for _, ds := range datasetImages {
			controlDatasetImages = append(controlDatasetImages, vfrogapi.ControlImageRef{
				ID:       ds.ID,
				ImageURL: ds.ImageURL,
			})
		}

		controlParams := vfrogapi.ControlParams{
			ProjectIterationID: iterationID,
			DatasetImages:      controlDatasetImages,
			OrganisationID:     cfg.OrganisationID,
		}

		if !jsonOutput {
			fmt.Println("Running control check to determine industry...")
		}

		controlResult, err := ssatClient.Control(controlParams)
		if err != nil {
			return fmt.Errorf("control check failed: %w. Use --industry to specify manually", err)
		}

		// Extract main_industry from control_json in the response
		if controlJSON, ok := controlResult["control_json"].(map[string]interface{}); ok {
			if mi, ok := controlJSON["main_industry"].(string); ok && mi != "" {
				industry = mi
			}
		}
		if industry == "" {
			return fmt.Errorf("control check did not return an industry. Use --industry to specify manually (e.g., --industry Manufacturing, --industry Retail)")
		}
		if !jsonOutput {
			fmt.Printf("Industry detected: %s\n", industry)
		}
	}

	// Save industry to project_iteration
	if err := client.Patch("project_iteration", iterationID, map[string]interface{}{
		"industry": industry,
	}); err != nil {
		// Non-fatal: continue even if save fails
		if !jsonOutput {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save industry to iteration: %v\n", err)
		}
	}

	// Submit batch via API project
	params := vfrogapi.BatchSubmitParams{
		ProjectIterationID: iterationID,
		OrganisationID:     cfg.OrganisationID,
		Industry:           industry,
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
			"industry":       industry,
			"method":         "ssat-batch",
		})
	}

	output.PrintSuccess(fmt.Sprintf("SSAT started for iteration %s with %d dataset images (industry: %s)", iterationID, len(datasetImages), industry))
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
	models, err := client.Get("model", map[string]string{
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

	// Build dataset images list
	datasetImages := make([]vfrogapi.InferenceImageRef, 0, len(datasetImageLinks))
	for _, link := range datasetImageLinks {
		if dsImg, ok := link["dataset_images"].(map[string]interface{}); ok {
			// Use project_iteration_dataset_images.id if available, fall back to dataset_image_id
			imgID := ""
			if id, ok := link["id"].(string); ok {
				imgID = id
			} else if id, ok := link["dataset_image_id"].(string); ok {
				imgID = id
			}
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
			"select":               "id,project_iteration_dataset_image_id,annotation,created_at,project_iteration_dataset_images(dataset_image_id)",
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
			dsID := ""
			if pidsiData, ok := img["project_iteration_dataset_images"].(map[string]interface{}); ok {
				if id, ok := pidsiData["dataset_image_id"].(string); ok {
					dsID = id
				}
			}
			if dsID == "" {
				dsID = fmt.Sprintf("%v", img["project_iteration_dataset_image_id"])
			}
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

		if cfg.IsFreePlan() {
			return cfg.FreePlanError("SSAT control")
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
		datasetImages := make([]vfrogapi.ControlImageRef, 0, len(linkedImages))
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
					datasetImages = append(datasetImages, vfrogapi.ControlImageRef{
						ID:       imgID,
						ImageURL: imgURL,
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

		if cfg.IsFreePlan() {
			return cfg.FreePlanError("Next iteration")
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

		if cfg.IsFreePlan() {
			return cfg.FreePlanError("Iteration restart")
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

// iterationsDeployCmd represents the iterations deploy command
var iterationsDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a trained model to production",
	Long: `Deploy a trained model to production by creating a class and model-class mapping.

The iteration must have trained_status 'completed' (training finished, model created).
This creates a class record, links it to the model, and updates trained_status to 'validated'.

Example:
  vfrog iterations deploy --iteration_id <id>
  vfrog iterations deploy --iteration_number 1 --object_id <id>`,
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

		if cfg.IsFreePlan() {
			return cfg.FreePlanError("Model deployment")
		}

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
		}

		// Get iteration details
		iterations, err := client.Get("project_iteration", map[string]string{
			"id":     fmt.Sprintf("eq.%s", iterationID),
			"select": "id,project_id,product_image_id,iteration_number,trained_status,model_id",
		})
		if err != nil {
			return fmt.Errorf("failed to get iteration: %w", err)
		}

		if len(iterations) == 0 {
			return fmt.Errorf("iteration not found: %s", iterationID)
		}

		iter := iterations[0]
		trainedStatus := ""
		if ts, ok := iter["trained_status"].(string); ok {
			trainedStatus = ts
		}

		if trainedStatus == "validated" {
			return fmt.Errorf("iteration is already deployed (trained_status: validated)")
		}

		if trainedStatus != "completed" {
			return fmt.Errorf("iteration must have trained_status 'completed' to deploy (current: %s)", trainedStatus)
		}

		modelID := ""
		if mid, ok := iter["model_id"].(string); ok {
			modelID = mid
		}
		if modelID == "" {
			return fmt.Errorf("iteration has no model_id. Training may not have completed successfully")
		}

		projectID := iter["project_id"].(string)
		productImageID := iter["product_image_id"].(string)
		iterationNumber := int(iter["iteration_number"].(float64))

		// Verify model exists
		models, err := client.Get("model", map[string]string{
			"id":     fmt.Sprintf("eq.%s", modelID),
			"select": "id,name",
		})
		if err != nil {
			return fmt.Errorf("failed to get model: %w", err)
		}

		if len(models) == 0 {
			return fmt.Errorf("model not found: %s", modelID)
		}

		// Get project title
		projects, err := client.Get("projects", map[string]string{
			"id":     fmt.Sprintf("eq.%s", projectID),
			"select": "title",
		})
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}

		if len(projects) == 0 {
			return fmt.Errorf("project not found: %s", projectID)
		}

		projectTitle := projects[0]["title"].(string)

		// Get product label
		productImages, err := client.Get("product_images", map[string]string{
			"id":     fmt.Sprintf("eq.%s", productImageID),
			"select": "label",
		})
		if err != nil {
			return fmt.Errorf("failed to get product image: %w", err)
		}

		productLabel := ""
		if len(productImages) > 0 {
			if l, ok := productImages[0]["label"].(string); ok {
				productLabel = l
			}
		}

		// Build class name (same format as platform)
		className := fmt.Sprintf("%s - Iter #%d", projectTitle, iterationNumber)
		if productLabel != "" {
			className = fmt.Sprintf("%s - %s - Iter #%d", projectTitle, productLabel, iterationNumber)
		}

		if !jsonOutput {
			fmt.Printf("Deploying model %s to production...\n", modelID)
			fmt.Printf("  Class name: %s\n", className)
		}

		// Create class record
		newClass, err := client.Post("class", map[string]interface{}{
			"name":            className,
			"organisation_id": cfg.OrganisationID,
			"project_id":      projectID,
			"description":     fmt.Sprintf("Validated model from iteration %d", iterationNumber),
		})
		if err != nil {
			return fmt.Errorf("failed to create class: %w", err)
		}

		classID := newClass["id"].(string)

		// Create model-class mapping
		_, err = client.Post("model_class_mapping", map[string]interface{}{
			"model_id": modelID,
			"class_id": classID,
		})
		if err != nil {
			return fmt.Errorf("failed to create model-class mapping: %w", err)
		}

		// Update trained_status to validated
		if err := client.Patch("project_iteration", iterationID, map[string]interface{}{
			"trained_status": "validated",
		}); err != nil {
			return fmt.Errorf("failed to update iteration status: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]interface{}{
				"iteration_id": iterationID,
				"model_id":     modelID,
				"class_id":     classID,
				"class_name":   className,
				"status":       "validated",
			})
		}

		output.PrintSuccess(fmt.Sprintf("Model deployed to production (class: %s, class_id: %s)", className, classID))
		return nil
	},
}

// iterationsManualCmd opens the manual annotation page for an iteration
var iterationsManualCmd = &cobra.Command{
	Use:   "manual",
	Short: "Open manual annotation for an iteration",
	Long:  `Opens the platform annotation page for the given iteration so you can manually annotate dataset images.`,
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

		client, err := supabase.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to create Supabase client: %w", err)
		}

		iterationID, err := getIterationID(cmd, cfg, client)
		if err != nil {
			return err
		}

		// Fetch iteration to get the product_image_id
		iterations, err := client.Get("project_iteration", map[string]string{
			"id":     fmt.Sprintf("eq.%s", iterationID),
			"select": "id,product_image_id",
		})
		if err != nil {
			return fmt.Errorf("failed to get iteration: %w", err)
		}
		if len(iterations) == 0 {
			return fmt.Errorf("iteration not found: %s", iterationID)
		}

		productImageID, ok := iterations[0]["product_image_id"].(string)
		if !ok || productImageID == "" {
			return fmt.Errorf("iteration has no product image linked")
		}

		host := cfg.PlatformHost
		if host == "" {
			host = config.DefaultPlatformHost
		}

		annotateURL := fmt.Sprintf("%s/org/%s/proj/%s/prod/%s/iter/%s/annotate",
			host, cfg.OrganisationID, cfg.ProjectID, productImageID, iterationID)

		if jsonOutput {
			return output.PrintJSON(map[string]string{
				"url":    annotateURL,
				"status": "success",
			})
		}

		fmt.Println(annotateURL)

		// Try to open the URL in the default browser
		var openErr error
		switch runtime.GOOS {
		case "windows":
			openErr = exec.Command("cmd", "/c", "start", annotateURL).Start()
		case "darwin":
			openErr = exec.Command("open", annotateURL).Start()
		default:
			openErr = exec.Command("xdg-open", annotateURL).Start()
		}
		if openErr != nil {
			fmt.Println("Could not open browser automatically. Please open the URL above.")
		}

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
	iterationsCmd.AddCommand(iterationsDeployCmd)
	iterationsCmd.AddCommand(iterationsManualCmd)

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
	iterationsSSATCmd.Flags().String("industry", "", "Industry for SSAT processing (e.g., Retail, Agriculture, Aquaculture, Manufacturing). If not set, runs control check to detect automatically")
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
	iterationsDeployCmd.Flags().String("iteration_id", "", "Iteration ID to deploy")
	iterationsDeployCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsDeployCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
	iterationsManualCmd.Flags().String("iteration_id", "", "Iteration ID")
	iterationsManualCmd.Flags().Int("iteration_number", 0, "Iteration number (uses object_id from config if not provided)")
	iterationsManualCmd.Flags().String("object_id", "", "Object (product image) ID (uses config value if not provided)")
}

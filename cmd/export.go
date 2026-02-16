package cmd

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vfrog-ai/vfrog-cli/internal/api/supabase"
	"github.com/vfrog-ai/vfrog-cli/internal/config"
	"github.com/vfrog-ai/vfrog-cli/internal/output"
)

// exportCmd represents the export command group
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export annotations and datasets",
	Long:  `Export annotations and datasets in various formats (YOLO, JSON).`,
}

// exportYoloCmd represents the export yolo command
var exportYoloCmd = &cobra.Command{
	Use:   "yolo",
	Short: "Export annotations in YOLO format",
	Long: `Export annotations for an iteration in YOLO format.

Creates the following structure:
  {output}/images/     - Downloaded dataset images
  {output}/labels/     - YOLO annotation files (class_id center_x center_y width height)
  {output}/data.yaml   - Class names and train/val split configuration

Example:
  vfrog export yolo --iteration_id <id> --output ./export
  vfrog export yolo --iteration_id <id> --output ./export --zip`,
	RunE: func(cmd *cobra.Command, args []string) error {
		iterationID, _ := cmd.Flags().GetString("iteration_id")
		outputDir, _ := cmd.Flags().GetString("output")
		createZip, _ := cmd.Flags().GetBool("zip")

		if iterationID == "" {
			return fmt.Errorf("--iteration_id is required")
		}

		if outputDir == "" {
			outputDir = "./export"
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

		// Get iteration details to find the product image (for class labels)
		iterations, err := client.Get("project_iteration", map[string]string{
			"id":     fmt.Sprintf("eq.%s", iterationID),
			"select": "id,product_image_id,project_id",
		})
		if err != nil {
			return fmt.Errorf("failed to get iteration: %w", err)
		}

		if len(iterations) == 0 {
			return fmt.Errorf("iteration not found: %s", iterationID)
		}

		productImageID := iterations[0]["product_image_id"].(string)

		// Get product image for class labels
		productImages, err := client.Get("product_images", map[string]string{
			"id":     fmt.Sprintf("eq.%s", productImageID),
			"select": "id,label",
		})
		if err != nil {
			return fmt.Errorf("failed to get product image: %w", err)
		}

		classNames := []string{"object"}
		if len(productImages) > 0 {
			if label, ok := productImages[0]["label"].(string); ok && label != "" {
				classNames = []string{label}
			}
		}

		// Get annotated images with joined dataset image info
		annotatedImages, err := client.Get("project_iteration_annotated_images", map[string]string{
			"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
			"select":               "id,project_iteration_dataset_image_id,annotation,project_iteration_dataset_images(dataset_image_id)",
		})
		if err != nil {
			return fmt.Errorf("failed to get annotated images: %w", err)
		}

		if len(annotatedImages) == 0 {
			return fmt.Errorf("no annotated images found for this iteration")
		}

		// Resolve the real dataset_image_id via the join table
		dsImageIDs := make([]string, 0, len(annotatedImages))
		for _, ai := range annotatedImages {
			dsID := ""
			if pidsiData, ok := ai["project_iteration_dataset_images"].(map[string]interface{}); ok {
				if id, ok := pidsiData["dataset_image_id"].(string); ok {
					dsID = id
				}
			}
			if dsID != "" {
				dsImageIDs = append(dsImageIDs, dsID)
			}
		}

		// Fetch dataset images in batch
		dsImages := make(map[string]map[string]interface{})
		for _, dsID := range dsImageIDs {
			imgs, err := client.Get("dataset_images", map[string]string{
				"id":     fmt.Sprintf("eq.%s", dsID),
				"select": "id,filename,file_url",
			})
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to get dataset image %s: %v\n", dsID, err)
				continue
			}
			if len(imgs) > 0 {
				dsImages[dsID] = imgs[0]
			}
		}

		// Create output directories
		imagesDir := filepath.Join(outputDir, "images")
		labelsDir := filepath.Join(outputDir, "labels")
		if err := os.MkdirAll(imagesDir, 0755); err != nil {
			return fmt.Errorf("failed to create images directory: %w", err)
		}
		if err := os.MkdirAll(labelsDir, 0755); err != nil {
			return fmt.Errorf("failed to create labels directory: %w", err)
		}

		// Process each annotated image
		var imageFiles []string
		httpClient := &http.Client{Timeout: 30 * time.Second}

		for _, ai := range annotatedImages {
			dsID := ""
			if pidsiData, ok := ai["project_iteration_dataset_images"].(map[string]interface{}); ok {
				if id, ok := pidsiData["dataset_image_id"].(string); ok {
					dsID = id
				}
			}
			if dsID == "" {
				continue
			}

			dsImg, exists := dsImages[dsID]
			if !exists {
				continue
			}

			fileURL, _ := dsImg["file_url"].(string)
			filename, _ := dsImg["filename"].(string)
			if fileURL == "" || filename == "" {
				continue
			}

			// Ensure valid filename
			baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
			if baseName == "" {
				baseName = dsID
			}

			// Download image
			imgPath := filepath.Join(imagesDir, filename)
			resp, err := httpClient.Get(fileURL)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to download %s: %v\n", filename, err)
				continue
			}

			imgFile, err := os.Create(imgPath)
			if err != nil {
				resp.Body.Close()
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to create %s: %v\n", imgPath, err)
				continue
			}

			_, err = io.Copy(imgFile, resp.Body)
			resp.Body.Close()
			imgFile.Close()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to write %s: %v\n", imgPath, err)
				continue
			}

			// Convert annotations to YOLO format
			annotations, ok := ai["annotation"].([]interface{})
			if !ok || len(annotations) == 0 {
				continue
			}

			labelPath := filepath.Join(labelsDir, baseName+".txt")
			labelFile, err := os.Create(labelPath)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to create label file %s: %v\n", labelPath, err)
				continue
			}

			for _, ann := range annotations {
				annMap, ok := ann.(map[string]interface{})
				if !ok {
					continue
				}

				// Extract bounding box coordinates (normalized 0-1)
				x, _ := annMap["x"].(float64)
				y, _ := annMap["y"].(float64)
				w, _ := annMap["width"].(float64)
				h, _ := annMap["height"].(float64)

				// Convert to YOLO format: class_id center_x center_y width height
				centerX := x + w/2
				centerY := y + h/2
				classID := 0

				fmt.Fprintf(labelFile, "%d %.6f %.6f %.6f %.6f\n", classID, centerX, centerY, w, h)
			}
			labelFile.Close()

			imageFiles = append(imageFiles, filename)
		}

		if len(imageFiles) == 0 {
			return fmt.Errorf("no images were exported successfully")
		}

		// Generate data.yaml with 90/10 train/val split
		rand.Seed(time.Now().UnixNano())
		shuffled := make([]string, len(imageFiles))
		copy(shuffled, imageFiles)
		rand.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		splitIdx := int(float64(len(shuffled)) * 0.9)
		if splitIdx == 0 {
			splitIdx = 1
		}

		yamlContent := fmt.Sprintf("# vfrog YOLO export\n# Generated: %s\n\ntrain: ./images\nval: ./images\n\nnc: %d\nnames: [%s]\n\n# Train/Val split (90/10)\n# Train images: %d\n# Val images: %d\n",
			time.Now().Format(time.RFC3339),
			len(classNames),
			"'"+strings.Join(classNames, "', '")+"'",
			splitIdx,
			len(shuffled)-splitIdx,
		)

		yamlPath := filepath.Join(outputDir, "data.yaml")
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			return fmt.Errorf("failed to write data.yaml: %w", err)
		}

		// Create ZIP if requested
		if createZip {
			zipPath := outputDir + ".zip"
			if err := createZipArchive(outputDir, zipPath); err != nil {
				return fmt.Errorf("failed to create ZIP archive: %w", err)
			}

			if jsonOutput {
				return output.PrintJSON(map[string]interface{}{
					"format":       "yolo",
					"images":       len(imageFiles),
					"output":       outputDir,
					"zip":          zipPath,
					"class_names":  classNames,
					"iteration_id": iterationID,
				})
			}

			output.PrintSuccess(fmt.Sprintf("Exported %d images in YOLO format to %s (ZIP: %s)", len(imageFiles), outputDir, zipPath))
			return nil
		}

		if jsonOutput {
			return output.PrintJSON(map[string]interface{}{
				"format":       "yolo",
				"images":       len(imageFiles),
				"output":       outputDir,
				"class_names":  classNames,
				"iteration_id": iterationID,
			})
		}

		output.PrintSuccess(fmt.Sprintf("Exported %d images in YOLO format to %s", len(imageFiles), outputDir))
		return nil
	},
}

// exportJSONCmd represents the export json command
var exportJSONCmd = &cobra.Command{
	Use:   "json",
	Short: "Export annotations in JSON format",
	Long: `Export all annotated images with full annotation data as JSON.

Example:
  vfrog export json --iteration_id <id> --output ./annotations.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		iterationID, _ := cmd.Flags().GetString("iteration_id")
		outputPath, _ := cmd.Flags().GetString("output")

		if iterationID == "" {
			return fmt.Errorf("--iteration_id is required")
		}

		if outputPath == "" {
			outputPath = "./annotations.json"
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

		// Get annotated images with full annotation data
		annotatedImages, err := client.Get("project_iteration_annotated_images", map[string]string{
			"project_iteration_id": fmt.Sprintf("eq.%s", iterationID),
			"select":               "id,project_iteration_dataset_image_id,annotation,created_at,project_iteration_dataset_images(dataset_image_id)",
		})
		if err != nil {
			return fmt.Errorf("failed to get annotated images: %w", err)
		}

		if len(annotatedImages) == 0 {
			return fmt.Errorf("no annotated images found for this iteration")
		}

		// Enrich with dataset image info via the join table
		for i, ai := range annotatedImages {
			dsID := ""
			if pidsiData, ok := ai["project_iteration_dataset_images"].(map[string]interface{}); ok {
				if id, ok := pidsiData["dataset_image_id"].(string); ok {
					dsID = id
				}
			}
			if dsID == "" {
				continue
			}

			imgs, err := client.Get("dataset_images", map[string]string{
				"id":     fmt.Sprintf("eq.%s", dsID),
				"select": "id,filename,file_url",
			})
			if err == nil && len(imgs) > 0 {
				annotatedImages[i]["dataset_image"] = imgs[0]
			}
		}

		exportData := map[string]interface{}{
			"iteration_id": iterationID,
			"exported_at":  time.Now().Format(time.RFC3339),
			"count":        len(annotatedImages),
			"annotations":  annotatedImages,
		}

		jsonData, err := json.MarshalIndent(exportData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}

		// Ensure output directory exists
		dir := filepath.Dir(outputPath)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write JSON file: %w", err)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]interface{}{
				"format":       "json",
				"annotations":  len(annotatedImages),
				"output":       outputPath,
				"iteration_id": iterationID,
			})
		}

		output.PrintSuccess(fmt.Sprintf("Exported %d annotations to %s", len(annotatedImages), outputPath))
		return nil
	},
}

// createZipArchive creates a ZIP archive from a directory
func createZipArchive(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Use forward slashes in ZIP
		relPath = filepath.ToSlash(relPath)

		f, err := w.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(f, file)
		return err
	})
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.AddCommand(exportYoloCmd)
	exportCmd.AddCommand(exportJSONCmd)

	// YOLO flags
	exportYoloCmd.Flags().String("iteration_id", "", "Iteration ID to export")
	exportYoloCmd.Flags().String("output", "./export", "Output directory")
	exportYoloCmd.Flags().Bool("zip", false, "Create ZIP archive after export")

	// JSON flags
	exportJSONCmd.Flags().String("iteration_id", "", "Iteration ID to export")
	exportJSONCmd.Flags().String("output", "./annotations.json", "Output file path")
}

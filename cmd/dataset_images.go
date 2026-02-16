package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/vfrog-ai/vfrog-cli/internal/api/storage"
	"github.com/vfrog-ai/vfrog-cli/internal/api/supabase"
	"github.com/vfrog-ai/vfrog-cli/internal/auth"
	"github.com/vfrog-ai/vfrog-cli/internal/config"
	"github.com/vfrog-ai/vfrog-cli/internal/output"
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
	Short: "Upload dataset images from URLs or local files",
	Long: `Upload dataset images to your project from URLs or local files.

URLs must be persistent and publicly accessible, as images are referenced by URL
and not stored on vfrog servers. Ensure URLs do not expire or require authentication.

Use --file or --dir flags to upload local files (uploaded to S3 via signed URL).
Use 'dataset_images import --csv' for bulk import from CSV files.

Examples:
  vfrog dataset_images upload https://example.com/img1.jpg https://example.com/img2.jpg
  vfrog dataset_images upload --file ./photo.jpg
  vfrog dataset_images upload --dir ./images/`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		dirPath, _ := cmd.Flags().GetString("dir")

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

		// Handle local file upload
		if filePath != "" || dirPath != "" {
			var filePaths []string

			if filePath != "" {
				filePaths = append(filePaths, filePath)
			}

			if dirPath != "" {
				entries, err := os.ReadDir(dirPath)
				if err != nil {
					return fmt.Errorf("failed to read directory: %w", err)
				}
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					ext := strings.ToLower(filepath.Ext(entry.Name()))
					if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" || ext == ".gif" || ext == ".bmp" {
						filePaths = append(filePaths, filepath.Join(dirPath, entry.Name()))
					}
				}
			}

			if len(filePaths) == 0 {
				return fmt.Errorf("no image files found")
			}

			for _, fp := range filePaths {
				uploadResult, err := storage.UploadFile(cfg, accessToken, "dataset-images", fp)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to upload %s: %v\n", fp, err)
					continue
				}

				filename := filepath.Base(fp)
				imageData := map[string]interface{}{
					"project_id": cfg.ProjectID,
					"user_id":    userID,
					"filename":   filename,
					"file_path":  uploadResult.FilePath,
					"file_url":   uploadResult.FileURL,
				}

				result, err := client.Post("dataset_images", imageData)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to create record for %s: %v\n", fp, err)
					continue
				}

				results = append(results, result)
			}
		} else {
			// Handle URL upload — download and re-upload to vfrog CDN (5 at a time)
			if len(args) == 0 {
				return fmt.Errorf("provide URLs as arguments, or use --file/--dir for local files")
			}

			type uploadResult struct {
				result   map[string]interface{}
				imageURL string
				err      string
			}

			var mu sync.Mutex
			sem := make(chan struct{}, 5)
			var wg sync.WaitGroup

			for _, imageURL := range args {
				if _, err := url.Parse(imageURL); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: invalid URL: %s\n", imageURL)
					continue
				}

				wg.Add(1)
				go func(imgURL string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					filename := imgURL
					if parsedURL, err := url.Parse(imgURL); err == nil {
						pathParts := strings.Split(parsedURL.Path, "/")
						if len(pathParts) > 0 && pathParts[len(pathParts)-1] != "" {
							filename = pathParts[len(pathParts)-1]
						}
					}

					uploadResult, err := storage.UploadFromURL(cfg, accessToken, "dataset-images", imgURL)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to upload %s: %v\n", imgURL, err)
						return
					}

					imageData := map[string]interface{}{
						"project_id": cfg.ProjectID,
						"user_id":    userID,
						"filename":   filename,
						"file_path":  uploadResult.FilePath,
						"file_url":   uploadResult.FileURL,
					}

					result, err := client.Post("dataset_images", imageData)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to create record for %s: %v\n", imgURL, err)
						return
					}

					mu.Lock()
					results = append(results, result)
					mu.Unlock()

					if !jsonOutput {
						fmt.Printf("  Uploaded: %s\n", filename)
					}
				}(imageURL)
			}
			wg.Wait()
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

// datasetImagesImportCmd represents the dataset_images import command
var datasetImagesImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import dataset images from CSV",
	Long: `Import dataset images from a CSV file.

CSV columns (with header row): image_url, external_id, label
Only image_url is required. Images are uploaded in batches of 5.

Example:
  vfrog dataset_images import --csv ./images.csv`,
	RunE: func(cmd *cobra.Command, args []string) error {
		csvPath, _ := cmd.Flags().GetString("csv")
		if csvPath == "" {
			return fmt.Errorf("--csv is required")
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

		// Parse CSV
		file, err := os.Open(csvPath)
		if err != nil {
			return fmt.Errorf("failed to open CSV file: %w", err)
		}
		defer file.Close()

		reader := csv.NewReader(file)

		// Read header
		header, err := reader.Read()
		if err != nil {
			return fmt.Errorf("failed to read CSV header: %w", err)
		}

		// Build column index map (case-insensitive)
		colIdx := make(map[string]int)
		for i, col := range header {
			colIdx[strings.ToLower(strings.TrimSpace(col))] = i
		}

		urlCol, hasURL := colIdx["image_url"]
		if !hasURL {
			return fmt.Errorf("CSV must have an 'image_url' column")
		}

		extIDCol, hasExtID := colIdx["external_id"]
		labelCol, hasLabel := colIdx["label"]

		// Parse all rows first
		type csvRow struct {
			rowNum   int
			imageURL string
			filename string
			extID    string
			label    string
		}

		var rows []csvRow
		var parseErrors []string
		rowNum := 1

		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("row %d: %v", rowNum+1, err))
				rowNum++
				continue
			}

			rowNum++

			if urlCol >= len(record) {
				parseErrors = append(parseErrors, fmt.Sprintf("row %d: missing image_url column", rowNum))
				continue
			}

			imageURL := strings.TrimSpace(record[urlCol])
			if imageURL == "" {
				parseErrors = append(parseErrors, fmt.Sprintf("row %d: empty image_url", rowNum))
				continue
			}

			if _, err := url.Parse(imageURL); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("row %d: invalid URL: %s", rowNum, imageURL))
				continue
			}

			filename := imageURL
			if parsedURL, err := url.Parse(imageURL); err == nil {
				pathParts := strings.Split(parsedURL.Path, "/")
				if len(pathParts) > 0 && pathParts[len(pathParts)-1] != "" {
					filename = pathParts[len(pathParts)-1]
				}
			}

			row := csvRow{rowNum: rowNum, imageURL: imageURL, filename: filename}
			if hasExtID && extIDCol < len(record) {
				row.extID = strings.TrimSpace(record[extIDCol])
			}
			if hasLabel && labelCol < len(record) {
				row.label = strings.TrimSpace(record[labelCol])
			}
			rows = append(rows, row)
		}

		// Upload in parallel (5 at a time)
		var results []map[string]interface{}
		var errors []string
		var mu sync.Mutex
		sem := make(chan struct{}, 5)
		var wg sync.WaitGroup

		errors = append(errors, parseErrors...)

		for _, row := range rows {
			wg.Add(1)
			go func(r csvRow) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				uploadResult, err := storage.UploadFromURL(cfg, accessToken, "dataset-images", r.imageURL)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("row %d (%s): failed to upload: %v", r.rowNum, r.filename, err))
					mu.Unlock()
					return
				}

				imageData := map[string]interface{}{
					"project_id": cfg.ProjectID,
					"user_id":    userID,
					"filename":   r.filename,
					"file_path":  uploadResult.FilePath,
					"file_url":   uploadResult.FileURL,
				}

				if r.extID != "" {
					imageData["external_id"] = r.extID
				}
				if r.label != "" {
					imageData["label"] = r.label
				}

				result, err := client.Post("dataset_images", imageData)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("row %d (%s): %v", r.rowNum, r.filename, err))
					mu.Unlock()
					return
				}

				mu.Lock()
				results = append(results, result)
				mu.Unlock()

				if !jsonOutput {
					fmt.Printf("  Imported: %s\n", r.filename)
				}
			}(row)
		}
		wg.Wait()

		if jsonOutput {
			return output.PrintJSON(map[string]interface{}{
				"imported": len(results),
				"errors":   len(errors),
				"results":  results,
			})
		}

		output.PrintSuccess(fmt.Sprintf("Imported %d dataset image(s) from CSV", len(results)))
		if len(errors) > 0 {
			fmt.Printf("Errors (%d):\n", len(errors))
			for _, e := range errors {
				fmt.Printf("  - %s\n", e)
			}
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
	datasetImagesCmd.AddCommand(datasetImagesImportCmd)

	datasetImagesUploadCmd.Flags().String("file", "", "Local image file to upload")
	datasetImagesUploadCmd.Flags().String("dir", "", "Directory of local image files to upload")
	datasetImagesDeleteCmd.Flags().String("dataset_image_id", "", "Dataset image ID to delete")
	datasetImagesImportCmd.Flags().String("csv", "", "CSV file path to import")
}

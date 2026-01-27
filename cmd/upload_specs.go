package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/momorph/cli/internal/auth"
	"github.com/momorph/cli/internal/graphql"
	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/upload"
	"github.com/spf13/cobra"
)

var (
	specUploadDir       string
	specUploadRecursive bool
	specUploadDryRun    bool
	specUploadContinue  bool
)

// CSV columns are mapped to spec fields:
//
//	No -> no, itemName -> design_item_name, nameJP -> name, nameTrans -> nameTrans,
//	itemId -> node_link_id, itemType -> type, itemSubtype -> otherType,
//	buttonType -> buttonType, dataType -> dataType, required -> required,
//	format -> format, minLength -> minLength, maxLength -> maxLength,
//	defaultValue -> defaultValue, validationNote -> validationNote,
//	userAction -> action, transitionNote -> navigationNote,
//	databaseTable -> tableName, databaseColumn -> columnName,
//	databaseNote -> databaseNote, description -> description
var uploadSpecsCmd = &cobra.Command{
	Use:   "specs [files...]",
	Short: "Upload specs to MoMorph server",
	Long: `Upload spec CSV files to MoMorph server.

Files must follow the path pattern:
  .momorph/specs/{file_key}/{frame_id}-{frame_name}.csv
`,
	Example: `  # Upload a single file
  momorph upload specs .momorph/specs/xxx/yyy.csv

  # Upload multiple files
  momorph upload specs file1.csv file2.csv

  # Upload all specs in a directory recursively
  momorph upload specs --dir .momorph/specs/ -r

  # Upload using glob pattern
  momorph upload specs ".momorph/specs/**/*.csv"

  # Dry run (show what would be uploaded)
  momorph upload specs --dry-run .momorph/specs/**/*.csv`,
	RunE: runUploadSpecs,
}

func init() {
	uploadSpecsCmd.Flags().StringVarP(&specUploadDir, "dir", "d", "", "Directory to search for CSV files")
	uploadSpecsCmd.Flags().BoolVarP(&specUploadRecursive, "recursive", "r", false, "Search directories recursively")
	uploadSpecsCmd.Flags().BoolVar(&specUploadDryRun, "dry-run", false, "Show what would be uploaded without actually uploading")
	uploadSpecsCmd.Flags().BoolVar(&specUploadContinue, "continue-on-error", false, "Continue uploading remaining files if one fails")
	uploadCmd.AddCommand(uploadSpecsCmd)
}

func runUploadSpecs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Signal handling for graceful cancellation
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n✗ Upload cancelled")
		cancel()
		os.Exit(0)
	}()

	// Check authentication
	if !auth.IsAuthenticated() {
		fmt.Println("✗ Not authenticated")
		fmt.Println("\nRun 'momorph login' to authenticate before uploading")
		return nil
	}

	// Get actor email for revision tracking
	actor, err := getActorEmail()
	if err != nil {
		logger.Warn("Failed to get user email: %v", err)
		fmt.Println("⚠ Could not get user email for revision tracking")
	}

	// Resolve files
	files, err := upload.ResolveFiles(args, specUploadDir, specUploadRecursive, "specs")
	if err != nil {
		return fmt.Errorf("failed to resolve files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No CSV files found to upload")
		fmt.Println("\nMake sure files are in the correct path format:")
		fmt.Println("  .momorph/specs/{file_key}/{frame_id}-{frame_name}.csv")
		return nil
	}

	// Validate files
	validFiles, skipped := upload.ValidateFiles(files, "specs")

	// Print skipped files
	for _, s := range skipped {
		fmt.Printf("  [SKIPPED] %s\n", s.FileName)
		fmt.Printf("    Reason: %s\n", s.Message)
	}

	if len(validFiles) == 0 {
		fmt.Println("\nNo valid files to upload")
		return nil
	}

	// Dry run mode
	if specUploadDryRun {
		fmt.Printf("\n[DRY RUN] Would upload %d file(s):\n", len(validFiles))
		for _, f := range validFiles {
			parsed, _ := upload.ParseFilePath(f)
			specs, _ := upload.ParseSpecsCSV(f)
			fmt.Printf("  - %s\n", filepath.Base(f))
			fmt.Printf("    File Key: %s\n", parsed.FileKey)
			fmt.Printf("    Frame ID: %s\n", parsed.FrameID)
			fmt.Printf("    Frame Name: %s\n", parsed.FrameName)
			fmt.Printf("    Specs count: %d\n", len(specs))
		}
		return nil
	}

	// Create GraphQL client
	client, err := graphql.NewClient()
	if err != nil {
		logger.Error("Failed to create GraphQL client", err)
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Upload files
	fmt.Printf("\nUploading %d spec file(s)...\n", len(validFiles))
	results := uploadSpecFiles(ctx, client, validFiles, actor, specUploadContinue)

	// Combine with skipped files
	allResults := append(skipped, results...)

	// Display summary
	displayUploadSummary(allResults)

	return nil
}

func uploadSpecFiles(ctx context.Context, client *graphql.Client, files []string, actor string, continueOnError bool) []upload.UploadResult {
	var results []upload.UploadResult

	for i, file := range files {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return results
		default:
		}

		fileName := filepath.Base(file)
		fmt.Printf("  [%d/%d] %s ", i+1, len(files), fileName)

		result := uploadSingleSpecFile(ctx, client, file, actor)
		results = append(results, result)

		switch result.Status {
		case upload.StatusSuccess:
			fmt.Println(".... done")
		case upload.StatusFailed:
			fmt.Println(".... failed")
			fmt.Printf("    Error: %s\n", result.Message)
			if !continueOnError {
				return results
			}
		case upload.StatusSkipped:
			fmt.Println(".... skipped")
			fmt.Printf("    Reason: %s\n", result.Message)
		}
	}

	return results
}

func uploadSingleSpecFile(ctx context.Context, client *graphql.Client, filePath, actor string) upload.UploadResult {
	fileName := filepath.Base(filePath)

	// Parse file path
	parsed, err := upload.ParseFilePath(filePath)
	if err != nil {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusSkipped,
			Error:    err,
			Message:  "Invalid file path format",
		}
	}

	// Parse CSV file
	specs, err := upload.ParseSpecsCSV(filePath)
	if err != nil {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusFailed,
			Error:    err,
			Message:  fmt.Sprintf("Failed to parse CSV: %v", err),
		}
	}

	if len(specs) == 0 {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusSkipped,
			Message:  "CSV file contains no specs",
		}
	}

	logger.Debug("Parsed %d specs from %s", len(specs), fileName)

	// Get frame to validate and get IDs
	frame, err := client.GetFrame(ctx, parsed.FileKey, parsed.FrameID)
	if err != nil {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusFailed,
			Error:    err,
			Message:  fmt.Sprintf("Frame not found: %v", err),
		}
	}

	// Check frame status
	if frame.Status == "design" {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusFailed,
			Message:  "Cannot upload specs to frame in 'design' status",
		}
	}

	// Get node link IDs from specs
	var nodeLinkIds []string
	for _, spec := range specs {
		if spec.NodeLinkID != "" {
			nodeLinkIds = append(nodeLinkIds, spec.NodeLinkID)
		}
	}

	// Get existing design items for comparison
	var existingItems []graphql.DesignItem
	if len(nodeLinkIds) > 0 {
		existingItems, err = client.ListDesignItemsByNodeLinkIds(ctx, parsed.FileKey, parsed.FrameID, nodeLinkIds)
		if err != nil {
			logger.Debug("Failed to get existing design items: %v", err)
		}
	}

	// Build map of existing items by node_link_id
	existingMap := make(map[string]graphql.DesignItem)
	for _, item := range existingItems {
		existingMap[item.NodeLinkID] = item
	}

	// Prepare items for upsert
	var items []map[string]interface{}
	for _, spec := range specs {
		payload := upload.TransformSpecToPayload(spec, frame.ID, frame.FileID)

		// Convert to map for GraphQL
		// Note: section_link_id is required for the unique constraint
		// design_items_section_link_id_node_link_id_file_id_key
		item := map[string]interface{}{
			"no":              payload.No,
			"name":            payload.Name,
			"type":            payload.Type,
			"node_link_id":    payload.NodeLinkID,
			"section_link_id": payload.SectionLinkID, // Required for upsert constraint
			"frame_id":        payload.FrameID,
			"file_id":         payload.FileID,
			"status":          "draft",
		}

		if payload.Specs != nil {
			specsJSON, _ := json.Marshal(payload.Specs)
			item["specs"] = json.RawMessage(specsJSON)
		}

		items = append(items, item)
	}

	// Upsert design items
	savedItems, err := client.UpsertDesignItemSpecs(ctx, items)
	if err != nil {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusFailed,
			Error:    err,
			Message:  fmt.Sprintf("Failed to upsert specs: %v", err),
		}
	}

	logger.Debug("Upserted %d design items", len(savedItems))

	// Create revisions if actor is available
	if actor != "" {
		user, err := client.GetMorpheusUserByEmail(ctx, actor)
		if err == nil && user != nil {
			// Prepare revision entries
			var revs []map[string]interface{}
			for _, item := range savedItems {
				// Check if item was updated (existed before)
				if _, existed := existingMap[item.NodeLinkID]; existed {
					rev := map[string]interface{}{
						"design_item_id": item.ID,
						"status":         item.Status,
						"specs":          item.Specs,
						"type":           item.Type,
						"change_type":    "user",
						"name":           item.Name,
						"user_id":        user.ID,
					}
					revs = append(revs, rev)
				}
			}

			if len(revs) > 0 {
				affectedRows, err := client.InsertDesignItemRevs(ctx, revs)
				if err != nil {
					logger.Warn("Failed to insert revisions: %v", err)
				} else {
					logger.Debug("Inserted %d revisions", affectedRows)
				}
			}
		} else {
			logger.Debug("Could not get user for revision tracking: %v", err)
		}
	}

	return upload.UploadResult{
		FilePath: filePath,
		FileName: fileName,
		Status:   upload.StatusSuccess,
		Message:  fmt.Sprintf("Uploaded %d specs", len(savedItems)),
	}
}

// getActorEmail gets the authenticated user's email from MoMorph API
func getActorEmail() (string, error) {
	token, err := auth.LoadToken()
	if err != nil {
		return "", fmt.Errorf("not authenticated: %w", err)
	}

	ctx := context.Background()
	user, err := auth.GetMoMorphUser(ctx, token.GitHubToken)
	if err != nil {
		return "", err
	}
	return user.Email, nil
}

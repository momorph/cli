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

	// Check frame status (matches SDK's inDesignFrame check)
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

	if len(nodeLinkIds) == 0 {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusFailed,
			Message:  "No valid node link IDs provided",
		}
	}

	// Get existing design items for comparison
	var existingItems []graphql.DesignItem
	existingItems, err = client.ListDesignItemsByNodeLinkIds(ctx, parsed.FileKey, parsed.FrameID, nodeLinkIds)
	if err != nil {
		logger.Debug("Failed to get existing design items: %v", err)
	}

	// Build map of existing items by node_link_id
	existingMap := make(map[string]graphql.DesignItem)
	for _, item := range existingItems {
		existingMap[item.NodeLinkID] = item
	}

	// Validate specs and determine status
	var validSpecs []upload.ValidatedSpec
	var invalidSpecs []upload.ValidatedSpec

	for _, spec := range specs {
		existingItem, exists := existingMap[spec.NodeLinkID]

		// Check if existing item is deleted
		if exists && existingItem.Status == upload.DesignItemStatusDeleted {
			invalidSpecs = append(invalidSpecs, upload.ValidatedSpec{
				Spec:    spec,
				IsValid: false,
				Errors:  []string{"The item has been deleted in Figma. Please review or remove the corresponding row."},
			})
			continue
		}

		// Determine status and validate
		status, validationErrors := upload.DetermineSpecStatus(&spec, "")

		// Check for changes (skip unchanged items)
		currentSpecMap := upload.MapSpecForComparison(&spec)
		var previousSpecMap map[string]interface{}
		if exists {
			// Convert existing item's specs to Spec for comparison
			existingSpec := convertDesignItemToSpec(existingItem)
			previousSpecMap = upload.MapSpecForComparison(&existingSpec)
		}

		hasChanged := !upload.CompareSpecs(currentSpecMap, previousSpecMap)

		// Skip unchanged items with same status
		if !hasChanged && exists && existingItem.Status == status {
			logger.Debug("Skipping unchanged spec: %s", spec.NodeLinkID)
			continue
		}

		if len(validationErrors) > 0 {
			invalidSpecs = append(invalidSpecs, upload.ValidatedSpec{
				Spec:    spec,
				Status:  status,
				IsValid: false,
				Errors:  validationErrors,
				Changed: hasChanged,
				IsNew:   !exists,
			})
		} else {
			validSpecs = append(validSpecs, upload.ValidatedSpec{
				Spec:    spec,
				Status:  status,
				IsValid: true,
				Changed: hasChanged,
				IsNew:   !exists,
			})
		}
	}

	// Validate linked frames (matches SDK's validateLinkedFrames)
	var linkedFrameNodeLinkIds []struct {
		nodeID        string
		linkedFrameID string
	}
	for i := range validSpecs {
		if validSpecs[i].LinkedFrameID != "" {
			linkedFrameNodeLinkIds = append(linkedFrameNodeLinkIds, struct {
				nodeID        string
				linkedFrameID string
			}{
				nodeID:        validSpecs[i].NodeLinkID,
				linkedFrameID: validSpecs[i].LinkedFrameID,
			})
		}
	}

	if len(linkedFrameNodeLinkIds) > 0 {
		// Collect unique linked frame IDs
		uniqueFrameIDs := make(map[string]bool)
		for _, lf := range linkedFrameNodeLinkIds {
			uniqueFrameIDs[lf.linkedFrameID] = true
		}
		var frameLinkIds []string
		for id := range uniqueFrameIDs {
			frameLinkIds = append(frameLinkIds, id)
		}

		// Query to validate linked frames exist
		linkedFrames, err := client.ListFramesByFrameLinkIds(ctx, parsed.FileKey, frameLinkIds)
		if err != nil {
			logger.Debug("Failed to validate linked frames: %v", err)
		} else {
			// Build map of existing frames
			frameMap := make(map[string]bool)
			for _, f := range linkedFrames {
				frameMap[f.FrameLinkID] = true
			}

			// Mark specs with invalid linked frames as invalid
			for i := range validSpecs {
				if validSpecs[i].LinkedFrameID != "" && validSpecs[i].IsValid {
					if !frameMap[validSpecs[i].LinkedFrameID] {
						validSpecs[i].IsValid = false
						validSpecs[i].Errors = append(validSpecs[i].Errors,
							fmt.Sprintf("Linked frame with ID \"%s\" not found", validSpecs[i].LinkedFrameID))
						// Move to invalid specs
						invalidSpecs = append(invalidSpecs, validSpecs[i])
					}
				}
			}

			// Filter out invalid specs from validSpecs
			var filteredValidSpecs []upload.ValidatedSpec
			for _, vs := range validSpecs {
				if vs.IsValid {
					filteredValidSpecs = append(filteredValidSpecs, vs)
				}
			}
			validSpecs = filteredValidSpecs
		}
	}

	// Log validation errors
	if len(invalidSpecs) > 0 {
		logger.Debug("Found %d invalid specs", len(invalidSpecs))
		for _, inv := range invalidSpecs {
			logger.Debug("  - %s: %v", inv.NodeLinkID, inv.Errors)
		}
	}

	if len(validSpecs) == 0 {
		if len(invalidSpecs) > 0 {
			return upload.UploadResult{
				FilePath: filePath,
				FileName: fileName,
				Status:   upload.StatusFailed,
				Message:  fmt.Sprintf("No valid specs to update (%d invalid)", len(invalidSpecs)),
			}
		}
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusSkipped,
			Message:  "No changes detected",
		}
	}

	// Prepare items for upsert
	var items []map[string]interface{}
	for _, validated := range validSpecs {
		spec := validated.Spec

		// Determine section_link_id: use existing or fallback to frame's link ID
		sectionLinkID := spec.SectionLinkID
		if sectionLinkID == "" {
			if existing, ok := existingMap[spec.NodeLinkID]; ok && existing.SectionLinkID != "" {
				sectionLinkID = existing.SectionLinkID
			}
		}
		if sectionLinkID == "" {
			sectionLinkID = frame.FrameLinkID
		}

		payload := upload.TransformSpecToPayload(spec, frame.ID, frame.FileID, sectionLinkID, validated.Status)

		// Convert to map for GraphQL
		item := map[string]interface{}{
			"no":              payload.No,
			"name":            payload.Name,
			"type":            payload.Type,
			"node_link_id":    payload.NodeLinkID,
			"section_link_id": payload.SectionLinkID,
			"frame_id":        payload.FrameID,
			"file_id":         payload.FileID,
			"status":          payload.Status,
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
			// Prepare revision entries for new AND changed items
			var revs []map[string]interface{}
			for _, item := range savedItems {
				existingItem, existed := existingMap[item.NodeLinkID]

				shouldCreateRevision := false
				if !existed {
					// New item - always create revision
					shouldCreateRevision = true
				} else {
					// Existing item - check if specs changed
					existingSpec := convertDesignItemToSpec(existingItem)
					currentSpecMap := upload.MapSpecForComparison(&existingSpec)

					// Find the validated spec to get current values
					for _, vs := range validSpecs {
						if vs.NodeLinkID == item.NodeLinkID {
							newSpecMap := upload.MapSpecForComparison(&vs.Spec)
							if !upload.CompareSpecs(newSpecMap, currentSpecMap) {
								shouldCreateRevision = true
							}
							break
						}
					}
				}

				if shouldCreateRevision {
					rev := map[string]interface{}{
						"design_item_id": item.ID,
						"status":         item.Status,
						"specs":          item.Specs,
						"type":           item.Type,
						"change_type":    "user",
						"name":           "",
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

	message := fmt.Sprintf("Uploaded %d specs", len(savedItems))
	if len(invalidSpecs) > 0 {
		message += fmt.Sprintf(" (%d invalid)", len(invalidSpecs))
	}

	return upload.UploadResult{
		FilePath: filePath,
		FileName: fileName,
		Status:   upload.StatusSuccess,
		Message:  message,
	}
}

// convertDesignItemToSpec converts a GraphQL DesignItem to a Spec for comparison
func convertDesignItemToSpec(item graphql.DesignItem) upload.Spec {
	spec := upload.Spec{
		No:            item.No,
		NodeLinkID:    item.NodeLinkID,
		SectionLinkID: item.SectionLinkID,
		Type:          item.Type,
	}

	// Parse specs JSON if available
	if len(item.Specs) > 0 {
		var specDetails struct {
			Item *struct {
				Name       string `json:"name"`
				NameTrans  string `json:"nameTrans"`
				ButtonType string `json:"buttonType"`
				OtherType  string `json:"otherType"`
			} `json:"item"`
			Navigation *struct {
				Action        string `json:"action"`
				LinkedFrameID string `json:"linkedFrameId"`
				Note          string `json:"note"`
			} `json:"navigation"`
			Validation *struct {
				DataType     string `json:"dataType"`
				Required     *bool  `json:"required"`
				Format       string `json:"format"`
				MinLength    *int   `json:"minLength"`
				MaxLength    *int   `json:"maxLength"`
				DefaultValue string `json:"defaultValue"`
				Note         string `json:"note"`
			} `json:"validation"`
			Database *struct {
				TableName  string `json:"tableName"`
				ColumnName string `json:"columnName"`
				Note       string `json:"note"`
			} `json:"database"`
			Description string `json:"description"`
		}

		if err := json.Unmarshal(item.Specs, &specDetails); err == nil {
			if specDetails.Item != nil {
				spec.Name = specDetails.Item.Name
				spec.NameTrans = specDetails.Item.NameTrans
				spec.ButtonType = specDetails.Item.ButtonType
				spec.OtherType = specDetails.Item.OtherType
			}
			if specDetails.Navigation != nil {
				spec.Action = specDetails.Navigation.Action
				spec.LinkedFrameID = specDetails.Navigation.LinkedFrameID
				spec.NavigationNote = specDetails.Navigation.Note
			}
			if specDetails.Validation != nil {
				spec.DataType = specDetails.Validation.DataType
				spec.Required = specDetails.Validation.Required
				spec.Format = specDetails.Validation.Format
				spec.MinLength = specDetails.Validation.MinLength
				spec.MaxLength = specDetails.Validation.MaxLength
				spec.DefaultValue = specDetails.Validation.DefaultValue
				spec.ValidationNote = specDetails.Validation.Note
			}
			if specDetails.Database != nil {
				spec.TableName = specDetails.Database.TableName
				spec.ColumnName = specDetails.Database.ColumnName
				spec.DatabaseNote = specDetails.Database.Note
			}
			spec.Description = specDetails.Description
		}
	}

	return spec
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

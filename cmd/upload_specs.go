package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
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
	specFrameID         string
	specFrameName       string
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

CSV files can be placed anywhere. The MoMorph frame ID must be supplied via
--frame-id or encoded in the filename as:
  {momorph_frame_id}-{frame_name}.csv  (e.g. 42-TopChannel.csv)

No Figma file key or Figma frame ID is required.
`,
	Example: `  # Upload using filename convention (frame ID embedded in name)
  momorph upload specs 42-TopChannel.csv

  # Upload from any location with explicit frame ID
  momorph upload specs ~/data/my-specs.csv --frame-id=42

  # Upload multiple files sharing the same frame
  momorph upload specs spec1.csv spec2.csv --frame-id=42

  # Upload all CSV files in a directory recursively
  momorph upload specs --dir ./specs/ -r

  # Dry run (show what would be uploaded)
  momorph upload specs --dry-run ./specs/*.csv`,
	RunE: runUploadSpecs,
}

func init() {
	uploadSpecsCmd.Flags().StringVarP(&specUploadDir, "dir", "d", "", "Directory to search for CSV files")
	uploadSpecsCmd.Flags().BoolVarP(&specUploadRecursive, "recursive", "r", false, "Search directories recursively")
	uploadSpecsCmd.Flags().BoolVar(&specUploadDryRun, "dry-run", false, "Show what would be uploaded without actually uploading")
	uploadSpecsCmd.Flags().BoolVar(&specUploadContinue, "continue-on-error", false, "Continue uploading remaining files if one fails")
	uploadSpecsCmd.Flags().StringVar(&specFrameID, "frame-id", "", "MoMorph frame ID (integer). Required when frame ID is not encoded in the filename.")
	uploadSpecsCmd.Flags().StringVar(&specFrameName, "frame-name", "", "Frame name for display (optional, used with --frame-id)")
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

	// Parse --frame-id flag when provided
	var flagsMeta *upload.MoMorphFrameMeta
	if specFrameID != "" {
		parsedID, err := strconv.Atoi(specFrameID)
		if err != nil || parsedID <= 0 {
			return fmt.Errorf("--frame-id must be a positive MoMorph integer frame ID, got: %s", specFrameID)
		}
		flagsMeta = &upload.MoMorphFrameMeta{
			FrameID:   parsedID,
			FrameName: specFrameName,
		}
	}

	// Resolve files - path convention is no longer required
	files, err := upload.ResolveFiles(args, specUploadDir, specUploadRecursive, "specs", true)
	if err != nil {
		return fmt.Errorf("failed to resolve files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No CSV files found to upload")
		fmt.Println("\nProvide CSV file paths directly or use --dir to scan a directory.")
		fmt.Println("The frame ID must be in the filename (e.g. 42-MyScreen.csv) or supplied via --frame-id.")
		return nil
	}

	// Validate files
	validFiles, skipped := upload.ValidateFiles(files, "specs", true)

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
			var meta upload.MoMorphFrameMeta
			if flagsMeta != nil {
				meta = *flagsMeta
			} else {
				id, name, err := upload.ParseFileNameForFrameID(f)
				if err == nil {
					meta = upload.MoMorphFrameMeta{FrameID: id, FrameName: name}
				}
			}
			specs, _ := upload.ParseSpecsCSV(f)
			fmt.Printf("  - %s\n", filepath.Base(f))
			fmt.Printf("    Frame ID:   %d\n", meta.FrameID)
			fmt.Printf("    Frame Name: %s\n", meta.FrameName)
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
	results := uploadSpecFiles(ctx, client, validFiles, flagsMeta, actor, specUploadContinue)

	// Combine with skipped files
	allResults := append(skipped, results...)

	// Display summary
	displayUploadSummary(allResults)

	return nil
}

func uploadSpecFiles(ctx context.Context, client *graphql.Client, files []string, flagsMeta *upload.MoMorphFrameMeta, actor string, continueOnError bool) []upload.UploadResult {
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

		result := uploadSingleSpecFile(ctx, client, file, flagsMeta, actor)
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

// uploadSingleSpecFile uploads a single spec CSV file.
// When flagsMeta is non-nil its frame ID overrides any filename-derived value.
func uploadSingleSpecFile(ctx context.Context, client *graphql.Client, filePath string, flagsMeta *upload.MoMorphFrameMeta, actor string) upload.UploadResult {
	fileName := filepath.Base(filePath)

	// Resolve frame metadata: flags take precedence, then filename
	var meta upload.MoMorphFrameMeta
	if flagsMeta != nil {
		meta = *flagsMeta
	} else {
		id, name, err := upload.ParseFileNameForFrameID(filePath)
		if err != nil {
			return upload.UploadResult{
				FilePath: filePath,
				FileName: fileName,
				Status:   upload.StatusSkipped,
				Error:    err,
				Message:  "Cannot determine frame ID: use --frame-id flag or name the file {frame_id}-{name}.csv",
			}
		}
		meta = upload.MoMorphFrameMeta{FrameID: id, FrameName: name}
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

	// Fetch frame by MoMorph integer ID
	frame, err := client.GetFrameByID(ctx, meta.FrameID)
	if err != nil {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusFailed,
			Error:    err,
			Message:  fmt.Sprintf("Frame not found (id=%d): %v", meta.FrameID, err),
		}
	}

	// Allow uploading specs for frames in any status, including 'design'.
	// This supports drafting specs before design assets are linked.

	// Collect node link IDs present in the CSV (may be empty for no-Figma specs)
	var nodeLinkIds []string
	for _, spec := range specs {
		if spec.NodeLinkID != "" {
			nodeLinkIds = append(nodeLinkIds, spec.NodeLinkID)
		}
	}

	// Get existing design items for change-detection (only when node link IDs are available)
	var existingItems []graphql.DesignItem
	if len(nodeLinkIds) > 0 {
		existingItems, err = client.ListDesignItemsByFrameID(ctx, frame.ID, nodeLinkIds)
		if err != nil {
			logger.Debug("Failed to get existing design items: %v", err)
		}
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

	if len(linkedFrameNodeLinkIds) > 0 && frame.FrameLinkID != "" {
		// Collect unique linked frame IDs
		uniqueFrameIDs := make(map[string]bool)
		for _, lf := range linkedFrameNodeLinkIds {
			uniqueFrameIDs[lf.linkedFrameID] = true
		}
		var frameLinkIds []string
		for id := range uniqueFrameIDs {
			frameLinkIds = append(frameLinkIds, id)
		}

		// Query to validate linked frames exist (best-effort; skip on error)
		linkedFrames, err := client.ListFramesByFrameLinkIds(ctx, "", frameLinkIds)
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

package cmd

import (
	"context"
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
	tcUploadDir       string
	tcUploadRecursive bool
	tcUploadDryRun    bool
	tcUploadContinue  bool
	tcFileKey         string
	tcFrameID         string
	tcFrameName       string
	tcScreenID        string
)

// CSV columns are mapped to test case fields:
//
//	TC_ID -> ID, Steps -> step, Category -> category, Page_Name -> page_name,
//	Section -> test_area, Test_Data -> test_data, Sub_Category -> sub_category,
//	Sub_Sub_Category -> sub_sub_category, Precondition -> pre_condition,
//	Expected_Result -> expected_result, Testcase_Type -> tc_type,
//	Priority -> priority, Test_Results -> test_results, Executed_Date -> executed_date,
//	Tester -> tester, Note -> note
var uploadTestcasesCmd = &cobra.Command{
	Use:   "testcases [files...]",
	Short: "Upload test cases to MoMorph server",
	Long: `Upload test case CSV files to MoMorph server.

By default, files must follow the path pattern:
  .momorph/testcases/{file_key}/{frame_id}-{frame_name}.csv

Alternatively, use --file-key (and optionally --frame-id, --frame-name) to
upload CSV files from any location without following the path convention.

You can also use --screen-id to upload by screen ID instead of frame ID.
`,
	Example: `  # Upload using path convention
  momorph upload testcases .momorph/testcases/xxx/9276:19907-TOP_Channel.csv

  # Upload from any location with explicit metadata
  momorph upload testcases ~/data/tc.csv --file-key=xxx --frame-id=9276:19907

  # Upload using screen ID
  momorph upload testcases ~/data/tc.csv --screen-id=42

  # Upload all testcases in a directory recursively
  momorph upload testcases --dir .momorph/testcases/ -r

  # Dry run (show what would be uploaded)
  momorph upload testcases --dry-run .momorph/testcases/**/*.csv`,
	RunE: runUploadTestcases,
}

func init() {
	uploadTestcasesCmd.Flags().StringVarP(&tcUploadDir, "dir", "d", "", "Directory to search for CSV files")
	uploadTestcasesCmd.Flags().BoolVarP(&tcUploadRecursive, "recursive", "r", false, "Search directories recursively")
	uploadTestcasesCmd.Flags().BoolVar(&tcUploadDryRun, "dry-run", false, "Show what would be uploaded without actually uploading")
	uploadTestcasesCmd.Flags().BoolVar(&tcUploadContinue, "continue-on-error", false, "Continue uploading remaining files if one fails")
	uploadTestcasesCmd.Flags().StringVar(&tcFileKey, "file-key", "", "Figma file key (required when CSV is not in .momorph/ path)")
	uploadTestcasesCmd.Flags().StringVar(&tcFrameID, "frame-id", "", "Figma frame ID (optional, used with --file-key)")
	uploadTestcasesCmd.Flags().StringVar(&tcFrameName, "frame-name", "", "Frame name (optional, used with --file-key)")
	uploadTestcasesCmd.Flags().StringVar(&tcScreenID, "screen-id", "", "Screen ID (MoMorph integer, alternative to --frame-id)")
	uploadCmd.AddCommand(uploadTestcasesCmd)
}

func runUploadTestcases(cmd *cobra.Command, args []string) error {
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

	// Validate conflicting flags
	if tcScreenID != "" && (tcFileKey != "" || tcFrameID != "") {
		return fmt.Errorf("--screen-id cannot be used together with --file-key or --frame-id")
	}

	// Determine if using flags mode (--file-key or --screen-id provided)
	useFlags := tcFileKey != "" || tcScreenID != ""

	// Build parsed metadata from flags when in flags mode
	var flagsParsed *upload.ParsedFilePath
	if useFlags {
		flagsParsed = &upload.ParsedFilePath{
			Type:      "testcases",
			FileKey:   tcFileKey,
			FrameID:   tcFrameID,
			FrameName: tcFrameName,
		}
	}

	// Resolve files
	files, err := upload.ResolveFiles(args, tcUploadDir, tcUploadRecursive, "testcases", useFlags)
	if err != nil {
		return fmt.Errorf("failed to resolve files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No CSV files found to upload")
		if !useFlags {
			fmt.Println("\nMake sure files are in the correct path format:")
			fmt.Println("  .momorph/testcases/{file_key}/{frame_id}-{frame_name}.csv")
			fmt.Println("\nOr use --file-key or --screen-id to upload from any location:")
			fmt.Println("  momorph upload testcases myfile.csv --file-key=<figma_file_key>")
			fmt.Println("  momorph upload testcases myfile.csv --screen-id=<screen_id>")
		}
		return nil
	}

	// Validate files
	validFiles, skipped := upload.ValidateFiles(files, "testcases", useFlags)

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
	if tcUploadDryRun {
		fmt.Printf("\n[DRY RUN] Would upload %d file(s):\n", len(validFiles))
		for _, f := range validFiles {
			var parsed *upload.ParsedFilePath
			if useFlags {
				parsed = flagsParsed
			} else {
				parsed, _ = upload.ParseFilePath(f)
			}
			fmt.Printf("  - %s\n", filepath.Base(f))
			if tcScreenID != "" {
				fmt.Printf("    Screen ID: %s\n", tcScreenID)
			} else {
				fmt.Printf("    File Key: %s\n", parsed.FileKey)
				fmt.Printf("    Frame ID: %s\n", parsed.FrameID)
			}
			fmt.Printf("    Frame Name: %s\n", parsed.FrameName)
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
	fmt.Printf("\nUploading %d test case file(s)...\n", len(validFiles))
	results := uploadTestcaseFiles(ctx, client, validFiles, flagsParsed, tcScreenID, tcUploadContinue)

	// Combine with skipped files
	allResults := append(skipped, results...)

	// Display summary
	displayUploadSummary(allResults)

	return nil
}

func uploadTestcaseFiles(ctx context.Context, client *graphql.Client, files []string, flagsParsed *upload.ParsedFilePath, screenID string, continueOnError bool) []upload.UploadResult {
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

		result := uploadSingleTestcaseFile(ctx, client, file, flagsParsed, screenID)
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

// uploadSingleTestcaseFile uploads a single testcase CSV file. When flagsParsed
// is non-nil it is used as metadata instead of parsing from the file path.
// When screenID != "", the frame is resolved by screen_id instead of file-key + frame-id.
func uploadSingleTestcaseFile(ctx context.Context, client *graphql.Client, filePath string, flagsParsed *upload.ParsedFilePath, screenID string) upload.UploadResult {
	fileName := filepath.Base(filePath)

	// Screen ID mode: resolve frame by screen_id directly
	if screenID != "" {
		// Parse CSV file
		frameName := ""
		if flagsParsed != nil {
			frameName = flagsParsed.FrameName
		}
		content, err := upload.ParseTestcasesCSV(filePath, frameName)
		if err != nil {
			return upload.UploadResult{
				FilePath: filePath,
				FileName: fileName,
				Status:   upload.StatusFailed,
				Error:    err,
				Message:  fmt.Sprintf("Failed to parse CSV: %v", err),
			}
		}

		if len(content.TestCases) == 0 {
			return upload.UploadResult{
				FilePath: filePath,
				FileName: fileName,
				Status:   upload.StatusSkipped,
				Message:  "CSV file contains no test cases",
			}
		}

		logger.Debug("Parsed %d test cases from %s", len(content.TestCases), fileName)

		// Check if test cases already exist for this screen
		existingTestCases, err := client.GetFrameTestCasesByScreenID(ctx, screenID)
		if err != nil {
			logger.Debug("No existing test cases found: %v", err)
		}

		if len(existingTestCases) > 0 {
			logger.Debug("Updating existing test case ID: %d", existingTestCases[0].ID)
			_, err = client.UpdateFrameTestcase(ctx, existingTestCases[0].ID, content)
			if err != nil {
				return upload.UploadResult{
					FilePath: filePath,
					FileName: fileName,
					Status:   upload.StatusFailed,
					Error:    err,
					Message:  fmt.Sprintf("Failed to update test case: %v", err),
				}
			}
		} else {
			// Get frame by screen_id to get internal ID
			frame, err := client.GetFrameByScreenID(ctx, screenID)
			if err != nil {
				return upload.UploadResult{
					FilePath: filePath,
					FileName: fileName,
					Status:   upload.StatusFailed,
					Error:    err,
					Message:  fmt.Sprintf("Frame not found for screen_id=%s: %v", screenID, err),
				}
			}

			logger.Debug("Creating new test case for frame ID: %d (screen_id=%s)", frame.ID, screenID)

			_, err = client.InsertFrameTestcase(ctx, frame.ID, content)
			if err != nil {
				return upload.UploadResult{
					FilePath: filePath,
					FileName: fileName,
					Status:   upload.StatusFailed,
					Error:    err,
					Message:  fmt.Sprintf("Failed to insert test case: %v", err),
				}
			}
		}

		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusSuccess,
			Message:  fmt.Sprintf("Uploaded %d test cases", len(content.TestCases)),
		}
	}

	// Resolve metadata: use flags or parse from path
	var parsed *upload.ParsedFilePath
	if flagsParsed != nil {
		parsed = flagsParsed
	} else {
		var err error
		parsed, err = upload.ParseFilePath(filePath)
		if err != nil {
			return upload.UploadResult{
				FilePath: filePath,
				FileName: fileName,
				Status:   upload.StatusSkipped,
				Error:    err,
				Message:  "Invalid file path format",
			}
		}
	}

	// Parse CSV file — pass FrameName as screen name (empty when using flags without --frame-name)
	content, err := upload.ParseTestcasesCSV(filePath, parsed.FrameName)
	if err != nil {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusFailed,
			Error:    err,
			Message:  fmt.Sprintf("Failed to parse CSV: %v", err),
		}
	}

	if len(content.TestCases) == 0 {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusSkipped,
			Message:  "CSV file contains no test cases",
		}
	}

	logger.Debug("Parsed %d test cases from %s", len(content.TestCases), fileName)

	// When no frame ID is provided, we cannot proceed (backend requires frame context)
	if parsed.FrameID == "" {
		return upload.UploadResult{
			FilePath: filePath,
			FileName: fileName,
			Status:   upload.StatusFailed,
			Message:  "frame-id is required for uploading test cases (use --frame-id or --screen-id flag)",
		}
	}

	// Check if test cases already exist for this frame
	existingTestCases, err := client.GetFrameTestCases(ctx, parsed.FileKey, parsed.FrameID)
	if err != nil {
		logger.Debug("No existing test cases found: %v", err)
	}

	if len(existingTestCases) > 0 {
		// Update existing test case
		logger.Debug("Updating existing test case ID: %d", existingTestCases[0].ID)
		_, err = client.UpdateFrameTestcase(ctx, existingTestCases[0].ID, content)
		if err != nil {
			return upload.UploadResult{
				FilePath: filePath,
				FileName: fileName,
				Status:   upload.StatusFailed,
				Error:    err,
				Message:  fmt.Sprintf("Failed to update test case: %v", err),
			}
		}
	} else {
		// Get frame to get internal ID
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

		logger.Debug("Creating new test case for frame ID: %d", frame.ID)

		// Insert new test case
		_, err = client.InsertFrameTestcase(ctx, frame.ID, content)
		if err != nil {
			return upload.UploadResult{
				FilePath: filePath,
				FileName: fileName,
				Status:   upload.StatusFailed,
				Error:    err,
				Message:  fmt.Sprintf("Failed to insert test case: %v", err),
			}
		}
	}

	return upload.UploadResult{
		FilePath: filePath,
		FileName: fileName,
		Status:   upload.StatusSuccess,
		Message:  fmt.Sprintf("Uploaded %d test cases", len(content.TestCases)),
	}
}

func displayUploadSummary(results []upload.UploadResult) {
	summary := upload.NewUploadSummary(results)

	fmt.Println()
	fmt.Println("─────────────────────────────────────────")
	fmt.Println("Summary")
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("  Total files:  %d\n", summary.Total)
	fmt.Printf("  Success:      %d\n", summary.Success)
	fmt.Printf("  Failed:       %d\n", summary.Failed)
	fmt.Printf("  Skipped:      %d\n", summary.Skipped)
	fmt.Println("─────────────────────────────────────────")

	// Show status message
	if summary.Failed == 0 && summary.Skipped == 0 {
		fmt.Printf("\n✓ Successfully uploaded %d file(s)\n", summary.Success)
	} else if summary.Success == 0 {
		fmt.Println("\n✗ All uploads failed or were skipped")
	} else {
		fmt.Printf("\n⚠ Uploaded %d file(s), %d failed, %d skipped\n",
			summary.Success, summary.Failed, summary.Skipped)
	}
}

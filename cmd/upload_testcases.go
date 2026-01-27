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

Files must follow the path pattern:
  .momorph/testcases/{file_key}/{frame_id}-{frame_name}.csv
`,
	Example: `  # Upload a single file
  momorph upload testcases .momorph/testcases/xxx/yyy.csv

  # Upload multiple files
  momorph upload testcases file1.csv file2.csv

  # Upload all testcases in a directory recursively
  momorph upload testcases --dir .momorph/testcases/ -r

  # Upload using glob pattern
  momorph upload testcases ".momorph/testcases/**/*.csv"

  # Dry run (show what would be uploaded)
  momorph upload testcases --dry-run .momorph/testcases/**/*.csv`,
	RunE: runUploadTestcases,
}

func init() {
	uploadTestcasesCmd.Flags().StringVarP(&tcUploadDir, "dir", "d", "", "Directory to search for CSV files")
	uploadTestcasesCmd.Flags().BoolVarP(&tcUploadRecursive, "recursive", "r", false, "Search directories recursively")
	uploadTestcasesCmd.Flags().BoolVar(&tcUploadDryRun, "dry-run", false, "Show what would be uploaded without actually uploading")
	uploadTestcasesCmd.Flags().BoolVar(&tcUploadContinue, "continue-on-error", false, "Continue uploading remaining files if one fails")
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

	// Resolve files
	files, err := upload.ResolveFiles(args, tcUploadDir, tcUploadRecursive, "testcases")
	if err != nil {
		return fmt.Errorf("failed to resolve files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No CSV files found to upload")
		fmt.Println("\nMake sure files are in the correct path format:")
		fmt.Println("  .momorph/testcases/{file_key}/{frame_id}-{frame_name}.csv")
		return nil
	}

	// Validate files
	validFiles, skipped := upload.ValidateFiles(files, "testcases")

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
			parsed, _ := upload.ParseFilePath(f)
			fmt.Printf("  - %s\n", filepath.Base(f))
			fmt.Printf("    File Key: %s\n", parsed.FileKey)
			fmt.Printf("    Frame ID: %s\n", parsed.FrameID)
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
	results := uploadTestcaseFiles(ctx, client, validFiles, tcUploadContinue)

	// Combine with skipped files
	allResults := append(skipped, results...)

	// Display summary
	displayUploadSummary(allResults)

	return nil
}

func uploadTestcaseFiles(ctx context.Context, client *graphql.Client, files []string, continueOnError bool) []upload.UploadResult {
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

		result := uploadSingleTestcaseFile(ctx, client, file)
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

func uploadSingleTestcaseFile(ctx context.Context, client *graphql.Client, filePath string) upload.UploadResult {
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
	content, err := upload.ParseTestcasesCSV(filePath)
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

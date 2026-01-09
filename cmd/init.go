package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/momorph/cli/internal/api"
	"github.com/momorph/cli/internal/auth"
	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/template"
	"github.com/momorph/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	aiTool string
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new MoMorph project from the latest template",
	Example: `  momorph init my-project --ai=copilot
  momorph init . --ai=cursor
  momorph init my-project`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&aiTool, "ai", "", "AI tool to use (copilot, cursor, claude)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	projectName := args[0]

	// Check authentication
	if !auth.IsAuthenticated() {
		fmt.Println("✗ Not authenticated")
		fmt.Println("\nRun 'momorph login' to authenticate before initializing projects")
		return nil
	}

	// Determine target directory
	var targetDir string
	if projectName == "." {
		var err error
		targetDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		absPath, err := filepath.Abs(projectName)
		if err != nil {
			return fmt.Errorf("failed to resolve path: %w", err)
		}
		targetDir = absPath
	}

	// Check if directory exists and is not empty
	if err := checkDirectory(targetDir); err != nil {
		return err
	}

	// Prompt for AI tool if not provided
	if aiTool == "" {
		selectedTool, err := ui.PromptAITool()
		if err != nil {
			return fmt.Errorf("failed to get AI tool selection: %w", err)
		}
		aiTool = selectedTool
	}

	// Validate AI tool
	validTools := map[string]bool{
		"copilot": true,
		"cursor":  true,
		"claude":  true,
	}
	if !validTools[aiTool] {
		return fmt.Errorf("invalid AI tool: %s (must be one of: copilot, cursor, claude)", aiTool)
	}

	fmt.Printf("\n🚀 Initializing MoMorph project with %s...\n\n", aiTool)

	// Create API client
	client, err := api.NewClient()
	if err != nil {
		logger.Error("Failed to create API client", err)
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Get template metadata
	fmt.Println("📋 Fetching template metadata...")
	templateMeta, err := client.GetProjectTemplate(ctx, aiTool)
	if err != nil {
		logger.Error("Failed to get template", err)
		return fmt.Errorf("failed to get template: %w", err)
	}

	logger.Info("Template metadata received:")
	logger.Info("  Key: %s", templateMeta.Key)
	logger.Info("  DownloadURL: %s", templateMeta.DownloadURL)
	logger.Info("  ExpiresIn: %d", templateMeta.ExpiresIn)
	logger.Info("  Cached: %v", templateMeta.Cached)

	// Download template
	fmt.Println("📥 Downloading template...")
	// Note: API doesn't provide size, so progress bar will show bytes downloaded
	var progressBar *ui.ProgressBar

	zipPath, err := template.Download(templateMeta.DownloadURL, "", func(downloaded, total int64) {
		if progressBar == nil && total > 0 {
			progressBar = ui.NewProgressBar(total)
		}
		if progressBar != nil {
			progressBar.Update(downloaded)
		}
	})
	if err != nil {
		logger.Error("Failed to download template", err)
		return fmt.Errorf("failed to download template: %w", err)
	}
	if progressBar != nil {
		progressBar.Finish()
	}

	// Extract template
	fmt.Println("📦 Extracting template...")
	if err := template.Extract(zipPath, targetDir); err != nil {
		logger.Error("Failed to extract template", err)
		// Clean up on error
		template.CleanupPartial(targetDir)
		return fmt.Errorf("failed to extract template: %w", err)
	}

	// Clean up downloaded ZIP
	os.Remove(zipPath)

	// Update AI tool config with GitHub token if needed
	fmt.Printf("🔧 Configuring %s with GitHub token...\n", aiTool)
	token, err := auth.LoadToken()
	if err != nil {
		logger.Warn("Failed to load GitHub token: %v", err)
		fmt.Println("⚠️  Warning: Could not update GitHub token in AI tool config")
	} else if token.GitHubToken != "" {
		if err := template.UpdateAIToolConfig(aiTool, targetDir, token.GitHubToken); err != nil {
			logger.Warn("Failed to update AI tool config: %v", err)
			fmt.Println("⚠️  Warning: Could not update GitHub token in AI tool config")
		} else {
			logger.Info("Successfully updated GitHub token in %s config", aiTool)
		}
	}

	// Success message
	fmt.Println("\n✓ Project initialized successfully!")
	fmt.Printf("\n📁 Project directory: %s\n", targetDir)
	fmt.Printf("🤖 AI tool: %s\n", aiTool)

	fmt.Println("\n📚 Next steps:")
	if projectName != "." {
		fmt.Printf("  1. cd %s\n", projectName)
		fmt.Println("  2. Follow the README.md for setup instructions")
		fmt.Println("  3. Start building with your AI assistant!")
	} else {
		fmt.Println("  1. Follow the README.md for setup instructions")
		fmt.Println("  2. Start building with your AI assistant!")
	}

	logger.Info("Project initialized at %s with %s", targetDir, aiTool)

	return nil
}

// checkDirectory checks if the directory exists and handles confirmation
func checkDirectory(dirPath string) error {
	// Check if directory exists
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		// Directory doesn't exist, will be created during extraction
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check directory: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory: %s", dirPath)
	}

	// Check if directory is empty
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// If directory is not empty, ask for confirmation
	if len(entries) > 0 {
		confirm, err := ui.ConfirmOverwrite(dirPath)
		if err != nil {
			return fmt.Errorf("failed to get confirmation: %w", err)
		}
		if !confirm {
			return fmt.Errorf("initialization cancelled")
		}
	}

	return nil
}

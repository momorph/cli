package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/momorph/cli/internal/api"
	"github.com/momorph/cli/internal/auth"
	"github.com/momorph/cli/internal/beads"
	"github.com/momorph/cli/internal/config"
	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/template"
	"github.com/momorph/cli/internal/ui"
	"github.com/momorph/cli/internal/vscode"
	"github.com/spf13/cobra"
)

var (
	aiTool       string
	templateTag  string
	installBeads bool
	// ErrUserCancelled is returned when the user cancels an operation
	ErrUserCancelled = errors.New("user cancelled")
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new MoMorph project from the latest template",
	Example: `  momorph init my-project --ai=copilot
  momorph init . --ai=cursor
  momorph init my-project --with-beads
  momorph init my-project --ai=claude --tag=stable --with-beads`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&aiTool, "ai", "", "AI tool to use (copilot, cursor, claude, windsurf, gemini)")
	initCmd.Flags().StringVar(&templateTag, "tag", "", "Template version tag (stable, latest, or specific version)")
	initCmd.Flags().BoolVar(&installBeads, "with-beads", false, "Install uv and beads-mcp for task management")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	projectName := args[0]

	// Setup signal handling for graceful cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n✗ Initialization cancelled")
		cancel()
		os.Exit(0)
	}()

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
		if errors.Is(err, ErrUserCancelled) {
			fmt.Println("Initialization cancelled")
			return nil
		}
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
		"copilot":  true,
		"cursor":   true,
		"claude":   true,
		"windsurf": true,
		"gemini":   true,
	}
	if !validTools[aiTool] {
		return fmt.Errorf("invalid AI tool: %s (must be one of: copilot, cursor, claude, windsurf, gemini)", aiTool)
	}

	fmt.Printf("🚀 Initializing MoMorph project with %s\n", aiTool)

	// Create API client
	client, err := api.NewClient()
	if err != nil {
		logger.Error("Failed to create API client", err)
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Get template metadata
	fmt.Println("📋 Fetching template...")
	templateMeta, err := client.GetProjectTemplate(ctx, aiTool, templateTag)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return nil // User cancelled
		}
		logger.Error("Failed to get template", err)
		return fmt.Errorf("failed to get template: %w", err)
	}

	logger.Info("Template metadata received:")
	logger.Info("  Key: %s", templateMeta.Key)
	logger.Info("  DownloadURL: %s", templateMeta.DownloadURL)
	logger.Info("  ExpiresIn: %d", templateMeta.ExpiresIn)
	logger.Info("  Cached: %v", templateMeta.Cached)

	// Download template
	fmt.Print("📥 Downloading...")
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
		if ctx.Err() == context.Canceled {
			return nil // User cancelled
		}
		logger.Error("Failed to download template", err)
		return fmt.Errorf("failed to download template: %w", err)
	}
	if progressBar != nil {
		progressBar.Finish()
		fmt.Println()
	}

	// Extract template (with config file merging)
	fmt.Println("📦 Extracting...")
	if err := template.ExtractWithMerge(zipPath, targetDir); err != nil {
		logger.Error("Failed to extract template", err)
		// Clean up on error
		template.CleanupPartial(targetDir)
		return fmt.Errorf("failed to extract template: %w", err)
	}

	// Clean up downloaded ZIP
	os.Remove(zipPath)

	// Update AI tool config with GitHub token if needed
	fmt.Println("🔧 Configuring...")
	token, err := auth.LoadToken()
	if err != nil {
		logger.Warn("Failed to load GitHub token: %v", err)
	} else if token.GitHubToken != "" {
		// Load config to get MCP server endpoint
		cfg, err := config.Load()
		if err != nil {
			logger.Warn("Failed to load config: %v", err)
		} else {
			if err := template.UpdateAIToolConfig(aiTool, targetDir, token.GitHubToken, cfg.MCPServerEndpoint); err != nil {
				logger.Warn("Failed to update AI tool config: %v", err)
			} else {
				logger.Info("Successfully updated GitHub token in %s config", aiTool)
			}
		}
	}

	// Install beads-mcp (requires uv) - only if flag is set
	if installBeads {
		fmt.Println("🔮 Installing beads-mcp...")
		beadsResult := beads.EnsureInstalled()
		if beadsResult.Error != nil {
			logger.Warn("beads-mcp installation failed: %v", beadsResult.Error)
			fmt.Printf("  ⚠ %s\n", beadsResult.Message)
		} else if beadsResult.Installed {
			fmt.Printf("  ✓ %s\n", beadsResult.Message)
		} else {
			fmt.Printf("  ⚠ %s\n", beadsResult.Message)
		}
	}

	// Install VS Code extension
	fmt.Println("📦 Installing VS Code extension...")
	result := vscode.InstallExtension()
	if result.Error != nil {
		logger.Warn("Extension installation failed: %v", result.Error)
		fmt.Printf("  ⚠ %s\n", result.Message)
	} else if result.Installed {
		fmt.Printf("  ✓ %s\n", result.Message)
	} else {
		fmt.Printf("  ⚠ %s\n", result.Message)
	}

	// Success message
	fmt.Printf("\n✓ Project initialized successfully!\n")
	fmt.Printf("  Directory: %s\n", ui.ShortenPath(targetDir))
	fmt.Printf("  AI tool: %s\n\n", aiTool)

	if projectName != "." {
		fmt.Println("-> Next steps:")
		fmt.Printf("  cd %s\n", projectName)
	}

	fmt.Println("\n  Enjoy building with MoMorph! 🚀")

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
			return ErrUserCancelled
		}
	}

	return nil
}

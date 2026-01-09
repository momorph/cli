package cmd

import (
	"context"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/ui"
	"github.com/momorph/cli/internal/update"
	"github.com/momorph/cli/internal/version"
	"github.com/spf13/cobra"
)

var (
	checkOnly bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update MoMorph CLI to the latest version",
	Example: `  momorph update           # Check and install update
  momorph update --check   # Only check for updates`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, don't install")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	currentVersion := version.Version
	fmt.Printf("Current version: %s\n\n", currentVersion)

	// Check for latest release
	fmt.Println("🔍 Checking for updates...")
	release, err := update.GetLatestRelease(ctx)
	if err != nil {
		logger.Error("Failed to check for updates", err)
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := release.GetVersion()
	logger.Debug("Latest version: %s", latestVersion)

	// Compare versions
	comparison := update.CompareVersions(currentVersion, latestVersion)

	if comparison >= 0 {
		fmt.Println(lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Render("✓ Already on the latest version!"))
		return nil
	}

	// Update available
	fmt.Printf("\n%s %s → %s\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("⚡ Update available:"),
		currentVersion,
		lipgloss.NewStyle().Bold(true).Render(latestVersion))

	fmt.Printf("   Release notes: %s\n\n", release.HTMLURL)

	// If only checking, stop here
	if checkOnly {
		fmt.Println("Run 'momorph update' (without --check) to install the update.")
		return nil
	}

	// Get platform-specific asset
	asset, err := release.GetAssetForPlatform()
	if err != nil {
		logger.Error("Failed to find release asset", err)
		return fmt.Errorf("failed to find release for your platform: %w", err)
	}

	logger.Debug("Downloading: %s", asset.Name)

	// Download and install
	fmt.Printf("📥 Downloading %s...\n", asset.Name)
	progressBar := ui.NewProgressBar(asset.Size)

	err = update.DownloadAndReplace(ctx, asset, func(downloaded, total int64) {
		progressBar.Update(downloaded)
	})
	progressBar.Finish()

	if err != nil {
		logger.Error("Failed to update", err)
		return fmt.Errorf("failed to update: %w", err)
	}

	fmt.Println(lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Bold(true).
		Render("\n✓ Updated successfully!"))

	fmt.Printf("  Restart the CLI to use version %s\n", latestVersion)

	return nil
}

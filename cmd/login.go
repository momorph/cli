package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/momorph/cli/internal/auth"
	"github.com/momorph/cli/internal/logger"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with MoMorph using GitHub",
	Example: `  momorph login              # Start authentication flow
  momorph login --debug      # Start with debug logging enabled`,
	RunE: runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Setup signal handling for graceful cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n✗ Login cancelled by user")
		cancel()
		os.Exit(0)
	}()

	// Check if already authenticated
	if auth.IsAuthenticated() {
		fmt.Println("✓ Already authenticated. Use 'momorph logout' to sign out.")
		return nil
	}

	// Request device code
	fmt.Println("🔑 Requesting device code from GitHub")
	deviceCode, err := auth.RequestDeviceCode(ctx)
	if err != nil {
		logger.Error("Failed to request device code", err)
		return fmt.Errorf("failed to request device code: %w", err)
	}

	// Display verification code
	codeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42")).
		Background(lipgloss.Color("235")).
		Padding(0, 1)

	fmt.Printf("\n1. Press Enter to open your browser: %s\n", lipgloss.NewStyle().Underline(true).Render(deviceCode.VerificationURI))
	fmt.Printf("2. Enter this code: %s\n", codeStyle.Render(deviceCode.UserCode))
	fmt.Printf("\n%s", lipgloss.NewStyle().Faint(true).Render("Press Enter to continue..."))

	// Wait for user to press enter
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	// Open browser
	fmt.Println("\n🌐 Opening browser...")
	if err := openBrowser(deviceCode.VerificationURI); err != nil {
		logger.Warn("Failed to open browser: %v", err)
		fmt.Printf("⚠  Could not open browser automatically. Please visit: %s\n\n", deviceCode.VerificationURI)
	} else {
		fmt.Println("")
	}

	// Poll for token
	fmt.Println("⏳ Waiting for authorization...")

	pollCtx, pollCancel := context.WithTimeout(ctx, time.Duration(deviceCode.ExpiresIn)*time.Second)
	defer pollCancel()

	tokenResp, err := auth.PollForToken(pollCtx, deviceCode.DeviceCode, deviceCode.Interval)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return nil // User cancelled
		}
		logger.Error("Failed to get GitHub token", err)
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	// Get user info to display
	fmt.Println("👤 Fetching user information...")
	moMorphUser, err := auth.GetMoMorphUser(ctx, tokenResp.AccessToken)
	if err != nil {
		logger.Error("Failed to get user info", err)
		return fmt.Errorf("failed to get user info: %w", err)
	}

	// Save GitHub access token
	fmt.Println("💾 Saving credentials...")
	if err := auth.SaveToken(tokenResp.AccessToken); err != nil {
		logger.Error("Failed to save token", err)
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render("✓ Successfully authenticated!"))
	fmt.Printf("  Logged in as: %s\n", lipgloss.NewStyle().Bold(true).Render(maskEmail(moMorphUser.Email)))

	return nil
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

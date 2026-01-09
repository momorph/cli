package cmd

import (
	"context"
	"fmt"
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

	// Check if already authenticated
	if auth.IsAuthenticated() {
		fmt.Println("✓ Already authenticated. Use 'momorph logout' to sign out.")
		return nil
	}

	// Request device code
	fmt.Println("🔑 Requesting device code from GitHub...")
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

	fmt.Println("\n" + lipgloss.NewStyle().Bold(true).Render("📱 GitHub Device Flow Authentication"))
	fmt.Println(lipgloss.NewStyle().Faint(true).Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Printf("\n1. Visit: %s\n", lipgloss.NewStyle().Underline(true).Render(deviceCode.VerificationURI))
	fmt.Printf("2. Enter code: %s\n\n", codeStyle.Render(deviceCode.UserCode))

	// Poll for token
	fmt.Println("⏳ Waiting for authorization...")

	pollCtx, cancel := context.WithTimeout(ctx, time.Duration(deviceCode.ExpiresIn)*time.Second)
	defer cancel()

	tokenResp, err := auth.PollForToken(pollCtx, deviceCode.DeviceCode, deviceCode.Interval)
	if err != nil {
		logger.Error("Failed to get GitHub token", err)
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	// Get user info
	fmt.Println("👤 Fetching user information...")
	user, err := auth.GetAuthenticatedUser(ctx, tokenResp.AccessToken)
	if err != nil {
		logger.Error("Failed to get user info", err)
		return fmt.Errorf("failed to get user info: %w", err)
	}

	// Exchange for MoMorph token
	fmt.Println("🔄 Exchanging for MoMorph token...")
	moMorphToken, err := auth.ExchangeGitHubToken(ctx, tokenResp.AccessToken)
	if err != nil {
		logger.Error("Failed to exchange token", err)
		return fmt.Errorf("failed to exchange token: %w", err)
	}

	// Create auth token
	token := &auth.AuthToken{
		GitHubToken:      tokenResp.AccessToken,
		GitHubTokenType:  tokenResp.TokenType,
		GitHubScopes:     []string{tokenResp.Scope},
		MoMorphToken:     moMorphToken.Token,
		MoMorphExpiresAt: time.Now().Add(4 * time.Hour), // Default 4 hours, will be validated from JWT
		Username:         user.Login,
		AvatarURL:        user.AvatarURL,
		Email:            user.Email,
		UserID:           "", // Not provided by API
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Save token
	fmt.Println("💾 Saving credentials...")
	if err := auth.SaveToken(token); err != nil {
		logger.Error("Failed to save token", err)
		return fmt.Errorf("failed to save token: %w", err)
	}

	logger.Info("User %s authenticated successfully", user.Login)

	fmt.Println("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render("✓ Successfully authenticated!"))
	fmt.Printf("  Logged in as: %s\n", lipgloss.NewStyle().Bold(true).Render(user.Login))

	return nil
}

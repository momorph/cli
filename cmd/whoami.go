package cmd

import (
	"fmt"
	"time"

	"github.com/momorph/cli/internal/auth"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current authenticated user information",
	Example: `  momorph whoami            # Show current user info
  momorph whoami --debug    # Show with debug information`,
	RunE: runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	// Load token
	token, err := auth.LoadToken()
	if err != nil {
		fmt.Println("✗ Not authenticated")
		fmt.Println("\nRun 'momorph login' to authenticate with GitHub and MoMorph")
		return nil
	}

	// Check if token is valid
	if !token.IsValid() {
		fmt.Println("✗ Token expired")
		fmt.Println("\nRun 'momorph login' to reauthenticate")
		return nil
	}

	// Display user info
	fmt.Printf("✓ Authenticated as: %s\n", token.Username)
	if token.Email != "" {
		fmt.Printf("  Email: %s\n", token.Email)
	}
	fmt.Printf("  User ID: %s\n", token.UserID)

	// Token expiration
	expiresIn := time.Until(token.MoMorphExpiresAt)
	if expiresIn > 0 {
		days := int(expiresIn.Hours() / 24)
		hours := int(expiresIn.Hours()) % 24

		if days > 0 {
			fmt.Printf("  Token expires in: %d days, %d hours\n", days, hours)
		} else {
			fmt.Printf("  Token expires in: %d hours\n", hours)
		}
	} else {
		fmt.Println("  Token: Expired")
	}

	// Check if needs refresh
	if token.NeedsRefresh() {
		fmt.Println("\n⚠ Your token will expire soon. Run 'momorph login' to refresh.")
	}

	return nil
}

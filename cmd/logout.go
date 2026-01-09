package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/momorph/cli/internal/auth"
	"github.com/momorph/cli/internal/logger"
	"github.com/spf13/cobra"
)

var (
	forceLogout bool
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and delete stored credentials",
	Example: `  momorph logout            # Log out with confirmation prompt
  momorph logout --force    # Log out without confirmation`,
	RunE: runLogout,
}

func init() {
	logoutCmd.Flags().BoolVar(&forceLogout, "force", false, "Skip confirmation prompt")
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	// Check if authenticated
	if !auth.IsAuthenticated() {
		fmt.Println("Not currently authenticated")
		return nil
	}

	// Get token to show user info
	token, err := auth.LoadToken()
	if err != nil {
		logger.Error("Failed to load token", err)
		return fmt.Errorf("failed to load token: %w", err)
	}

	// Confirm logout unless --force is used
	if !forceLogout {
		fmt.Printf("You are currently signed in as: %s\n", token.Username)
		fmt.Print("Are you sure you want to sign out? (y/N): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Logout cancelled")
			return nil
		}
	}

	// Clear token
	if err := auth.ClearToken(); err != nil {
		logger.Error("Failed to clear token", err)
		return fmt.Errorf("failed to clear credentials: %w", err)
	}

	logger.Info("User %s logged out", token.Username)
	fmt.Println("✓ Successfully signed out")

	return nil
}

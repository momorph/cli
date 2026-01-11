package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/momorph/cli/internal/auth"
	"github.com/momorph/cli/internal/logger"
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

// formatDate formats a date string for display in the specified timezone
func formatDate(dateStr string, timezone string) string {
	if dateStr == "" {
		return "Unknown"
	}

	// Try to parse ISO 8601 format
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// If parsing fails, return the original string
		return dateStr
	}

	// Load timezone if provided
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			t = t.In(loc)
		}
	}

	return t.Format("Jan 02, 2006")
}

func runWhoami(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load token
	token, err := auth.LoadToken()
	if err != nil {
		fmt.Println("✗ Not authenticated")
		fmt.Println("\nRun 'momorph login' to authenticate with GitHub and MoMorph")
		return nil
	}

	// Check if token is valid
	if !token.IsValid() {
		fmt.Println("✗ Token invalid")
		fmt.Println("\nRun 'momorph login' to reauthenticate")
		return nil
	}

	// Fetch fresh user info from MoMorph API
	user, err := auth.GetMoMorphUser(ctx, token.GitHubToken)
	if err != nil {
		logger.Error("Failed to get user info", err)
		fmt.Println("✗ Failed to fetch user information")
		fmt.Println("\nRun 'momorph login' to reauthenticate")
		return nil
	}

	// Define styles
	// Define styles
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	// labelStyle reserved for future use

	// Display user information as table
	fmt.Println("\n" + headerStyle.Render("👤 User Profile"))
	profileRows := [][]string{
		{"Email", maskEmail(user.Email)},
		{"Created at", formatDate(user.CreatedAt, user.TimeZone)},
		{"Timezone", user.TimeZone},
	}

	profileTable := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("243"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			return lipgloss.NewStyle().Padding(0, 2)
		}).
		Headers("Information", "Value").
		Rows(profileRows...)

	fmt.Println(profileTable.String())
	if len(user.ConnectedAccounts) > 0 {
		fmt.Println("\n" + headerStyle.Render("🔗 Connected Accounts"))

		// Build table rows
		rows := make([][]string, len(user.ConnectedAccounts))
		for i, account := range user.ConnectedAccounts {
			rows[i] = []string{account.Provider, account.Name, maskEmail(account.Email)}
		}

		// Styles for table
		bodyCellStyle := lipgloss.NewStyle().Padding(0, 2)

		// Create table with padding styles
		t := table.New().
			Border(lipgloss.NormalBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("243"))).
			StyleFunc(func(row, col int) lipgloss.Style {
				// Apply padding to all cells; header will inherit formatting from Headers
				return bodyCellStyle
			}).
			Headers("Provider", "Name", "Email").
			Rows(rows...)

		fmt.Println(t.String())
	}

	fmt.Println()
	return nil
}

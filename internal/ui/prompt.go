package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PromptAITool prompts the user to select an AI tool
func PromptAITool() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n🤖 Select AI Tool:")
	fmt.Println("  1. Claude Code")
	fmt.Println("  2. GitHub Copilot")
	fmt.Println("  3. Cursor")
	fmt.Println("  4. Gemini")
	fmt.Println("  5. Windsurf")
	fmt.Print("\nEnter your choice (1-4): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)

	switch input {
	case "1":
		return "claude", nil
	case "2":
		return "copilot", nil
	case "3":
		return "cursor", nil
	case "4":
		return "gemini", nil
	case "5":
		return "windsurf", nil
	default:
		return "", fmt.Errorf("invalid choice: %s", input)
	}
}

// ConfirmOverwrite prompts the user to confirm overwriting a non-empty directory
func ConfirmOverwrite(dirPath string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("⚠  Directory not empty: %s\n", ShortenPath(dirPath))
	fmt.Print("Do you want to continue? (y/N): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

// ConfirmUpdate prompts the user to confirm updating to a new version
func ConfirmUpdate(currentVersion, newVersion string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Do you want to update from %s to %s? (y/N): ", currentVersion, newVersion)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.TrimSpace(strings.ToLower(input))
	// Default to yes (empty input or "y"/"yes")
	return input == "y" || input == "yes", nil
}

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
	fmt.Println("  1. GitHub Copilot")
	fmt.Println("  2. Cursor")
	fmt.Println("  3. Claude Code")
	fmt.Print("\nEnter your choice (1-3): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)

	switch input {
	case "1":
		return "copilot", nil
	case "2":
		return "cursor", nil
	case "3":
		return "claude", nil
	default:
		return "", fmt.Errorf("invalid choice: %s", input)
	}
}

// ConfirmOverwrite prompts the user to confirm overwriting a non-empty directory
func ConfirmOverwrite(dirPath string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n⚠  Warning: Directory '%s' is not empty.\n", dirPath)
	fmt.Print("Do you want to continue? (y/N): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

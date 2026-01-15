package template

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/momorph/cli/internal/logger"
)

// MergeType defines how to merge a specific file type
type MergeType int

const (
	MergeTypeJSON MergeType = iota
	MergeTypeGitignore
)

// MergeableFiles defines which files should be merged instead of overwritten
var MergeableFiles = map[string]MergeType{
	".vscode/settings.json": MergeTypeJSON,
	".mcp.json":             MergeTypeJSON,
	".gitignore":            MergeTypeGitignore,
}

// ShouldMerge checks if a file should be merged based on its relative path
func ShouldMerge(relativePath string) (MergeType, bool) {
	mergeType, exists := MergeableFiles[relativePath]
	return mergeType, exists
}

// MergeJSONFiles performs a deep merge of template JSON into existing JSON file
// Template values are merged into existing values using deep merge strategy
func MergeJSONFiles(existingPath, templatePath string) error {
	// Read existing file
	existingData, err := os.ReadFile(existingPath)
	if err != nil {
		return fmt.Errorf("failed to read existing file: %w", err)
	}

	// Read template file
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Parse both as generic maps
	var existing, template map[string]interface{}
	if err := json.Unmarshal(existingData, &existing); err != nil {
		return fmt.Errorf("failed to parse existing JSON: %w", err)
	}
	if err := json.Unmarshal(templateData, &template); err != nil {
		return fmt.Errorf("failed to parse template JSON: %w", err)
	}

	// Deep merge template into existing
	merged := deepMerge(existing, template)

	// Write merged result with proper formatting
	mergedData, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merged JSON: %w", err)
	}

	if err := os.WriteFile(existingPath, mergedData, 0644); err != nil {
		return fmt.Errorf("failed to write merged file: %w", err)
	}

	logger.Debug("Merged JSON file: %s", existingPath)
	return nil
}

// deepMerge recursively merges template map into existing map
// - Keys only in existing are preserved
// - Keys only in template are added
// - Keys in both: if both are maps, merge recursively; otherwise keep existing value
func deepMerge(existing, template map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy all existing values first
	for k, v := range existing {
		result[k] = v
	}

	// Merge template values
	for k, templateVal := range template {
		existingVal, exists := result[k]
		if !exists {
			// Key doesn't exist in existing, add template value
			result[k] = templateVal
			continue
		}

		// Both exist, check if both are maps for recursive merge
		existingMap, existingIsMap := existingVal.(map[string]interface{})
		templateMap, templateIsMap := templateVal.(map[string]interface{})

		if existingIsMap && templateIsMap {
			// Recursive merge for nested objects
			result[k] = deepMerge(existingMap, templateMap)
		}
		// Otherwise, keep existing value (existing takes precedence)
	}

	return result
}

// MergeGitignoreFiles appends unique lines from template .gitignore to existing .gitignore
func MergeGitignoreFiles(existingPath, templatePath string) error {
	// Read existing lines into a set for deduplication
	existingLines, err := readLinesAsSet(existingPath)
	if err != nil {
		return fmt.Errorf("failed to read existing .gitignore: %w", err)
	}

	// Read template lines
	templateLines, err := readLines(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template .gitignore: %w", err)
	}

	// Open existing file for appending
	file, err := os.OpenFile(existingPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open .gitignore for appending: %w", err)
	}
	defer file.Close()

	// Track if we need to add separator
	addedSeparator := false
	addedCount := 0

	// Append unique lines from template
	for _, line := range templateLines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comments when checking for duplicates
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if _, exists := existingLines[trimmed]; !exists {
			if !addedSeparator {
				file.WriteString("\n# Added by MoMorph\n")
				addedSeparator = true
			}
			file.WriteString(line + "\n")
			existingLines[trimmed] = struct{}{} // Mark as added to avoid duplicates
			addedCount++
		}
	}

	if addedCount > 0 {
		logger.Debug("Added %d lines to .gitignore", addedCount)
	}
	return nil
}

// readLinesAsSet reads a file and returns non-empty lines as a set
func readLinesAsSet(path string) (map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed != "" {
			lines[trimmed] = struct{}{}
		}
	}
	return lines, scanner.Err()
}

// readLines reads all lines from a file
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

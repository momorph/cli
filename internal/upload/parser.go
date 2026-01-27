package upload

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ParseFilePath extracts metadata from file path
// Expected format: .momorph/{testcases|specs}/{file_key}/{frame_id}-{frame_name}.csv
// Example: .momorph/testcases/i09vM3jClQiu8cwXsMo6uy/9276:19907-TOP_Channel.csv
func ParseFilePath(fullFilePath string) (*ParsedFilePath, error) {
	// Normalize path separators
	normalizedPath := strings.ReplaceAll(fullFilePath, "\\", "/")

	// Regex to match the expected pattern
	// .momorph/(testcases|specs)/(fileKey)/(frameId)-(frameName).csv
	regex := regexp.MustCompile(`\.momorph/(testcases|specs)/([^/]+)/([^-]+)-([^.]+)\.csv$`)

	match := regex.FindStringSubmatch(normalizedPath)
	if match == nil {
		return nil, fmt.Errorf("file path does not match expected pattern: .momorph/{testcases|specs}/{file_key}/{frame_id}-{frame_name}.csv")
	}

	uploadType := strings.ToLower(match[1])
	fileKey := strings.TrimSpace(match[2])
	frameID := strings.TrimSpace(match[3])
	frameName := strings.TrimSpace(match[4])

	// Validate all parts are non-empty
	if fileKey == "" || frameID == "" || frameName == "" {
		return nil, fmt.Errorf("invalid file path: file_key, frame_id, and frame_name must not be empty")
	}

	return &ParsedFilePath{
		Type:      uploadType,
		FileKey:   fileKey,
		FrameID:   frameID,
		FrameName: frameName,
	}, nil
}

// ParseTestcasesCSV parses a test cases CSV file and returns TestCaseContent
func ParseTestcasesCSV(filePath string) (*TestCaseContent, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file is empty or has no data rows")
	}

	// Build column index map from header
	header := records[0]
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[strings.TrimSpace(col)] = i
	}

	// Parse data rows
	var testCases []TestCase
	for i, row := range records[1:] {
		tc, err := parseTestcaseRow(row, colIndex, i+2) // +2 because 1-indexed and skip header
		if err != nil {
			return nil, fmt.Errorf("error parsing row %d: %w", i+2, err)
		}
		testCases = append(testCases, *tc)
	}

	// Extract screen name from file path
	parsed, err := ParseFilePath(filePath)
	if err != nil {
		return nil, err
	}

	return &TestCaseContent{
		ScreenName: parsed.FrameName,
		TestCases:  testCases,
	}, nil
}

func parseTestcaseRow(row []string, colIndex map[string]int, lineNum int) (*TestCase, error) {
	getValue := func(csvCol string) string {
		if idx, ok := colIndex[csvCol]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	// Map Section value to test_area
	// Server only accepts: 'ACCESSING', 'GUI', 'FUNCTION' for test_area
	section := getValue("Section")
	testArea := section
	if strings.EqualFold(section, "functional") || strings.EqualFold(section, "function") {
		testArea = "FUNCTION"
	}

	return &TestCase{
		ID:             getValue("TC_ID"),
		Step:           getValue("Steps"),
		Category:       getValue("Category"),
		PageName:       getValue("Page_Name"),
		TestArea:       testArea,
		TestData:       getValue("Test_Data"),
		SubCategory:    getValue("Sub_Category"),
		SubSubCategory: getValue("Sub_Sub_Category"),
		PreCondition:   getValue("Precondition"),
		ExpectedResult: getValue("Expected_Result"),
		TCType:         getValue("Testcase_Type"),
		Priority:       getValue("Priority"),
		TestResults:    getValue("Test_Results"),
		ExecutedDate:   getValue("Executed_Date"),
		Tester:         getValue("Tester"),
		Note:           getValue("Note"),
	}, nil
}

// ParseSpecsCSV parses a specs CSV file and returns a slice of Spec
func ParseSpecsCSV(filePath string) ([]Spec, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file is empty or has no data rows")
	}

	// Build column index map from header
	header := records[0]
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[strings.TrimSpace(col)] = i
	}

	// Parse data rows
	var specs []Spec
	for i, row := range records[1:] {
		spec, err := parseSpecRow(row, colIndex, i+2)
		if err != nil {
			return nil, fmt.Errorf("error parsing row %d: %w", i+2, err)
		}
		specs = append(specs, *spec)
	}

	return specs, nil
}

func parseSpecRow(row []string, colIndex map[string]int, lineNum int) (*Spec, error) {
	getValue := func(csvCol string) string {
		if idx, ok := colIndex[csvCol]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	getInt := func(csvCol string) *int {
		val := getValue(csvCol)
		if val == "" {
			return nil
		}
		num, err := strconv.Atoi(val)
		if err != nil {
			return nil
		}
		return &num
	}

	getBool := func(csvCol string) *bool {
		val := getValue(csvCol)
		if val == "" {
			return nil
		}
		lower := strings.ToLower(val)
		if lower == "true" || lower == "yes" || lower == "1" {
			b := true
			return &b
		}
		if lower == "false" || lower == "no" || lower == "0" {
			b := false
			return &b
		}
		// Return nil for unrecognized values
		return nil
	}

	return &Spec{
		No:             getValue("No"),
		DesignItemName: getValue("itemName"),
		Name:           getValue("nameJP"),
		NameTrans:      getValue("nameTrans"),
		NodeLinkID:     getValue("itemId"),
		Type:           getValue("itemType"),
		OtherType:      getValue("itemSubtype"),
		ButtonType:     getValue("buttonType"),
		DataType:       getValue("dataType"),
		Required:       getBool("required"),
		Format:         getValue("format"),
		MinLength:      getInt("minLength"),
		MaxLength:      getInt("maxLength"),
		DefaultValue:   getValue("defaultValue"),
		ValidationNote: getValue("validationNote"),
		Action:         getValue("userAction"),
		LinkedFrameID:  getValue("linkedFrameId"),
		NavigationNote: getValue("transitionNote"),
		TableName:      getValue("databaseTable"),
		ColumnName:     getValue("databaseColumn"),
		DatabaseNote:   getValue("databaseNote"),
		Description:    getValue("description"),
	}, nil
}

// TransformSpecToPayload transforms a Spec to SpecPayload for GraphQL mutation
// Uses type-based conditionals matching SDK's prepareSpecContentPayload
func TransformSpecToPayload(spec Spec, frameID, fileID int, sectionLinkID, status string) *SpecPayload {
	itemType := spec.Type

	// Helper functions for conditional values
	conditionalString := func(condition bool, value string) string {
		if condition {
			return value
		}
		return ""
	}

	conditionalInt := func(condition bool, value *int) *int {
		if condition {
			return value
		}
		return nil
	}

	conditionalBool := func(condition bool, value *bool) *bool {
		if condition {
			return value
		}
		return nil
	}

	// Build nested specs structure with type-based conditionals
	specs := &SpecDetails{
		Item: &ItemSpec{
			Name:      spec.Name,
			NameTrans: spec.NameTrans,
			// Only set buttonType if type is "button"
			ButtonType: conditionalString(itemType == "button", spec.ButtonType),
			// Only set otherType if type is "others"
			OtherType: conditionalString(itemType == "others", spec.OtherType),
		},
		Navigation: &NavigationSpec{
			Action: spec.Action,
			// Only set linkedFrameId if action is present
			LinkedFrameID: conditionalString(spec.Action != "", spec.LinkedFrameID),
			Note:          spec.NavigationNote,
		},
		Validation: &ValidationSpec{
			// Only set dataType for specific types
			DataType: conditionalString(
				contains(TypesRequiringDataType, itemType),
				spec.DataType,
			),
			// Required is nil for button/label types
			Required: conditionalBool(
				!contains(TypesWithoutValidation, itemType),
				spec.Required,
			),
			// Format excluded for button/label
			Format: conditionalString(
				!contains(TypesWithoutValidation, itemType),
				spec.Format,
			),
			// MinLength/MaxLength only for specific types
			MinLength: conditionalInt(
				contains(TypesRequiringLength, itemType),
				spec.MinLength,
			),
			MaxLength: conditionalInt(
				contains(TypesRequiringLength, itemType),
				spec.MaxLength,
			),
			DefaultValue: spec.DefaultValue,
			Note:         spec.ValidationNote,
		},
		Database: &DatabaseSpec{
			// Excluded for button type
			TableName:  conditionalString(itemType != "button", spec.TableName),
			ColumnName: conditionalString(itemType != "button", spec.ColumnName),
			Note:       spec.DatabaseNote,
		},
		Description: spec.Description,
	}

	return &SpecPayload{
		Type:          spec.Type,
		No:            spec.No,
		Name:          spec.DesignItemName,
		Status:        status,
		NodeLinkID:    spec.NodeLinkID,
		SectionLinkID: sectionLinkID,
		FrameID:       frameID,
		FileID:        fileID,
		IsReviewed:    spec.IsReviewed,
		Specs:         specs,
	}
}

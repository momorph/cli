package upload

import (
	"fmt"
	"reflect"
)

// Length constraints matching SDK's UpdateSpecDto
const (
	MaxNameLength           = 255
	MaxNameTransLength      = 255
	MaxButtonTypeLength     = 255
	MaxOtherTypeLength      = 255
	MaxFormatLength         = 255
	MaxDefaultValueLength   = 255
	MaxTableNameLength      = 255
	MaxColumnNameLength     = 255
	MaxNavigationNoteLength = 2000
	MaxValidationNoteLength = 2000
	MaxDatabaseNoteLength   = 2000
	MaxDescriptionLength    = 10000
)

// Accepted types for validation
var AcceptedOptionTypes = []string{
	"button", "checkbox", "radio_button", "dropdown",
	"file_or_image", "video", "date_picker", "pagination",
	"popup_dialog", "label", "text_form", "textarea", "others",
}

var AcceptedDataTypes = []string{
	"array", "boolean", "byte", "character", "string",
	"date", "integer", "long", "short", "float", "double", "nothing",
}

var AcceptedButtonTypes = []string{"icon_text", "toggle", "text_link", "others"}
var AcceptedActionTypes = []string{"on_click", "while_hovering", "key_gamepad", "after_delay"}

// Types requiring specific validations
var TypesRequiringDataType = []string{"textarea", "text_form", "others"}
var TypesRequiringLength = []string{"textarea", "text_form", "file_or_image", "video", "others"}
var TypesWithoutValidation = []string{"button", "label"}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ValidateSpecContent validates a spec content using the same validation logic as SDK's UpdateSpecDto
func ValidateSpecContent(spec *Spec, status string) []string {
	var errors []string
	isCompleted := status == DesignItemStatusCompleted
	itemType := spec.Type

	// ==================== TYPE VALIDATION ====================
	if isCompleted || itemType != "" {
		if itemType == "" {
			errors = append(errors, "type is required when status is completed")
		} else if !contains(AcceptedOptionTypes, itemType) {
			errors = append(errors, fmt.Sprintf("type must be one of: %v", AcceptedOptionTypes))
		}
	}

	// ==================== ITEM SPECS VALIDATION ====================
	// name validation
	if (isCompleted || spec.Name != "") && len(spec.Name) > MaxNameLength {
		errors = append(errors, fmt.Sprintf("name must not exceed %d characters", MaxNameLength))
	}

	// nameTrans validation
	if (isCompleted || spec.NameTrans != "") && len(spec.NameTrans) > MaxNameTransLength {
		errors = append(errors, fmt.Sprintf("nameTrans must not exceed %d characters", MaxNameTransLength))
	}

	// buttonType validation - required when type is BUTTON and status is COMPLETED
	if (itemType == "button" && isCompleted) || spec.ButtonType != "" {
		if spec.ButtonType != "" && !contains(AcceptedButtonTypes, spec.ButtonType) {
			errors = append(errors, fmt.Sprintf("buttonType must be one of: %v", AcceptedButtonTypes))
		}
	}

	// otherType validation - required when type is OTHERS and status is COMPLETED
	if (itemType == "others" && isCompleted) || spec.OtherType != "" {
		if len(spec.OtherType) > MaxOtherTypeLength {
			errors = append(errors, fmt.Sprintf("otherType must not exceed %d characters", MaxOtherTypeLength))
		}
	}

	// ==================== NAVIGATION SPECS VALIDATION ====================
	// action validation
	if spec.Action != "" {
		if !contains(AcceptedActionTypes, spec.Action) {
			errors = append(errors, fmt.Sprintf("action must be one of: %v", AcceptedActionTypes))
		}
	}

	// navigationNote validation
	if (spec.Action != "" && isCompleted) || spec.NavigationNote != "" {
		if len(spec.NavigationNote) > MaxNavigationNoteLength {
			errors = append(errors, fmt.Sprintf("navigationNote must not exceed %d characters", MaxNavigationNoteLength))
		}
	}

	// ==================== VALIDATION SPECS VALIDATION ====================
	// dataType validation - required for specific types when COMPLETED
	if (contains(TypesRequiringDataType, itemType) && isCompleted) || spec.DataType != "" {
		if spec.DataType != "" && !contains(AcceptedDataTypes, spec.DataType) {
			errors = append(errors, fmt.Sprintf("dataType must be one of: %v", AcceptedDataTypes))
		}
	}

	// format validation
	if (!contains(TypesWithoutValidation, itemType) && isCompleted) || spec.Format != "" {
		if len(spec.Format) > MaxFormatLength {
			errors = append(errors, fmt.Sprintf("format must not exceed %d characters", MaxFormatLength))
		}
	}

	// minLength/maxLength validation
	requiresLength := contains(TypesRequiringLength, itemType) && isCompleted

	if requiresLength || spec.MinLength != nil {
		if spec.MinLength != nil && *spec.MinLength < 0 {
			errors = append(errors, "minLength must be greater than or equal to 0")
		}
	}

	if requiresLength || spec.MaxLength != nil {
		if spec.MaxLength != nil && *spec.MaxLength < 0 {
			errors = append(errors, "maxLength must be greater than or equal to 0")
		}
	}

	// Cross-field validation: minLength < maxLength
	if spec.MinLength != nil && spec.MaxLength != nil {
		if *spec.MinLength >= *spec.MaxLength {
			errors = append(errors, "minLength must be less than maxLength")
		}
	}

	// defaultValue validation
	if (isCompleted || spec.DefaultValue != "") && len(spec.DefaultValue) > MaxDefaultValueLength {
		errors = append(errors, fmt.Sprintf("defaultValue must not exceed %d characters", MaxDefaultValueLength))
	}

	// validationNote validation
	if (isCompleted || spec.ValidationNote != "") && len(spec.ValidationNote) > MaxValidationNoteLength {
		errors = append(errors, fmt.Sprintf("validationNote must not exceed %d characters", MaxValidationNoteLength))
	}

	// ==================== DATABASE SPECS VALIDATION ====================
	requiresDatabase := isCompleted && itemType != "button"

	// tableName validation
	if (requiresDatabase || spec.TableName != "") && len(spec.TableName) > MaxTableNameLength {
		errors = append(errors, fmt.Sprintf("tableName must not exceed %d characters", MaxTableNameLength))
	}

	// columnName validation
	if (requiresDatabase || spec.ColumnName != "") && len(spec.ColumnName) > MaxColumnNameLength {
		errors = append(errors, fmt.Sprintf("columnName must not exceed %d characters", MaxColumnNameLength))
	}

	// databaseNote validation
	if (requiresDatabase || spec.DatabaseNote != "") && len(spec.DatabaseNote) > MaxDatabaseNoteLength {
		errors = append(errors, fmt.Sprintf("databaseNote must not exceed %d characters", MaxDatabaseNoteLength))
	}

	// ==================== DESCRIPTION VALIDATION ====================
	if (isCompleted || spec.Description != "") && len(spec.Description) > MaxDescriptionLength {
		errors = append(errors, fmt.Sprintf("description must not exceed %d characters", MaxDescriptionLength))
	}

	return errors
}

// IsSpecContentEmpty checks if spec content is empty (only contains structural/metadata fields)
func IsSpecContentEmpty(spec *Spec) bool {
	if spec == nil {
		return true
	}

	// Check if all content fields are empty
	return spec.Name == "" &&
		spec.NameTrans == "" &&
		spec.Type == "" &&
		spec.ButtonType == "" &&
		spec.OtherType == "" &&
		spec.Action == "" &&
		spec.LinkedFrameID == "" &&
		spec.NavigationNote == "" &&
		spec.DataType == "" &&
		spec.Required == nil &&
		spec.Format == "" &&
		spec.MinLength == nil &&
		spec.MaxLength == nil &&
		spec.DefaultValue == "" &&
		spec.ValidationNote == "" &&
		spec.TableName == "" &&
		spec.ColumnName == "" &&
		spec.DatabaseNote == "" &&
		spec.Description == ""
}

// MapSpecForComparison extracts fields for comparison
// Matches SDK's mapSpecForComparison()
func MapSpecForComparison(spec *Spec) map[string]interface{} {
	if spec == nil {
		return nil
	}

	result := map[string]interface{}{
		"name":           spec.Name,
		"nameTrans":      spec.NameTrans,
		"type":           spec.Type,
		"buttonType":     spec.ButtonType,
		"otherType":      spec.OtherType,
		"action":         spec.Action,
		"linkedFrameId":  spec.LinkedFrameID,
		"navigationNote": spec.NavigationNote,
		"dataType":       spec.DataType,
		"format":         spec.Format,
		"defaultValue":   spec.DefaultValue,
		"validationNote": spec.ValidationNote,
		"tableName":      spec.TableName,
		"columnName":     spec.ColumnName,
		"databaseNote":   spec.DatabaseNote,
		"description":    spec.Description,
	}

	// Handle pointer fields
	if spec.Required != nil {
		result["required"] = *spec.Required
	} else {
		result["required"] = nil
	}

	if spec.MinLength != nil {
		result["minLength"] = *spec.MinLength
	} else {
		result["minLength"] = nil
	}

	if spec.MaxLength != nil {
		result["maxLength"] = *spec.MaxLength
	} else {
		result["maxLength"] = nil
	}

	return result
}

// CompareSpecs returns true if specs are equal
func CompareSpecs(current, previous map[string]interface{}) bool {
	if current == nil && previous == nil {
		return true
	}
	if current == nil || previous == nil {
		return false
	}
	return reflect.DeepEqual(current, previous)
}

// DetermineSpecStatus determines the appropriate status for a spec
// Returns (status, validationErrors)
func DetermineSpecStatus(spec *Spec, existingStatus string) (string, []string) {
	// If spec content is empty, status is "none"
	if IsSpecContentEmpty(spec) {
		return DesignItemStatusNone, nil
	}

	// Try COMPLETED status first
	completedErrors := ValidateSpecContent(spec, DesignItemStatusCompleted)
	if len(completedErrors) == 0 {
		return DesignItemStatusCompleted, nil
	}

	// Try DRAFT status if COMPLETED fails
	draftErrors := ValidateSpecContent(spec, DesignItemStatusDraft)
	if len(draftErrors) == 0 {
		return DesignItemStatusDraft, nil
	}

	// Return draft errors if both fail (draft is more lenient)
	return DesignItemStatusDraft, draftErrors
}

package upload

// TestCase represents a single test case item
type TestCase struct {
	ID             string `json:"ID"`
	Step           string `json:"step,omitempty"`
	Category       string `json:"category,omitempty"`
	PageName       string `json:"page_name,omitempty"`
	TestArea       string `json:"test_area,omitempty"`
	TestData       string `json:"test_data,omitempty"`
	SubCategory    string `json:"sub_category,omitempty"`
	SubSubCategory string `json:"sub_sub_category,omitempty"`
	PreCondition   string `json:"pre_condition,omitempty"`
	ExpectedResult string `json:"expected_result,omitempty"`
	TCType         string `json:"tc_type,omitempty"`
	Priority       string `json:"priority,omitempty"`
	TestResults    string `json:"test_results,omitempty"`
	ExecutedDate   string `json:"executed_date,omitempty"`
	Tester         string `json:"tester,omitempty"`
	Note           string `json:"note,omitempty"`
}

// TestCaseContent represents the content payload for uploading test cases
type TestCaseContent struct {
	ScreenName string     `json:"screen_name"`
	TestCases  []TestCase `json:"test_cases"`
}

// Design item status constants
const (
	DesignItemStatusDeleted   = "deleted"
	DesignItemStatusNone      = "none"
	DesignItemStatusDraft     = "draft"
	DesignItemStatusCompleted = "completed"
)

// Spec represents a single spec item from CSV
type Spec struct {
	No             string `json:"no"`
	DesignItemName string `json:"design_item_name"`
	Name           string `json:"name"`
	NameTrans      string `json:"nameTrans,omitempty"`
	NodeLinkID     string `json:"node_link_id,omitempty"`
	SectionLinkID  string `json:"section_link_id,omitempty"`
	Type           string `json:"type,omitempty"`
	OtherType      string `json:"otherType,omitempty"`
	ButtonType     string `json:"buttonType,omitempty"`
	DataType       string `json:"dataType,omitempty"`
	Required       *bool  `json:"required,omitempty"`
	Format         string `json:"format,omitempty"`
	MinLength      *int   `json:"minLength,omitempty"`
	MaxLength      *int   `json:"maxLength,omitempty"`
	DefaultValue   string `json:"defaultValue,omitempty"`
	ValidationNote string `json:"validationNote,omitempty"`
	Action         string `json:"action,omitempty"`
	LinkedFrameID  string `json:"linkedFrameId,omitempty"`
	NavigationNote string `json:"navigationNote,omitempty"`
	TableName      string `json:"tableName,omitempty"`
	ColumnName     string `json:"columnName,omitempty"`
	DatabaseNote   string `json:"databaseNote,omitempty"`
	Description    string `json:"description,omitempty"`
	IsReviewed     *bool  `json:"is_reviewed,omitempty"`
}

// ValidatedSpec represents a spec with validation results
type ValidatedSpec struct {
	Spec
	Status   string   // determined status: none, draft, completed
	IsValid  bool     // whether spec passed validation
	Errors   []string // validation error messages
	Changed  bool     // whether spec differs from existing
	IsNew    bool     // whether this is a new item (not in DB)
	Existing *Spec    // reference to existing spec if any
}

// SpecPayload represents the transformed payload for GraphQL mutation
type SpecPayload struct {
	Type          string       `json:"type,omitempty"`
	NodeType      string       `json:"node_type,omitempty"`
	No            string       `json:"no"`
	Name          string       `json:"name"`
	Status        string       `json:"status,omitempty"`
	SectionLinkID string       `json:"section_link_id,omitempty"`
	NodeLinkID    string       `json:"node_link_id,omitempty"`
	FrameID       int          `json:"frame_id,omitempty"`
	FileID        int          `json:"file_id,omitempty"`
	IsReviewed    *bool        `json:"is_reviewed,omitempty"`
	Parent        string       `json:"parent,omitempty"`
	Position      interface{}  `json:"position,omitempty"`
	Specs         *SpecDetails `json:"specs,omitempty"`
}

// SpecDetails contains the detailed spec information
type SpecDetails struct {
	Item        *ItemSpec       `json:"item,omitempty"`
	Navigation  *NavigationSpec `json:"navigation,omitempty"`
	Validation  *ValidationSpec `json:"validation,omitempty"`
	Database    *DatabaseSpec   `json:"database,omitempty"`
	Description string          `json:"description,omitempty"`
}

// ItemSpec contains item-related spec fields
type ItemSpec struct {
	Name       string `json:"name,omitempty"`
	NameTrans  string `json:"nameTrans,omitempty"`
	ButtonType string `json:"buttonType,omitempty"`
	OtherType  string `json:"otherType,omitempty"`
}

// NavigationSpec contains navigation-related spec fields
type NavigationSpec struct {
	Action          string `json:"action,omitempty"`
	LinkedFrameID   string `json:"linkedFrameId,omitempty"`
	LinkedFrameName string `json:"linkedFrameName,omitempty"`
	Note            string `json:"note,omitempty"`
}

// ValidationSpec contains validation-related spec fields
type ValidationSpec struct {
	DataType     string `json:"dataType,omitempty"`
	Required     *bool  `json:"required,omitempty"`
	Format       string `json:"format,omitempty"`
	MinLength    *int   `json:"minLength,omitempty"`
	MaxLength    *int   `json:"maxLength,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
	Note         string `json:"note,omitempty"`
}

// DatabaseSpec contains database-related spec fields
type DatabaseSpec struct {
	TableName  string `json:"tableName,omitempty"`
	ColumnName string `json:"columnName,omitempty"`
	Note       string `json:"note,omitempty"`
}

// ParsedFilePath contains extracted parameters from file path
type ParsedFilePath struct {
	Type      string // "testcases" or "specs"
	FileKey   string // Figma file key
	FrameID   string // Frame ID (can contain colons)
	FrameName string // Frame name
}

// UploadStatus represents the status of a file upload
type UploadStatus string

const (
	StatusSuccess UploadStatus = "success"
	StatusFailed  UploadStatus = "failed"
	StatusSkipped UploadStatus = "skipped"
)

// UploadResult represents the result of uploading a single file
type UploadResult struct {
	FilePath string
	FileName string
	Status   UploadStatus
	Error    error
	Message  string
}

// UploadSummary contains aggregated upload results
type UploadSummary struct {
	Total     int
	Success   int
	Failed    int
	Skipped   int
	Results   []UploadResult
}

// NewUploadSummary creates a new UploadSummary from results
func NewUploadSummary(results []UploadResult) *UploadSummary {
	summary := &UploadSummary{
		Total:   len(results),
		Results: results,
	}
	for _, r := range results {
		switch r.Status {
		case StatusSuccess:
			summary.Success++
		case StatusFailed:
			summary.Failed++
		case StatusSkipped:
			summary.Skipped++
		}
	}
	return summary
}

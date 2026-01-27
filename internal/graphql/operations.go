package graphql

import (
	"context"
	"encoding/json"
	"fmt"
)

// Frame represents a MoMorph frame
type Frame struct {
	ID          int    `json:"id"`
	FrameLinkID string `json:"frame_link_id"`
	FileID      int    `json:"file_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
}

// FrameTestCase represents a test case for a frame
type FrameTestCase struct {
	ID            int             `json:"id"`
	TestcasableID int             `json:"testcasable_id"`
	Content       json.RawMessage `json:"content"`
	BaseStructure json.RawMessage `json:"base_structure"`
	Status        string          `json:"status"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

// DesignItem represents a design item
type DesignItem struct {
	ID            int             `json:"id"`
	No            string          `json:"no"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	NodeLinkID    string          `json:"node_link_id"`
	SectionLinkID string          `json:"section_link_id"`
	FrameID       int             `json:"frame_id"`
	Status        string          `json:"status"`
	Specs         json.RawMessage `json:"specs"`
	IsReviewed    bool            `json:"is_reviewed"`
}

// MorpheusUser represents a MoMorph user
type MorpheusUser struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

// GraphQL queries
const (
	// GetFrame query - uses Hasura standard query with where filter
	queryGetFrame = `
query GetFrame($fileKey: String!, $frameLinkId: String!) {
  frames(
    where: {
      _and: [
        {file: {file_key: {_eq: $fileKey}}},
        {frame_link_id: {_eq: $frameLinkId}}
      ]
    },
    limit: 1
  ) {
    id
    frame_link_id
    file_id
    name
    status
  }
}
`

	// GetFrameTestCases query - uses Hasura standard query with where filter
	queryGetFrameTestCases = `
query GetFrameTestCases($fileKey: String!, $frameLinkId: String!) {
  frame_testcases(
    where: {
      _and: [
        {frame: {file: {file_key: {_eq: $fileKey}}}},
        {frame: {frame_link_id: {_eq: $frameLinkId}}}
      ]
    }
  ) {
    id
    testcasable_id
    content
    base_structure
    status
    created_at
    updated_at
  }
}
`

	// ListDesignItemsByNodeLinkIds query
	queryListDesignItemsByNodeLinkIds = `
query ListDesignItemsByNodeLinkIds($fileKey: String!, $frameLinkId: String!, $nodeLinkIds: [String!]!) {
  design_items(
    where: {
      _and: [
        {frame: {frame_link_id: {_eq: $frameLinkId}}},
        {frame: {file: {file_key: {_eq: $fileKey}}}},
        {node_link_id: {_in: $nodeLinkIds}}
      ]
    }
  ) {
    id
    no
    name
    type
    node_link_id
    section_link_id
    frame_id
    status
    specs
    is_reviewed
  }
}
`

	// GetMorpheusUserByEmail query
	queryGetMorpheusUserByEmail = `
query GetMorpheusUserByEmail($email: String!) {
  morpheus_users(where: {email: {_eq: $email}}, limit: 1) {
    id
    email
  }
}
`
)

// GraphQL mutations
const (
	// InsertFrameTestcase mutation
	mutationInsertFrameTestcase = `
mutation InsertFrameTestcase($item: frame_testcases_insert_input!) {
  insert_frame_testcases(objects: [$item]) {
    returning {
      id
      testcasable_id
      content
      status
      created_at
      updated_at
    }
  }
}
`

	// UpdateFrameTestcase mutation
	mutationUpdateFrameTestcase = `
mutation UpdateFrameTestcase($id: bigint!, $content: jsonb!, $baseStructure: jsonb) {
  update_frame_testcases(
    where: {id: {_eq: $id}},
    _set: {content: $content, base_structure: $baseStructure}
  ) {
    returning {
      id
      testcasable_id
      content
      status
      updated_at
    }
  }
}
`

	// UpsertMultipleDesignItemSpecs mutation
	// Uses constraint: design_items_section_link_id_node_link_id_file_id_key
	// which requires section_link_id, node_link_id, and file_id to be unique
	mutationUpsertDesignItemSpecs = `
mutation UpsertMultipleDesignItemSpecs($items: [design_items_insert_input!]!) {
  insert_design_items(
    objects: $items,
    on_conflict: {
      constraint: design_items_section_link_id_node_link_id_file_id_key,
      update_columns: [specs, type, status, name, no, is_reviewed]
    }
  ) {
    returning {
      id
      no
      name
      node_link_id
      status
      specs
    }
  }
}
`

	// InsertDesignItemRevs mutation
	mutationInsertDesignItemRevs = `
mutation InsertDesignItemRevs($revs: [design_items_revs_insert_input!]!) {
  insert_design_items_revs(objects: $revs) {
    affected_rows
  }
}
`
)

// GetFrame fetches a frame by file key and frame ID
func (c *Client) GetFrame(ctx context.Context, fileKey, frameID string) (*Frame, error) {
	variables := map[string]interface{}{
		"fileKey":     fileKey,
		"frameLinkId": frameID,
	}

	var result struct {
		Frames []Frame `json:"frames"`
	}

	if err := c.ExecuteWithResult(ctx, queryGetFrame, variables, &result); err != nil {
		return nil, err
	}

	if len(result.Frames) == 0 {
		return nil, fmt.Errorf("frame not found: fileKey=%s, frameId=%s", fileKey, frameID)
	}

	return &result.Frames[0], nil
}

// GetFrameTestCases fetches test cases for a frame
func (c *Client) GetFrameTestCases(ctx context.Context, fileKey, frameID string) ([]FrameTestCase, error) {
	variables := map[string]interface{}{
		"fileKey":     fileKey,
		"frameLinkId": frameID,
	}

	var result struct {
		FrameTestcases []FrameTestCase `json:"frame_testcases"`
	}

	if err := c.ExecuteWithResult(ctx, queryGetFrameTestCases, variables, &result); err != nil {
		return nil, err
	}

	return result.FrameTestcases, nil
}

// InsertFrameTestcase creates a new test case for a frame
func (c *Client) InsertFrameTestcase(ctx context.Context, testcasableID int, content interface{}) (*FrameTestCase, error) {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	variables := map[string]interface{}{
		"item": map[string]interface{}{
			"testcasable_id":   testcasableID,
			"testcasable_type": "frame",
			"status":           "received",
			"content":          json.RawMessage(contentJSON),
		},
	}

	var result struct {
		InsertFrameTestcases struct {
			Returning []FrameTestCase `json:"returning"`
		} `json:"insert_frame_testcases"`
	}

	if err := c.ExecuteWithResult(ctx, mutationInsertFrameTestcase, variables, &result); err != nil {
		return nil, err
	}

	if len(result.InsertFrameTestcases.Returning) == 0 {
		return nil, fmt.Errorf("failed to insert test case: no result returned")
	}

	return &result.InsertFrameTestcases.Returning[0], nil
}

// UpdateFrameTestcase updates an existing test case
func (c *Client) UpdateFrameTestcase(ctx context.Context, id int, content interface{}) (*FrameTestCase, error) {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	variables := map[string]interface{}{
		"id":      id,
		"content": json.RawMessage(contentJSON),
	}

	var result struct {
		UpdateFrameTestcases struct {
			Returning []FrameTestCase `json:"returning"`
		} `json:"update_frame_testcases"`
	}

	if err := c.ExecuteWithResult(ctx, mutationUpdateFrameTestcase, variables, &result); err != nil {
		return nil, err
	}

	if len(result.UpdateFrameTestcases.Returning) == 0 {
		return nil, fmt.Errorf("failed to update test case: no result returned")
	}

	return &result.UpdateFrameTestcases.Returning[0], nil
}

// ListDesignItemsByNodeLinkIds fetches design items by node link IDs
func (c *Client) ListDesignItemsByNodeLinkIds(ctx context.Context, fileKey, frameID string, nodeLinkIds []string) ([]DesignItem, error) {
	variables := map[string]interface{}{
		"fileKey":     fileKey,
		"frameLinkId": frameID,
		"nodeLinkIds": nodeLinkIds,
	}

	var result struct {
		DesignItems []DesignItem `json:"design_items"`
	}

	if err := c.ExecuteWithResult(ctx, queryListDesignItemsByNodeLinkIds, variables, &result); err != nil {
		return nil, err
	}

	return result.DesignItems, nil
}

// UpsertDesignItemSpecs upserts multiple design item specs
func (c *Client) UpsertDesignItemSpecs(ctx context.Context, items []map[string]interface{}) ([]DesignItem, error) {
	variables := map[string]interface{}{
		"items": items,
	}

	var result struct {
		InsertDesignItems struct {
			Returning []DesignItem `json:"returning"`
		} `json:"insert_design_items"`
	}

	if err := c.ExecuteWithResult(ctx, mutationUpsertDesignItemSpecs, variables, &result); err != nil {
		return nil, err
	}

	return result.InsertDesignItems.Returning, nil
}

// GetMorpheusUserByEmail fetches a user by email
func (c *Client) GetMorpheusUserByEmail(ctx context.Context, email string) (*MorpheusUser, error) {
	variables := map[string]interface{}{
		"email": email,
	}

	var result struct {
		MorpheusUsers []MorpheusUser `json:"morpheus_users"`
	}

	if err := c.ExecuteWithResult(ctx, queryGetMorpheusUserByEmail, variables, &result); err != nil {
		return nil, err
	}

	if len(result.MorpheusUsers) == 0 {
		return nil, fmt.Errorf("user not found: email=%s", email)
	}

	return &result.MorpheusUsers[0], nil
}

// InsertDesignItemRevs inserts design item revisions
func (c *Client) InsertDesignItemRevs(ctx context.Context, revs []map[string]interface{}) (int, error) {
	variables := map[string]interface{}{
		"revs": revs,
	}

	var result struct {
		InsertDesignItemsRevs struct {
			AffectedRows int `json:"affected_rows"`
		} `json:"insert_design_items_revs"`
	}

	if err := c.ExecuteWithResult(ctx, mutationInsertDesignItemRevs, variables, &result); err != nil {
		return 0, err
	}

	return result.InsertDesignItemsRevs.AffectedRows, nil
}

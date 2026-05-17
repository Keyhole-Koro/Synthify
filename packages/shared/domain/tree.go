package domain

import (
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

type Tree struct {
	TreeID      string `json:"tree_id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type Item struct {
	ItemID            string                     `json:"id"`
	WorkspaceID       string                     `json:"workspace_id"`
	Title             string                     `json:"title"`
	Level             int                        `json:"level,omitempty"`
	Description       string                     `json:"description"`
	Content           string                     `json:"content,omitempty"`
	OverrideCSS       string                     `json:"override_css,omitempty"`
	CreatedBy         string                     `json:"created_by,omitempty"`
	GovernanceState   treev1.ItemGovernanceState `json:"governance_state,omitempty"`
	LastMutationJobID string                     `json:"last_mutation_job_id,omitempty"`
	CreatedAt         string                     `json:"created_at"`
	ParentID          string                     `json:"parent_id,omitempty"`
	ChildIDs          []string                   `json:"child_ids,omitempty"`
	Scope             treev1.TreeProjectionScope `json:"scope,omitempty"`
}

type ItemSource struct {
	ItemID     string  `json:"item_id"`
	DocumentID string  `json:"document_id"`
	FileID     string  `json:"file_id"`
	ChunkID    string  `json:"chunk_id,omitempty"`
	SourceText string  `json:"source_text,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type ItemEvidence struct {
	Sources []*ItemSource `json:"sources,omitempty"`
}

// TreeItem is the item representation returned by the API.
type TreeItem struct {
	ID          string                     `json:"id"`
	Scope       treev1.TreeProjectionScope `json:"scope"`
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Content     string                     `json:"content,omitempty"`
}

type SynthesizedItem struct {
	LocalID        string   `json:"local_id"`
	Title          string   `json:"title"`
	Level          int      `json:"level"`
	Description    string   `json:"description"`
	Content        string   `json:"content"`
	OverrideCSS    string   `json:"override_css,omitempty"`
	ParentLocalID  string   `json:"parent_local_id"`
	ChildLocalIDs  []string `json:"child_local_ids"`
	SourceChunkIDs []string `json:"source_chunk_ids"`
}

type Chunk struct {
	ChunkIndex int    `json:"chunk_index"`
	Heading    string `json:"heading"`
	Text       string `json:"text"`
}

type SubtreeItem struct {
	Item
	HasChildren bool `json:"has_children"`
}

type TreePath struct {
	ItemIDs  []string `json:"item_ids"`
	HopCount int      `json:"hop_count"`
	Evidence struct {
		SourceDocumentIDs []string `json:"source_document_ids"`
	} `json:"evidence_ref"`
}

type NodeGovernanceState string

package domain

import (
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// Tree represents a hierarchical knowledge tree aggregate within a workspace.
type Tree struct {
	TreeID      string `json:"tree_id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Item represents a node within the hierarchical knowledge tree.
type Item struct {
	ItemID            string                    `json:"id"`
	WorkspaceID       string                    `json:"workspace_id"`
	Title             string                    `json:"title"`
	Level             int                       `json:"level,omitempty"`
	Description       string                    `json:"description"`
	Content           string                    `json:"content,omitempty"`
	OverrideCSS       string                    `json:"override_css,omitempty"`
	CreatedBy         string                    `json:"created_by,omitempty"`
	GovernanceState   appv1.ItemGovernanceState `json:"governance_state,omitempty"`
	LastMutationJobID string                    `json:"last_mutation_job_id,omitempty"`
	CreatedAt         string                    `json:"created_at"`
	ParentID          string                    `json:"parent_id,omitempty"`
	ChildIDs          []string                  `json:"child_ids,omitempty"`
	Scope             appv1.TreeProjectionScope `json:"scope,omitempty"`
	CrossDocument     bool                      `json:"cross_document,omitempty"`
}

// ItemSource connects a knowledge tree item to an ingested document chunk as grounding evidence.
type ItemSource struct {
	ItemID     string  `json:"item_id"`
	DocumentID string  `json:"document_id"`
	FileID     string  `json:"file_id"`
	ChunkID    string  `json:"chunk_id,omitempty"`
	SourceText string  `json:"source_text,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// ItemSourceRef records an external reference or repository source location supporting a tree item.
type ItemSourceRef struct {
	SourceRefID  string  `json:"source_ref_id"`
	ItemID       string  `json:"item_id"`
	Type         string  `json:"type"`
	URL          string  `json:"url,omitempty"`
	Repo         string  `json:"repo,omitempty"`
	Ref          string  `json:"ref,omitempty"`
	Path         string  `json:"path,omitempty"`
	LineStart    int     `json:"line_start,omitempty"`
	LineEnd      int     `json:"line_end,omitempty"`
	Kind         string  `json:"kind,omitempty"`
	ExternalID   string  `json:"external_id,omitempty"`
	Title        string  `json:"title,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
	SnapshotRef  string  `json:"snapshot_ref,omitempty"`
	ContentHash  string  `json:"content_hash,omitempty"`
	MetadataJSON string  `json:"metadata_json,omitempty"`
}

// ItemEvidence aggregates all internal document chunks and external source references supporting an item.
type ItemEvidence struct {
	Sources    []*ItemSource    `json:"sources,omitempty"`
	SourceRefs []*ItemSourceRef `json:"source_refs,omitempty"`
}

// TreeItem is the lightweight item projection returned by read-only tree listing APIs.
type TreeItem struct {
	ID          string                    `json:"id"`
	Scope       appv1.TreeProjectionScope `json:"scope"`
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	Content     string                    `json:"content,omitempty"`
}

// GeneratedTreeItem represents an item candidate synthesized during document analysis prior to tree insertion.
type GeneratedTreeItem struct {
	LocalID           string   `json:"local_id"`
	MergeTargetItemID string   `json:"merge_target_item_id,omitempty"`
	Title             string   `json:"title"`
	Level             int      `json:"level"`
	Description       string   `json:"description"`
	Content           string   `json:"content"`
	OverrideCSS       string   `json:"override_css,omitempty"`
	ParentLocalID     string   `json:"parent_local_id"`
	ChildLocalIDs     []string `json:"child_local_ids"`
	SourceChunkIDs    []string `json:"source_chunk_ids"`
}

// Chunk represents a numbered text block with heading extracted from a source document.
type Chunk struct {
	ChunkIndex int    `json:"chunk_index"`
	Heading    string `json:"heading"`
	Text       string `json:"text"`
}

// SubtreeItem decorates an Item with a boolean indicating if it possesses child nodes.
type SubtreeItem struct {
	Item
	HasChildren bool `json:"has_children"`
}

// TreePath represents an ordered chain of item IDs representing a hierarchical traversal.
type TreePath struct {
	ItemIDs  []string `json:"item_ids"`
	HopCount int      `json:"hop_count"`
	Evidence struct {
		SourceDocumentIDs []string `json:"source_document_ids"`
	} `json:"evidence_ref"`
}

// NodeGovernanceState denotes approval or review status of a tree node.
type NodeGovernanceState string

// ExistingNode is a compact view of a workspace tree node shown to the LLM
// during knowledge-tree generation so it can decide whether to merge a new
// concept into an existing node (by emitting its ID as merge_target_item_id).
type ExistingNode struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

const (
	// GovernanceStateSystemGenerated indicates the node was generated autonomously by AI.
	GovernanceStateSystemGenerated NodeGovernanceState = "system_generated"
	// GovernanceStatePendingReview indicates that the node requires human review.
	GovernanceStatePendingReview NodeGovernanceState = "pending_review"
	// GovernanceStateHumanCurated indicates human edits or verification have been applied.
	GovernanceStateHumanCurated NodeGovernanceState = "human_curated"
	// GovernanceStateLocked protects the node from automatic modification or merging.
	GovernanceStateLocked NodeGovernanceState = "locked"
)

package domain

// DocumentLifecycleState はドキュメント処理ライフサイクルの状態を表す。
type DocumentLifecycleState string

const (
	DocumentLifecycleUploaded             DocumentLifecycleState = "uploaded"
	DocumentLifecyclePendingNormalization DocumentLifecycleState = "pending_normalization"
	DocumentLifecycleProcessing           DocumentLifecycleState = "processing"
	DocumentLifecycleCompleted            DocumentLifecycleState = "completed"
	DocumentLifecycleFailed               DocumentLifecycleState = "failed"
)

// WorkspaceRole はワークスペースのメンバーロールを表す。
type WorkspaceRole string

const (
	WorkspaceRoleOwner  WorkspaceRole = "owner"
	WorkspaceRoleEditor WorkspaceRole = "editor"
	WorkspaceRoleViewer WorkspaceRole = "viewer"
)

type Workspace struct {
	WorkspaceID       string `json:"workspace_id"`
	Name              string `json:"name"`
	OwnerID           string `json:"owner_id"`
	Plan              string `json:"plan"` // "free" | "pro"
	StorageUsedBytes  int64  `json:"storage_used_bytes"`
	StorageQuotaBytes int64  `json:"storage_quota_bytes"`
	MaxFileSizeBytes  int64  `json:"max_file_size_bytes"`
	MaxUploadsPerDay  int64  `json:"max_uploads_per_day"`
	CreatedAt         string `json:"created_at"`
}

type WorkspaceMember struct {
	UserID    string        `json:"user_id"`
	Email     string        `json:"email"`
	Role      WorkspaceRole `json:"role"`
	IsDev     bool          `json:"is_dev"`
	InvitedAt string        `json:"invited_at"`
	InvitedBy string        `json:"invited_by,omitempty"`
}

type Document struct {
	DocumentID      string                 `json:"document_id"`
	WorkspaceID     string                 `json:"workspace_id"`
	UploadedBy      string                 `json:"uploaded_by"`
	Filename        string                 `json:"filename"`
	MimeType        string                 `json:"mime_type"`
	FileSize        int64                  `json:"file_size"`
	Status          DocumentLifecycleState `json:"status"`
	ExtractionDepth string                 `json:"extraction_depth,omitempty"`
	NodeCount       int                    `json:"node_count,omitempty"`
	CurrentStage    string                 `json:"current_stage,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
}

type Node struct {
	NodeID      string `json:"node_id"`
	DocumentID  string `json:"document_id"`
	Label       string `json:"label"`
	Level       int    `json:"level"`
	Category    string `json:"category"`
	EntityType  string `json:"entity_type,omitempty"`
	Description string `json:"description"`
	SummaryHTML string `json:"summary_html,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
	CreatedAt   string `json:"created_at"`
}

type Edge struct {
	EdgeID       string `json:"edge_id"`
	DocumentID   string `json:"document_id"`
	SourceNodeID string `json:"source_node_id"`
	TargetNodeID string `json:"target_node_id"`
	EdgeType     string `json:"edge_type"`
	Description  string `json:"description,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

// GraphNode は API レスポンスで返すノード表現。
type GraphNode struct {
	ID              string `json:"id"`
	CanonicalNodeID string `json:"canonical_node_id,omitempty"`
	Scope           string `json:"scope"` // "document" | "canonical"
	Label           string `json:"label"`
	Level           int    `json:"level"`
	Category        string `json:"category"`
	EntityType      string `json:"entity_type,omitempty"`
	Description     string `json:"description"`
	SummaryHTML     string `json:"summary_html,omitempty"`
}

// GraphEdge は API レスポンスで返すエッジ表現。
type GraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Scope  string `json:"scope"` // "document" | "canonical"
}

type DocumentChunk struct {
	ChunkID    string `json:"chunk_id"`
	DocumentID string `json:"document_id"`
	Heading    string `json:"heading"`
	Text       string `json:"text"`
	SourcePage int    `json:"source_page,omitempty"`
}

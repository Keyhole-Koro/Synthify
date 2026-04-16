package domain

// DocumentLifecycleState represents the document processing lifecycle state.
type DocumentLifecycleState string

const (
	DocumentLifecycleUploaded             DocumentLifecycleState = "uploaded"
	DocumentLifecyclePendingNormalization DocumentLifecycleState = "pending_normalization"
	DocumentLifecycleProcessing           DocumentLifecycleState = "processing"
	DocumentLifecycleCompleted            DocumentLifecycleState = "completed"
	DocumentLifecycleFailed               DocumentLifecycleState = "failed"
)

type Account struct {
	AccountID          string `json:"account_id"`
	Name               string `json:"name"`
	Plan               string `json:"plan"` // "anonymous" | "registered" | "pro"
	StorageQuotaBytes  int64  `json:"storage_quota_bytes"`
	StorageUsedBytes   int64  `json:"storage_used_bytes"`
	MaxFileSizeBytes   int64  `json:"max_file_size_bytes"`
	MaxUploadsPerFiveH int64  `json:"max_uploads_per_5h"`
	MaxUploadsPerWeek  int64  `json:"max_uploads_per_1week"`
	CreatedAt          string `json:"created_at"`
}

type AccountUser struct {
	AccountID string `json:"account_id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
}

type Workspace struct {
	WorkspaceID string `json:"workspace_id"`
	AccountID   string `json:"account_id"`
	Name        string `json:"name"`
	CreatedAt   string `json:"created_at"`
}

type Document struct {
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
	UploadedBy  string `json:"uploaded_by"`
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type"`
	FileSize    int64  `json:"file_size"`
	CreatedAt   string `json:"created_at"`
}

type DocumentProcessingJob struct {
	JobID        string `json:"job_id"`
	DocumentID   string `json:"document_id"`
	GraphID      string `json:"graph_id,omitempty"`
	JobType      string `json:"job_type"`
	Status       string `json:"status"`
	CurrentStage string `json:"current_stage,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	ParamsJSON   string `json:"params_json,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type Graph struct {
	GraphID     string `json:"graph_id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type Node struct {
	NodeID      string `json:"node_id"`
	GraphID     string `json:"graph_id"`
	Label       string `json:"label"`
	Category    string `json:"category,omitempty"`
	Level       int    `json:"level,omitempty"`
	EntityType  string `json:"entity_type,omitempty"`
	Description string `json:"description"`
	SummaryHTML string `json:"summary_html,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
	CreatedAt   string `json:"created_at"`
}

type Edge struct {
	EdgeID       string `json:"edge_id"`
	GraphID      string `json:"graph_id"`
	SourceNodeID string `json:"source_node_id"`
	TargetNodeID string `json:"target_node_id"`
	EdgeType     string `json:"edge_type"`
	Description  string `json:"description,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

type NodeSource struct {
	NodeID     string  `json:"node_id"`
	DocumentID string  `json:"document_id"`
	ChunkID    string  `json:"chunk_id,omitempty"`
	SourceText string  `json:"source_text,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type EdgeSource struct {
	EdgeID     string  `json:"edge_id"`
	DocumentID string  `json:"document_id"`
	ChunkID    string  `json:"chunk_id,omitempty"`
	SourceText string  `json:"source_text,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// GraphNode is the node representation returned by the API.
type GraphNode struct {
	ID              string `json:"id"`
	CanonicalNodeID string `json:"canonical_node_id,omitempty"`
	Scope           string `json:"scope"` // "document" | "canonical"
	Label           string `json:"label"`
	EntityType      string `json:"entity_type,omitempty"`
	Description     string `json:"description"`
	SummaryHTML     string `json:"summary_html,omitempty"`
}

// GraphEdge is the edge representation returned by the API.
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

type Job struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type GraphPath struct {
	NodeIDs  []string `json:"node_ids"`
	HopCount int      `json:"hop_count"`
	Evidence struct {
		SourceDocumentIDs []string `json:"source_document_ids"`
	} `json:"evidence_ref"`
}

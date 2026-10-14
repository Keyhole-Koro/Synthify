package domain

// Document represents an uploaded document metadata record.
type Document struct {
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
	UploadedBy  string `json:"uploaded_by"`
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type"`
	FileSize    int64  `json:"file_size"`
	CreatedAt   string `json:"created_at"`
}

// DocumentFile represents an individual file asset extracted from or associated with a document.
type DocumentFile struct {
	FileID     string `json:"file_id"`
	DocumentID string `json:"document_id"`
	Path       string `json:"path"`
	MimeType   string `json:"mime_type"`
	FileSize   int64  `json:"file_size"`
	CreatedAt  string `json:"created_at"`
}

// DocumentChunk represents a discrete semantic text section and its vector embedding.
type DocumentChunk struct {
	ChunkID    string    `json:"chunk_id"`
	DocumentID string    `json:"document_id"`
	FileID     string    `json:"file_id"`
	Heading    string    `json:"heading"`
	Text       string    `json:"text"`
	SourcePage int       `json:"source_page,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"`
}

// ObjectMetadata encapsulates storage object size and MIME type attributes.
type ObjectMetadata struct {
	Size        int64  `json:"size"`
	ContentType string `json:"content_type,omitempty"`
}

// SourceFile represents a raw document file loaded from storage for worker processing.
type SourceFile struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	DocumentID  string `json:"document_id,omitempty"`
	Filename    string `json:"filename"`
	URI         string `json:"uri"`
	MimeType    string `json:"mime_type"`
	Content     []byte `json:"content,omitempty"`
}

// DocumentBrief summarizes high-level claims, entities, and structural outlines of a document.
type DocumentBrief struct {
	Topic        string   `json:"topic"`
	Level01Hints []string `json:"level01_hints"`
	ClaimSummary string   `json:"claim_summary"`
	Entities     []string `json:"entities"`
	Outline      []string `json:"outline"`
}

// SectionBrief contains candidate items and semantic connections found within a document section.
type SectionBrief struct {
	Heading         string   `json:"heading"`
	Topic           string   `json:"topic"`
	ItemCandidates  []string `json:"item_candidates"`
	ConnectionHints string   `json:"connection_hints"`
}

// DocumentStatus indicates the upload and validation status of a document.
type DocumentStatus string

const (
	// DocumentStatusReserved indicates that an upload reservation has been created.
	DocumentStatusReserved DocumentStatus = "reserved"
	// DocumentStatusConfirmed indicates that file upload has been verified.
	DocumentStatusConfirmed DocumentStatus = "confirmed"
	// DocumentStatusRejected indicates that the file failed validation.
	DocumentStatusRejected DocumentStatus = "rejected"
)

// PlanStatus represents the execution state of an execution plan.
type PlanStatus string

const (
	// PlanStatusPending indicates that a plan is awaiting execution.
	PlanStatusPending PlanStatus = "pending"
	// PlanStatusInProgress indicates that a plan is actively running.
	PlanStatusInProgress PlanStatus = "in_progress"
	// PlanStatusCompleted indicates that all plan steps finished successfully.
	PlanStatusCompleted PlanStatus = "completed"
	// PlanStatusFailed indicates that plan execution encountered an unrecoverable error.
	PlanStatusFailed PlanStatus = "failed"
	// PlanStatusRejected indicates that the plan was explicitly rejected.
	PlanStatusRejected PlanStatus = "rejected"
)


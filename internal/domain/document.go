package domain

// Document represents an uploaded document metadata entity within a workspace.
type Document struct {
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
	UploadedBy  string `json:"uploaded_by"`
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type"`
	FileSize    int64  `json:"file_size"`
	CreatedAt   string `json:"created_at"`
}

// DocumentFile represents an individual file or asset associated with an uploaded document.
type DocumentFile struct {
	FileID     string `json:"file_id"`
	DocumentID string `json:"document_id"`
	Path       string `json:"path"`
	MimeType   string `json:"mime_type"`
	FileSize   int64  `json:"file_size"`
	CreatedAt  string `json:"created_at"`
}

// DocumentFileLocation carries the coordinates needed to stream a file's bytes
// from object storage, resolved from a file_id. WorkspaceID + DocumentID form
// the {ws}/{doc} object prefix; MimeType seeds the delivery Content-Type.
type DocumentFileLocation struct {
	FileID      string
	DocumentID  string
	WorkspaceID string
	Path        string
	MimeType    string
}

// DocumentChunk represents a semantic chunk of extracted text with headings and optional vector embeddings.
type DocumentChunk struct {
	ChunkID    string    `json:"chunk_id"`
	DocumentID string    `json:"document_id"`
	FileID     string    `json:"file_id"`
	Heading    string    `json:"heading"`
	Text       string    `json:"text"`
	SourcePage int       `json:"source_page,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"`
}

// ObjectMetadata describes byte size and MIME type of an object in blob storage.
type ObjectMetadata struct {
	Size        int64  `json:"size"`
	ContentType string `json:"content_type,omitempty"`
}

// ExpiredReservation describes a reservation that ExpireUploadReservations just
// transitioned out of the "reserved" state, so the caller can clean up any
// orphaned GCS object the client might have uploaded before abandoning the flow.
type ExpiredReservation struct {
	DocumentID  string
	WorkspaceID string
}

// SourceFile represents an input file with binary content or URI reference ready for ingestion.
type SourceFile struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	DocumentID  string `json:"document_id,omitempty"`
	Filename    string `json:"filename"`
	URI         string `json:"uri"`
	MimeType    string `json:"mime_type"`
	Content     []byte `json:"content,omitempty"`
}

// DocumentBrief contains top-level topical synthesis, key claim summaries, and entity outlines.
type DocumentBrief struct {
	Topic        string   `json:"topic"`
	Level01Hints []string `json:"level01_hints"`
	ClaimSummary string   `json:"claim_summary"`
	Entities     []string `json:"entities"`
	Outline      []string `json:"outline"`
}

// SectionBrief contains section-specific topic models, entity candidates, and relationship connections.
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

// PlanStatus is the approval state of a job's execution plan. The same values
// are stored in job_execution_plans.status and processing_jobs.plan_status.
type PlanStatus string

const (
	// PlanStatusNone is the processing_jobs default before any plan exists.
	PlanStatusNone PlanStatus = "none"
	// PlanStatusPendingApproval indicates that the plan waits for a reviewer.
	PlanStatusPendingApproval PlanStatus = "pending_approval"
	// PlanStatusApproved indicates that the plan may run.
	PlanStatusApproved PlanStatus = "approved"
	// PlanStatusRejected indicates that the plan was explicitly rejected.
	PlanStatusRejected PlanStatus = "rejected"
)

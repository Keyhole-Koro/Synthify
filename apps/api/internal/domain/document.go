package domain

type Document struct {
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
	UploadedBy  string `json:"uploaded_by"`
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type"`
	FileSize    int64  `json:"file_size"`
	CreatedAt   string `json:"created_at"`
}

type DocumentFile struct {
	FileID     string `json:"file_id"`
	DocumentID string `json:"document_id"`
	Path       string `json:"path"`
	MimeType   string `json:"mime_type"`
	FileSize   int64  `json:"file_size"`
	CreatedAt  string `json:"created_at"`
}

type DocumentChunk struct {
	ChunkID    string    `json:"chunk_id"`
	DocumentID string    `json:"document_id"`
	FileID     string    `json:"file_id"`
	Heading    string    `json:"heading"`
	Text       string    `json:"text"`
	SourcePage int       `json:"source_page,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"`
}

type ObjectMetadata struct {
	Size        int64  `json:"size"`
	ContentType string `json:"content_type,omitempty"`
}

type SourceFile struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	DocumentID  string `json:"document_id,omitempty"`
	Filename    string `json:"filename"`
	URI         string `json:"uri"`
	MimeType    string `json:"mime_type"`
	Content     []byte `json:"content,omitempty"`
}

type DocumentBrief struct {
	Topic        string   `json:"topic"`
	Level01Hints []string `json:"level01_hints"`
	ClaimSummary string   `json:"claim_summary"`
	Entities     []string `json:"entities"`
	Outline      []string `json:"outline"`
}

type SectionBrief struct {
	Heading         string   `json:"heading"`
	Topic           string   `json:"topic"`
	ItemCandidates  []string `json:"item_candidates"`
	ConnectionHints string   `json:"connection_hints"`
}

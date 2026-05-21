package domain

type JobStageCheckpoint struct {
	JobID     string `json:"job_id"`
	Stage     string `json:"stage"`
	Status    string `json:"status"` // "running" | "succeeded" | "failed"
	GCSRef    string `json:"gcs_ref"`
	UpdatedAt string `json:"updated_at"`
}

type CheckpointEnvelope struct {
	SchemaVersion int            `json:"schema_version"`
	Kind          string         `json:"kind"` // "synthify.worker_checkpoint"
	Stage         string         `json:"stage"`
	JobID         string         `json:"job_id"`
	DocumentID    string         `json:"document_id"`
	WorkspaceID   string         `json:"workspace_id"`
	CreatedAt     string         `json:"created_at"`
	Inputs        map[string]any `json:"inputs"`
	Outputs       map[string]any `json:"outputs"`
	Stats         map[string]any `json:"stats"`
}

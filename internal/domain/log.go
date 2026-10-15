package domain

import (
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// JobLog represents an immutable structured log record emitted during job execution.
type JobLog struct {
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"`
	Event       string `json:"event"`
	Message     string `json:"message"`
	DetailJSON  string `json:"detail_json"`
	Source      string `json:"source"`
	SourceID    string `json:"source_id"`
	JobID       string `json:"job_id"`
	DocumentID  string `json:"document_id"`
	WorkspaceID string `json:"workspace_id"`
}

// JobLogSearchFilter defines search and pagination parameters for querying job execution logs.
type JobLogSearchFilter struct {
	Query         string
	WorkspaceID   string
	DocumentID    string
	JobID         string
	Levels        []string
	Events        []string
	FromTimestamp string
	ToTimestamp   string
	PageToken     string
	Limit         int
}

// RelatedLogScope specifies the grouping dimension for correlated job log searches.
type RelatedLogScope string

const (
	// RelatedLogScopeJob confines related log correlation to a single job execution.
	RelatedLogScopeJob RelatedLogScope = "job"
	// RelatedLogScopeDocument correlates logs across all jobs for a specific document.
	RelatedLogScopeDocument RelatedLogScope = "document"
	// RelatedLogScopeWorkspace correlates logs across all jobs in a workspace.
	RelatedLogScopeWorkspace RelatedLogScope = "workspace"
)

// JobLogJob aggregates logs for a single job run alongside its lifecycle status.
type JobLogJob struct {
	JobID     string                  `json:"job_id"`
	Status    appv1.JobLifecycleState `json:"status"`
	CreatedAt string                  `json:"created_at"`
	Logs      []*JobLog               `json:"logs"`
}

// JobLogGroup groups related jobs and their execution logs by workspace and document.
type JobLogGroup struct {
	WorkspaceID string       `json:"workspace_id"`
	DocumentID  string       `json:"document_id"`
	Jobs        []*JobLogJob `json:"jobs"`
}

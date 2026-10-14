package domain

import (
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// JobLog represents a structured observability log event emitted during worker job execution.
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

// JobLogSearchFilter specifies search and filtering criteria for structured job log queries.
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

// RelatedLogScope determines the grouping boundary when retrieving associated job logs.
type RelatedLogScope string

const (
	// RelatedLogScopeJob queries logs scoped to a single job execution.
	RelatedLogScopeJob RelatedLogScope = "job"
	// RelatedLogScopeDocument queries logs across all jobs for a specific document.
	RelatedLogScopeDocument RelatedLogScope = "document"
	// RelatedLogScopeWorkspace queries logs across all jobs within a workspace.
	RelatedLogScopeWorkspace RelatedLogScope = "workspace"
)

// JobLogJob aggregates logs and status metadata for a specific job run.
type JobLogJob struct {
	JobID     string                  `json:"job_id"`
	Status    appv1.JobLifecycleState `json:"status"`
	CreatedAt string                  `json:"created_at"`
	Logs      []*JobLog               `json:"logs"`
}

// JobLogGroup groups multiple job log runs by workspace and document.
type JobLogGroup struct {
	WorkspaceID string       `json:"workspace_id"`
	DocumentID  string       `json:"document_id"`
	Jobs        []*JobLogJob `json:"jobs"`
}

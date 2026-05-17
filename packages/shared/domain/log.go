package domain

import (
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

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

type RelatedLogScope string

const (
	RelatedLogScopeJob       RelatedLogScope = "job"
	RelatedLogScopeDocument  RelatedLogScope = "document"
	RelatedLogScopeWorkspace RelatedLogScope = "workspace"
)

type JobLogJob struct {
	JobID     string                   `json:"job_id"`
	Status    treev1.JobLifecycleState `json:"status"`
	CreatedAt string                   `json:"created_at"`
	Logs      []*JobLog                `json:"logs"`
}

type JobLogGroup struct {
	WorkspaceID string       `json:"workspace_id"`
	DocumentID  string       `json:"document_id"`
	Jobs        []*JobLogJob `json:"jobs"`
}

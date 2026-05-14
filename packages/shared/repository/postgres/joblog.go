package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
	"github.com/synthify/backend/packages/shared/job/log"
)

type DBLogger struct {
	repo joblog.Repository
}

func NewDBLogger(repo joblog.Repository) *DBLogger {
	return &DBLogger{repo: repo}
}

func (l *DBLogger) Log(ctx context.Context, e joblog.Event) {
	_ = l.repo.LogJobEvent(ctx, e)
}

func (s *Store) LogJobEvent(ctx context.Context, e joblog.Event) error {
	detailJSON, err := json.Marshal(e.Detail)
	if err != nil {
		detailJSON = []byte("{}")
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO job_logs (id, job_id, workspace_id, document_id, created_at, level, event, message, detail_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		newID(), e.JobID, e.WorkspaceID, e.DocumentID, nowTime(),
		string(e.Level), e.Event, e.Message, string(detailJSON))
	return err
}

func (s *Store) ListJobLogs(ctx context.Context, jobID string, pageToken string, limit int) ([]*domain.JobLog, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}

	where := "job_id = $1"
	args := []any{jobID}
	if pageToken != "" {
		cursor, err := decodeJobLogCursor(pageToken)
		if err != nil {
			return nil, "", err
		}
		args = append(args, cursor.Timestamp, cursor.SourceID)
		where += fmt.Sprintf(" AND (timestamp, source_id) < ($%d, $%d)", len(args)-1, len(args))
	}

	args = append(args, limit+1)
	query := fmt.Sprintf(`
SELECT timestamp, level, event, message, detail_json, source, source_id, job_id, document_id, workspace_id
FROM (%s) logs
WHERE %s
ORDER BY timestamp DESC, source_id DESC
LIMIT $%d`, unifiedJobLogsSourceSQL(), where, len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	logRows, err := scanJobLogRows(rows)
	if err != nil {
		return nil, "", err
	}
	logs, nextPageToken := paginateLogRows(logRows, limit)
	return logs, nextPageToken, nil
}

func (s *Store) SearchJobLogs(ctx context.Context, filter domain.JobLogSearchFilter) ([]*domain.JobLog, string, error) {
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 200
	}

	var where []string
	var args []any
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if filter.WorkspaceID != "" {
		add("workspace_id = $%d", filter.WorkspaceID)
	}
	if filter.DocumentID != "" {
		add("document_id = $%d", filter.DocumentID)
	}
	if filter.JobID != "" {
		add("job_id = $%d", filter.JobID)
	}
	if filter.Query != "" {
		add("LOWER(search_text) LIKE LOWER($%d)", "%"+filter.Query+"%")
	}
	if filter.FromTimestamp != "" {
		add("timestamp >= $%d::timestamptz", filter.FromTimestamp)
	}
	if filter.ToTimestamp != "" {
		add("timestamp <= $%d::timestamptz", filter.ToTimestamp)
	}
	if len(filter.Levels) > 0 {
		where = append(where, inCondition("level", &args, filter.Levels))
	}
	if len(filter.Events) > 0 {
		where = append(where, inCondition("event", &args, filter.Events))
	}
	if len(where) == 0 {
		return nil, "", fmt.Errorf("at least one search scope is required")
	}

	if filter.PageToken != "" {
		cursor, err := decodeJobLogCursor(filter.PageToken)
		if err != nil {
			return nil, "", err
		}
		args = append(args, cursor.Timestamp, cursor.SourceID)
		where = append(where, fmt.Sprintf("(timestamp, source_id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	args = append(args, filter.Limit+1)
	query := fmt.Sprintf(`
SELECT timestamp, level, event, message, detail_json, source, source_id, job_id, document_id, workspace_id
FROM (%s) logs
WHERE %s
ORDER BY timestamp DESC, source_id DESC
LIMIT $%d`, unifiedJobLogsSourceSQL(), strings.Join(where, " AND "), len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	logRows, err := scanJobLogRows(rows)
	if err != nil {
		return nil, "", err
	}
	logs, nextPageToken := paginateLogRows(logRows, filter.Limit)
	return logs, nextPageToken, nil
}

func (s *Store) ListRelatedJobLogs(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string, pageToken string, limit int) ([]*domain.JobLogGroup, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	offset := 0
	if pageToken != "" {
		fmt.Sscanf(pageToken, "%d", &offset)
	}

	var rows *sql.Rows
	var err error
	switch scope {
	case domain.RelatedLogScopeJob:
		rows, err = s.db.QueryContext(ctx, relatedJobsSQL("j.document_id = (SELECT document_id FROM document_processing_jobs WHERE job_id = $1)", limit, offset), jobID)
	case domain.RelatedLogScopeDocument:
		rows, err = s.db.QueryContext(ctx, relatedJobsSQL("j.document_id = $1", limit, offset), documentID)
	case domain.RelatedLogScopeWorkspace:
		rows, err = s.db.QueryContext(ctx, relatedJobsSQL("j.workspace_id = $1", limit, offset), workspaceID)
	default:
		return nil, "", fmt.Errorf("unsupported related log scope: %s", scope)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	groupsByDocument := map[string]*domain.JobLogGroup{}
	var groups []*domain.JobLogGroup
	var count int
	for rows.Next() {
		count++
		var jobID, docID, wsID, status string
		var createdAt time.Time
		if err := rows.Scan(&jobID, &docID, &wsID, &status, &createdAt); err != nil {
			return nil, "", err
		}
		group := groupsByDocument[docID]
		if group == nil {
			group = &domain.JobLogGroup{WorkspaceID: wsID, DocumentID: docID}
			groupsByDocument[docID] = group
			groups = append(groups, group)
		}
		logs, _, _ := s.ListJobLogs(ctx, jobID, "", 50) // load some logs for each related job
		group.Jobs = append(group.Jobs, &domain.JobLogJob{
			JobID:     jobID,
			Status:    parseJobStatus(status),
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
			Logs:      logs,
		})
	}
	nextPageToken := ""
	if count == limit {
		nextPageToken = fmt.Sprintf("%d", offset+limit)
	}
	return groups, nextPageToken, rows.Err()
}

func unifiedJobLogsSourceSQL() string {
	return `
SELECT
  l.created_at AS timestamp,
  l.level,
  l.event,
  l.message,
  l.detail_json::text AS detail_json,
  'system' AS source,
  l.id AS source_id,
  l.job_id,
  l.document_id,
  l.workspace_id,
  LOWER(l.event || ' ' || l.message || ' ' || l.detail_json::text) AS search_text
FROM job_logs l
UNION ALL
SELECT
  m.created_at AS timestamp,
  CASE
    WHEN LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"success":false%'
      OR LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"status":"error"%'
      OR LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"status":"failed"%'
      OR LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"error":%'
      OR LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"errors":%'
    THEN 'ERROR'
    ELSE 'INFO'
  END AS level,
  CASE
    WHEN LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"success":false%'
      OR LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"status":"error"%'
      OR LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"status":"failed"%'
      OR LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"error":%'
      OR LOWER(REPLACE(REPLACE(REPLACE(m.after_json, ' ', ''), E'\n', ''), E'\t', '')) LIKE '%"errors":%'
    THEN 'tool.call.failed'
    ELSE 'tool.call.completed'
  END AS event,
  m.target_id AS message,
  jsonb_build_object(
    'tool', m.target_id,
    'target_type', m.target_type,
    'mutation_type', m.mutation_type,
    'risk_tier', m.risk_tier,
    'input', m.before_json,
    'output', m.after_json,
    'provenance', m.provenance_json
  )::text AS detail_json,
  'tool' AS source,
  m.mutation_id AS source_id,
  m.job_id,
  j.document_id,
  m.workspace_id,
  LOWER(m.target_id || ' ' || m.target_type || ' ' || m.mutation_type || ' ' || m.before_json || ' ' || m.after_json || ' ' || m.provenance_json) AS search_text
FROM job_mutation_logs m
INNER JOIN document_processing_jobs j ON j.job_id = m.job_id`
}

func relatedJobsSQL(where string, limit, offset int) string {
	return fmt.Sprintf(`
SELECT j.job_id, j.document_id, j.workspace_id, j.status, j.created_at
FROM document_processing_jobs j
WHERE %s
ORDER BY j.created_at DESC
LIMIT %d OFFSET %d`, where, limit, offset)
}

func inCondition(column string, args *[]any, values []string) string {
	var placeholders []string
	for _, value := range values {
		*args = append(*args, value)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(*args)))
	}
	return fmt.Sprintf("%s IN (%s)", column, joinStrings(placeholders, ", "))
}

type jobLogCursor struct {
	Timestamp time.Time
	SourceID  string
}

type jobLogRow struct {
	Timestamp   time.Time
	Level       string
	Event       string
	Message     string
	DetailJSON  string
	Source      string
	SourceID    string
	JobID       string
	DocumentID  string
	WorkspaceID string
}

func parseJobStatus(s string) treev1.JobLifecycleState {
	status, _ := strconv.Atoi(s)
	return treev1.JobLifecycleState(status)
}

func encodeJobLogCursor(ts time.Time, sourceID string) string {
	return ts.UTC().Format(time.RFC3339Nano) + "|" + sourceID
}

func decodeJobLogCursor(token string) (jobLogCursor, error) {
	parts := strings.SplitN(token, "|", 2)
	if len(parts) != 2 {
		return jobLogCursor{}, fmt.Errorf("invalid job log cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return jobLogCursor{}, fmt.Errorf("invalid job log cursor timestamp: %w", err)
	}
	return jobLogCursor{Timestamp: ts, SourceID: parts[1]}, nil
}

func paginateLogRows(rows []jobLogRow, limit int) ([]*domain.JobLog, string) {
	nextPageToken := ""
	if len(rows) > limit {
		rows = rows[:limit]
		oldest := rows[len(rows)-1]
		nextPageToken = encodeJobLogCursor(oldest.Timestamp, oldest.SourceID)
	}

	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
	return toDomainJobLogs(rows), nextPageToken
}

func joinStrings(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += sep + value
	}
	return out
}

func scanJobLogRows(rows *sql.Rows) ([]jobLogRow, error) {
	var logs []jobLogRow
	for rows.Next() {
		log := jobLogRow{}
		if err := rows.Scan(&log.Timestamp, &log.Level, &log.Event, &log.Message, &log.DetailJSON, &log.Source, &log.SourceID, &log.JobID, &log.DocumentID, &log.WorkspaceID); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func toDomainJobLogs(rows []jobLogRow) []*domain.JobLog {
	var logs []*domain.JobLog
	for _, row := range rows {
		log := &domain.JobLog{
			Timestamp:   row.Timestamp.UTC().Format(time.RFC3339Nano),
			Level:       row.Level,
			Event:       row.Event,
			Message:     row.Message,
			DetailJSON:  row.DetailJSON,
			Source:      row.Source,
			SourceID:    row.SourceID,
			JobID:       row.JobID,
			DocumentID:  row.DocumentID,
			WorkspaceID: row.WorkspaceID,
		}
		logs = append(logs, log)
	}
	return logs
}

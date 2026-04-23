-- name: ListDocuments :many
SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at
FROM documents
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: CountSameDocumentItems :one
SELECT COUNT(DISTINCT item_sources.item_id)
FROM item_sources
INNER JOIN tree_items ON tree_items.id = item_sources.item_id
WHERE item_sources.document_id = $1
  AND tree_items.workspace_id = $2;

-- name: CountApprovedAliases :one
SELECT COUNT(*)
FROM item_aliases
INNER JOIN tree_items canonical_items ON canonical_items.id = item_aliases.canonical_item_id
INNER JOIN tree_items alias_items ON alias_items.id = item_aliases.alias_item_id
WHERE item_aliases.workspace_id = $1
  AND item_aliases.status = 'approved'
  AND canonical_items.workspace_id = $2
  AND alias_items.workspace_id = $2;

-- name: CountProtectedAliases :one
SELECT COUNT(*)
FROM item_aliases
INNER JOIN tree_items canonical_items ON canonical_items.id = item_aliases.canonical_item_id
INNER JOIN tree_items alias_items ON alias_items.id = item_aliases.alias_item_id
WHERE item_aliases.workspace_id = $1
  AND item_aliases.status = 'approved'
  AND canonical_items.workspace_id = $2
  AND alias_items.workspace_id = $2
  AND (
    canonical_items.governance_state IN ('human_curated', 'locked')
    OR alias_items.governance_state IN ('human_curated', 'locked')
  );

-- name: GetDocument :one
SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at
FROM documents
WHERE document_id = $1;

-- name: CreateDocument :exec
INSERT INTO documents (document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetLatestProcessingJob :one
SELECT job_id, document_id, workspace_id, job_type, status, current_stage, error_message, params_json,
       requested_by, capability_id, execution_plan_id, plan_status, evaluation_status, retry_count, budget_json,
       created_at, updated_at
FROM document_processing_jobs
WHERE document_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetProcessingJob :one
SELECT job_id, document_id, workspace_id, job_type, status, current_stage, error_message, params_json,
       requested_by, capability_id, execution_plan_id, plan_status, evaluation_status, retry_count, budget_json,
       created_at, updated_at
FROM document_processing_jobs
WHERE job_id = $1;

-- name: CreateProcessingJob :exec
INSERT INTO document_processing_jobs (
  job_id, document_id, workspace_id, job_type, status, current_stage, error_message, params_json,
  requested_by, capability_id, execution_plan_id, plan_status, evaluation_status, retry_count, budget_json,
  created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $16);

-- name: MarkProcessingJobRunning :execrows
UPDATE document_processing_jobs
SET status = 'running',
    error_message = '',
    plan_status = CASE WHEN plan_status IN ('', 'approved', 'none') THEN 'executing' ELSE plan_status END,
    updated_at = $2
WHERE job_id = $1;

-- name: CompleteProcessingJob :execrows
UPDATE document_processing_jobs
SET status = 'completed',
    current_stage = '',
    plan_status = 'completed',
    evaluation_status = 'passed',
    updated_at = $2
WHERE job_id = $1;

-- name: FailProcessingJob :execrows
UPDATE document_processing_jobs
SET status = 'failed',
    error_message = $2,
    evaluation_status = 'failed',
    updated_at = $3
WHERE job_id = $1;

-- name: UpdateProcessingJobStage :exec
UPDATE document_processing_jobs
SET current_stage = $2, updated_at = $3
WHERE job_id = $1;

-- name: ListDocumentChunks :many
SELECT chunk_id, document_id, heading, text, source_page
FROM document_chunks
WHERE document_id = $1
ORDER BY chunk_id;

-- name: DeleteDocumentChunks :exec
DELETE FROM document_chunks
WHERE document_id = $1;

-- name: CreateDocumentChunk :exec
INSERT INTO document_chunks (chunk_id, document_id, heading, text, source_page)
VALUES ($1, $2, $3, $4, $5);

-- name: GetJobCapability :one
SELECT capability_id, job_id, workspace_id, allowed_document_ids_json, allowed_item_ids_json,
       allowed_operations_json, max_llm_calls, max_tool_runs, max_item_creations, expires_at, created_at
FROM job_capabilities
WHERE job_id = $1;

-- name: CreateJobCapability :exec
INSERT INTO job_capabilities (
  capability_id, job_id, workspace_id, allowed_document_ids_json, allowed_item_ids_json,
  allowed_operations_json, max_llm_calls, max_tool_runs, max_item_creations, expires_at, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetJobExecutionPlan :one
SELECT plan_id, job_id, status, summary, plan_json, created_by, created_at, updated_at
FROM job_execution_plans
WHERE job_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: UpsertJobExecutionPlan :exec
INSERT INTO job_execution_plans (plan_id, job_id, status, summary, plan_json, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (plan_id) DO UPDATE
SET status = EXCLUDED.status,
    summary = EXCLUDED.summary,
    plan_json = EXCLUDED.plan_json,
    created_by = EXCLUDED.created_by,
    updated_at = EXCLUDED.updated_at;

-- name: UpdateProcessingJobPlanState :exec
UPDATE document_processing_jobs
SET execution_plan_id = $2,
    plan_status = $3,
    updated_at = $4
WHERE job_id = $1;

-- name: CountJobMutationLogs :one
SELECT COUNT(*)
FROM job_mutation_logs
WHERE job_id = $1;

-- name: ListJobApprovalRequests :many
SELECT approval_id, job_id, plan_id, status, requested_operations_json, reason, risk_tier,
       requested_by, reviewed_by, requested_at, reviewed_at
FROM job_approval_requests
WHERE job_id = $1
ORDER BY requested_at DESC;

-- name: CreateJobApprovalRequest :exec
INSERT INTO job_approval_requests (
  approval_id, job_id, plan_id, status, requested_operations_json, reason, risk_tier,
  requested_by, reviewed_by, requested_at, reviewed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetJobApprovalPlanID :one
SELECT plan_id
FROM job_approval_requests
WHERE job_id = $1 AND approval_id = $2;

-- name: UpdateJobExecutionPlanStatus :execrows
UPDATE job_execution_plans
SET status = $2, updated_at = $3
WHERE plan_id = $1;

-- name: ApproveJobApproval :execrows
UPDATE job_approval_requests
SET status = 'approved', reviewed_by = $3, reviewed_at = $4
WHERE job_id = $1 AND approval_id = $2;

-- name: RejectJobApproval :execrows
UPDATE job_approval_requests
SET status = 'rejected',
    reviewed_by = $3,
    reviewed_at = $4,
    reason = CASE WHEN $5 = '' THEN reason ELSE $5 END
WHERE job_id = $1 AND approval_id = $2;

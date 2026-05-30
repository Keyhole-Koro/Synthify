package jobstatus

import appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"

// document_processing_jobs.status / job_type are stored as short lowercase
// strings (e.g. "queued", "process_document"), matching the SQL literals in
// db/queries/documents.sql and the Firestore status strings. Historically the
// api/worker repositories serialized the proto enums as their numeric value
// (strconv.Itoa), which never matched the SQL WHERE clauses (e.g.
// MarkProcessingJobRunning's `status IN ('queued','running')`) and left jobs
// stuck in queued. These helpers are the single source of truth for the
// on-disk encoding so both services agree.

var lifecycleStateToDB = map[appv1.JobLifecycleState]string{
	appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED:    "queued",
	appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING:   "running",
	appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED: "succeeded",
	appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED:    "failed",
}

var dbToLifecycleState = reverseStateMap(lifecycleStateToDB)

var jobTypeToDB = map[appv1.JobType]string{
	appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT:   "process_document",
	appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT: "reprocess_document",
}

var dbToJobType = reverseTypeMap(jobTypeToDB)

// LifecycleStateToDB encodes a lifecycle state for the status column. Unknown
// states fall back to the empty string rather than a bogus numeric value.
func LifecycleStateToDB(s appv1.JobLifecycleState) string {
	return lifecycleStateToDB[s]
}

// LifecycleStateFromDB decodes a status column value. Unknown / legacy values
// decode to UNSPECIFIED.
func LifecycleStateFromDB(s string) appv1.JobLifecycleState {
	return dbToLifecycleState[s]
}

// JobTypeToDB encodes a job type for the job_type column.
func JobTypeToDB(t appv1.JobType) string {
	return jobTypeToDB[t]
}

// JobTypeFromDB decodes a job_type column value.
func JobTypeFromDB(s string) appv1.JobType {
	return dbToJobType[s]
}

func reverseStateMap(m map[appv1.JobLifecycleState]string) map[string]appv1.JobLifecycleState {
	out := make(map[string]appv1.JobLifecycleState, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

func reverseTypeMap(m map[appv1.JobType]string) map[string]appv1.JobType {
	out := make(map[string]appv1.JobType, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

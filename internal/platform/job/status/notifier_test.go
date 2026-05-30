package jobstatus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateFieldsMapWithUpdatedAt_IncludesOnlySetFields(t *testing.T) {
	progress := 42
	fields := UpdateFields{
		Progress:                     &progress,
		Message:                      String("Generating brief"),
		SuggestedWorkspaceName:       String("Clinical Trial Protocol"),
		SuggestedWorkspaceNameSource: String("brief.topic"),
	}

	got := fields.mapWithUpdatedAt()

	if got["updatedAt"] == "" {
		t.Fatalf("updatedAt is empty")
	}
	if got["progress"] != 42 {
		t.Fatalf("progress = %v, want 42", got["progress"])
	}
	if got["message"] != "Generating brief" {
		t.Fatalf("message = %v, want Generating brief", got["message"])
	}
	if got["suggestedWorkspaceName"] != "Clinical Trial Protocol" {
		t.Fatalf("suggestedWorkspaceName = %v", got["suggestedWorkspaceName"])
	}
	if got["suggestedWorkspaceNameSource"] != "brief.topic" {
		t.Fatalf("suggestedWorkspaceNameSource = %v", got["suggestedWorkspaceNameSource"])
	}
	if _, ok := got["currentStage"]; ok {
		t.Fatalf("currentStage should be omitted when unset")
	}
}

func TestUpdateFieldsMapWithUpdatedAt_EmptyUpdateOnlyHasUpdatedAt(t *testing.T) {
	got := (UpdateFields{}).mapWithUpdatedAt()

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(got), got)
	}
	if got["updatedAt"] == "" {
		t.Fatalf("updatedAt is empty")
	}
}

func TestUpdateFieldsMapKeysExistInJSONSchema(t *testing.T) {
	progress := 1
	fields := UpdateFields{
		CurrentStage:                 String("briefing"),
		Progress:                     &progress,
		Message:                      String("Working"),
		ErrorMessage:                 String(""),
		SuggestedWorkspaceName:       String("Clinical Trial Protocol"),
		SuggestedWorkspaceNameSource: String("brief.topic"),
		StartedAt:                    String("2026-05-26T00:00:00Z"),
		CompletedAt:                  String("2026-05-26T00:01:00Z"),
	}

	schemaPath := filepath.Join("..", "..", "..", "..", "contracts", "firestore", "job-status.schema.json")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	for key := range fields.mapWithUpdatedAt() {
		if _, ok := schema.Properties[key]; !ok {
			t.Fatalf("field %q is emitted by UpdateFields but missing from JSON schema", key)
		}
	}
}

func TestValidateFirestoreJobStatus_AcceptsFullDocument(t *testing.T) {
	doc := map[string]any{
		FirestoreJobStatusFieldJobID:                        "job_1",
		FirestoreJobStatusFieldJobType:                      "JOB_TYPE_PROCESS_DOCUMENT",
		FirestoreJobStatusFieldDocumentID:                   "doc_1",
		FirestoreJobStatusFieldWorkspaceID:                  "ws_1",
		FirestoreJobStatusFieldTreeID:                       "ws_1",
		FirestoreJobStatusFieldStatus:                       string(FirestoreJobStatusStateRunning),
		FirestoreJobStatusFieldCurrentStage:                 "briefing",
		FirestoreJobStatusFieldProgress:                     30,
		FirestoreJobStatusFieldMessage:                      "Generating brief",
		FirestoreJobStatusFieldErrorMessage:                 "",
		FirestoreJobStatusFieldSuggestedWorkspaceName:       "Clinical Trial Protocol",
		FirestoreJobStatusFieldSuggestedWorkspaceNameSource: string(FirestoreJobStatusSuggestedWorkspaceNameSourceBriefTopic),
		FirestoreJobStatusFieldUpdatedAt:                    "2026-05-26T00:00:00Z",
	}

	if err := validateFirestoreJobStatus(doc); err != nil {
		t.Fatalf("validateFirestoreJobStatus() error = %v", err)
	}
}

// Failed/Completed write expiresAt as a native time.Time so the Firestore TTL
// policy can match it. The JSON-schema validator cannot introspect a Go struct
// and used to reject the whole document ("cannot validate against a struct"),
// which silently dropped the terminal status from Firestore.
func TestValidateFirestoreJobStatus_AcceptsTimeExpiresAt(t *testing.T) {
	doc := map[string]any{
		FirestoreJobStatusFieldJobID:        "job_1",
		FirestoreJobStatusFieldJobType:      "JOB_TYPE_PROCESS_DOCUMENT",
		FirestoreJobStatusFieldDocumentID:   "doc_1",
		FirestoreJobStatusFieldWorkspaceID:  "ws_1",
		FirestoreJobStatusFieldTreeID:       "ws_1",
		FirestoreJobStatusFieldStatus:       string(FirestoreJobStatusStateFailed),
		FirestoreJobStatusFieldCurrentStage: "",
		FirestoreJobStatusFieldMessage:      "Failed",
		FirestoreJobStatusFieldErrorMessage: "boom",
		FirestoreJobStatusFieldUpdatedAt:    nowRFC3339(),
		FirestoreJobStatusFieldCompletedAt:  nowRFC3339(),
		FirestoreJobStatusFieldExpiresAt:    ttlFromNow(),
	}

	if err := validateFirestoreJobStatus(doc); err != nil {
		t.Fatalf("validateFirestoreJobStatus() error = %v", err)
	}
}

func TestValidateFirestoreJobStatus_RejectsInvalidStatus(t *testing.T) {
	doc := map[string]any{
		FirestoreJobStatusFieldJobID:        "job_1",
		FirestoreJobStatusFieldJobType:      "JOB_TYPE_PROCESS_DOCUMENT",
		FirestoreJobStatusFieldDocumentID:   "doc_1",
		FirestoreJobStatusFieldWorkspaceID:  "ws_1",
		FirestoreJobStatusFieldTreeID:       "ws_1",
		FirestoreJobStatusFieldStatus:       "done",
		FirestoreJobStatusFieldCurrentStage: "",
		FirestoreJobStatusFieldErrorMessage: "",
		FirestoreJobStatusFieldUpdatedAt:    "2026-05-26T00:00:00Z",
	}

	if err := validateFirestoreJobStatus(doc); err == nil {
		t.Fatalf("validateFirestoreJobStatus() error = nil, want validation error")
	}
}

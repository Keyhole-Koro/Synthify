package mock

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
	"github.com/synthify/backend/packages/shared/job/log"
	"github.com/synthify/backend/packages/shared/repository"
)

type Store struct {
	mu            sync.RWMutex
	accounts      map[string]*domain.Account
	workspaces    map[string]*domain.Workspace
	wsOwners      map[string]string // wsID -> ownerAccountID
	documents     map[string]*domain.Document
	docFiles      map[string]map[string]*domain.DocumentFile // docID -> fileID -> File
	jobs          map[string]*domain.DocumentProcessingJob
	capabilities  map[string]*domain.JobCapability
	plans         map[string]*domain.JobExecutionPlan
	approvals     map[string][]*domain.JobApprovalRequest
	items         map[string]map[string]*domain.Item // workspaceID -> itemID -> Item
	sources       map[string][]*domain.ItemSource
	chunks        map[string][]*domain.DocumentChunk
	checkpoints   map[string]map[string]domain.JobStageCheckpoint // jobID -> stage -> checkpoint
	reservations  map[string]*uploadReservation
	billingEvents map[string]string
}

type uploadReservation struct {
	AccountID    string
	WorkspaceID  string
	DocumentID   string
	ExpectedSize int64
	ActualSize   int64
	Status       string
	ExpiresAt    time.Time
}

func NewStore() *Store {
	return &Store{
		accounts:      make(map[string]*domain.Account),
		workspaces:    make(map[string]*domain.Workspace),
		wsOwners:      make(map[string]string),
		documents:     make(map[string]*domain.Document),
		docFiles:      make(map[string]map[string]*domain.DocumentFile),
		jobs:          make(map[string]*domain.DocumentProcessingJob),
		capabilities:  make(map[string]*domain.JobCapability),
		plans:         make(map[string]*domain.JobExecutionPlan),
		approvals:     make(map[string][]*domain.JobApprovalRequest),
		items:         make(map[string]map[string]*domain.Item),
		sources:       make(map[string][]*domain.ItemSource),
		chunks:        make(map[string][]*domain.DocumentChunk),
		checkpoints:   make(map[string]map[string]domain.JobStageCheckpoint),
		reservations:  make(map[string]*uploadReservation),
		billingEvents: make(map[string]string),
	}
}

// AccountRepository
func (s *Store) GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.accounts[userID]; ok {
		return a, nil
	}
	a := &domain.Account{
		AccountID:            userID,
		Name:                 "User " + userID,
		Plan:                 "free",
		StorageQuotaBytes:    5 * 1 << 30,
		MaxFileSizeBytes:     100 << 20,
		MaxUploadsPerFiveH:   20,
		MaxUploadsPerWeek:    100,
		StripeCustomerID:     "",
		StripeSubscriptionID: "",
		BillingStatus:        string(domain.BillingStatusFree),
		CreatedAt:            time.Now().Format(time.RFC3339),
	}
	s.accounts[userID] = a
	return a, nil
}

func (s *Store) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if a, ok := s.accounts[id]; ok {
		return a, nil
	}
	return nil, domain.ErrNotFound
}

func (s *Store) IsAccountAccessible(ctx context.Context, accountID, userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if accountID == "" || userID == "" {
		return false
	}
	account, ok := s.accounts[accountID]
	if !ok {
		return false
	}
	return account.AccountID == userID
}

func (s *Store) SetAccountStripeCustomerID(ctx context.Context, accountID, stripeCustomerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[accountID]
	if !ok {
		return domain.ErrNotFound
	}
	account.StripeCustomerID = stripeCustomerID
	return nil
}

func (s *Store) ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[accountID]
	if !ok {
		return domain.ErrNotFound
	}
	applyMockBillingPlan(account, stripeCustomerID, stripeSubscriptionID, plan)
	return nil
}

func (s *Store) ApplyBillingPlanByStripeCustomerID(ctx context.Context, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, account := range s.accounts {
		if account.StripeCustomerID == stripeCustomerID {
			applyMockBillingPlan(account, stripeCustomerID, stripeSubscriptionID, plan)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (s *Store) RecordBillingWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error) {
	if event == nil || event.EventID == "" {
		return true, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := event.Provider + ":" + event.EventID
	if _, ok := s.billingEvents[key]; ok {
		return false, nil
	}
	s.billingEvents[key] = "received"
	return true, nil
}

func (s *Store) MarkBillingWebhookEventProcessed(ctx context.Context, provider, eventID, status, errorMessage string) error {
	if eventID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.billingEvents[provider+":"+eventID] = status
	return nil
}

func (s *Store) ApplyBillingEvent(ctx context.Context, event *domain.ProviderWebhookEvent) error {
	if event == nil || event.Plan == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var account *domain.Account
	if event.AccountID != "" {
		account = s.accounts[event.AccountID]
	}
	if account == nil && event.ExternalCustomerID != "" {
		for _, candidate := range s.accounts {
			if candidate.StripeCustomerID == event.ExternalCustomerID {
				account = candidate
				break
			}
		}
	}
	if account == nil {
		return domain.ErrNotFound
	}
	applyMockBillingPlan(account, event.ExternalCustomerID, event.ExternalSubscriptionID, event.Plan)
	account.BillingStatus = string(event.Status)
	if account.BillingStatus == "" {
		if event.Plan == domain.BillingPlanFree {
			account.BillingStatus = string(domain.BillingStatusFree)
		} else {
			account.BillingStatus = string(domain.BillingStatusActive)
		}
	}
	account.StripePriceID = event.ExternalPriceID
	account.BillingCurrency = string(event.Currency)
	account.BillingAmountMinor = event.AmountMinor
	account.BillingInterval = string(event.Interval)
	account.CurrentPeriodEnd = event.CurrentPeriodEnd
	account.CancelAtPeriodEnd = event.CancelAtPeriodEnd
	account.BillingUpdatedAt = time.Now().Format(time.RFC3339)
	return nil
}

func applyMockBillingPlan(account *domain.Account, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) {
	account.Plan = string(plan)
	if stripeCustomerID != "" {
		account.StripeCustomerID = stripeCustomerID
	}
	account.StripeSubscriptionID = stripeSubscriptionID
	switch plan {
	case domain.BillingPlanPro:
		account.StorageQuotaBytes = 50 * 1 << 30
		account.MaxFileSizeBytes = 500 << 20
		account.MaxUploadsPerFiveH = 200
		account.MaxUploadsPerWeek = 1000
	default:
		account.StorageQuotaBytes = 5 * 1 << 30
		account.MaxFileSizeBytes = 100 << 20
		account.MaxUploadsPerFiveH = 20
		account.MaxUploadsPerWeek = 100
	}
}

// WorkspaceRepository
func (s *Store) ListWorkspacesByUser(ctx context.Context, userID string) []*domain.Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var res []*domain.Workspace
	for _, w := range s.workspaces {
		owner := s.wsOwners[w.WorkspaceID]
		if owner == "" {
			owner = w.AccountID
		}
		if owner == userID {
			res = append(res, s.workspaceWithAccount(w))
		}
	}
	return res
}

func (s *Store) GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.workspaces[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s.workspaceWithAccount(w), nil
}

func (s *Store) IsWorkspaceAccessible(ctx context.Context, wsID, userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if wsID == "" || userID == "" {
		return false
	}
	owner := s.wsOwners[wsID]
	if owner == "" {
		if ws, ok := s.workspaces[wsID]; ok {
			owner = ws.AccountID
		}
	}
	return owner != "" && owner == userID
}

func (s *Store) CreateWorkspace(ctx context.Context, accountID, name string) *domain.Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	account := s.accounts[accountID]
	w := &domain.Workspace{
		WorkspaceID: "ws-" + name,
		AccountID:   accountID,
		Name:        name,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	if account != nil {
		w.Plan = account.Plan
		w.StorageUsedBytes = account.StorageUsedBytes
		w.StorageQuotaBytes = account.StorageQuotaBytes
		w.MaxFileSizeBytes = account.MaxFileSizeBytes
		w.MaxUploadsPerFiveH = account.MaxUploadsPerFiveH
		w.MaxUploadsPerWeek = account.MaxUploadsPerWeek
	}
	s.workspaces[w.WorkspaceID] = w
	s.wsOwners[w.WorkspaceID] = accountID
	return w
}

func (s *Store) workspaceWithAccount(w *domain.Workspace) *domain.Workspace {
	if w == nil {
		return nil
	}
	copied := *w
	if account := s.accounts[w.AccountID]; account != nil {
		copied.Plan = account.Plan
		copied.StorageUsedBytes = account.StorageUsedBytes
		copied.StorageQuotaBytes = account.StorageQuotaBytes
		copied.MaxFileSizeBytes = account.MaxFileSizeBytes
		copied.MaxUploadsPerFiveH = account.MaxUploadsPerFiveH
		copied.MaxUploadsPerWeek = account.MaxUploadsPerWeek
	}
	return &copied
}

// DocumentRepository
func (s *Store) ListDocuments(ctx context.Context, wsID string) []*domain.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var res []*domain.Document
	for _, d := range s.documents {
		if d.WorkspaceID == wsID {
			res = append(res, d)
		}
	}
	return res
}

func (s *Store) GetDocument(ctx context.Context, id string) (*domain.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if d, ok := s.documents[id]; ok {
		return d, nil
	}
	return nil, domain.ErrNotFound
}

func (s *Store) GetDocumentChunks(ctx context.Context, documentID string) ([]*domain.DocumentChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	chunks, ok := s.chunks[documentID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := make([]*domain.DocumentChunk, len(chunks))
	copy(copied, chunks)
	return copied, nil
}

func (s *Store) GetJobPlanningSignals(ctx context.Context, documentID, workspaceID, treeID string) (*domain.JobPlanningSignals, error) {
	return &domain.JobPlanningSignals{DocumentID: documentID, WorkspaceID: workspaceID}, nil
}

func (s *Store) CreateDocument(ctx context.Context, wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[wsID]
	if !ok {
		return nil, "", domain.ErrNotFound
	}
	account, ok := s.accounts[ws.AccountID]
	if !ok {
		return nil, "", domain.ErrNotFound
	}
	if err := validateMockUpload(account, fileSize); err != nil {
		return nil, "", err
	}
	d := &domain.Document{
		DocumentID:  "doc-" + filename,
		WorkspaceID: wsID,
		UploadedBy:  uploadedBy,
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    fileSize,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	s.documents[d.DocumentID] = d
	s.reservations[d.DocumentID] = &uploadReservation{
		AccountID:    account.AccountID,
		WorkspaceID:  wsID,
		DocumentID:   d.DocumentID,
		ExpectedSize: fileSize,
		Status:       "reserved",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}
	return d, "http://mock-upload-url/" + d.DocumentID, nil
}

func (s *Store) ConfirmDocumentUpload(ctx context.Context, documentID string, actualSize int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	reservation, ok := s.reservations[documentID]
	if !ok || reservation.Status == "failed" || time.Now().After(reservation.ExpiresAt) {
		return domain.ErrUploadNotConfirmed
	}
	if reservation.Status == "confirmed" {
		return nil
	}
	if reservation.ExpectedSize != actualSize {
		reservation.Status = "failed"
		return domain.ErrUploadSizeMismatch
	}
	account, ok := s.accounts[reservation.AccountID]
	if !ok {
		return domain.ErrNotFound
	}
	if account.StorageUsedBytes+actualSize > account.StorageQuotaBytes {
		return domain.ErrStorageQuotaExceeded
	}
	account.StorageUsedBytes += actualSize
	reservation.ActualSize = actualSize
	reservation.Status = "confirmed"
	return nil
}

func validateMockUpload(account *domain.Account, fileSize int64) error {
	if fileSize <= 0 {
		return domain.ErrUploadSizeMismatch
	}
	if account.MaxFileSizeBytes > 0 && fileSize > account.MaxFileSizeBytes {
		return domain.ErrFileTooLarge
	}
	if account.StorageQuotaBytes > 0 && account.StorageUsedBytes+fileSize > account.StorageQuotaBytes {
		return domain.ErrStorageQuotaExceeded
	}
	return nil
}

func (s *Store) CreateDocumentFile(ctx context.Context, docID, path, mimeType string, fileSize int64) (*domain.DocumentFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.docFiles[docID]; !ok {
		s.docFiles[docID] = make(map[string]*domain.DocumentFile)
	}
	f := &domain.DocumentFile{
		FileID:     "file-" + path,
		DocumentID: docID,
		Path:       path,
		MimeType:   mimeType,
		FileSize:   fileSize,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}
	s.docFiles[docID][f.FileID] = f
	return f, nil
}

func (s *Store) ListDocumentFiles(ctx context.Context, docID string) ([]*domain.DocumentFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var res []*domain.DocumentFile
	for _, f := range s.docFiles[docID] {
		res = append(res, f)
	}
	return res, nil
}

func (s *Store) GetDocumentFileByPath(ctx context.Context, docID, path string) (*domain.DocumentFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.docFiles[docID] {
		if f.Path == path {
			return f, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *Store) GetLatestProcessingJob(ctx context.Context, docID string) (*domain.DocumentProcessingJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, j := range s.jobs {
		if j.DocumentID == docID {
			return j, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *Store) GetProcessingJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if j, ok := s.jobs[jobID]; ok {
		return j, nil
	}
	return &domain.DocumentProcessingJob{
		JobID:       jobID,
		DocumentID:  "mock-doc-" + jobID,
		WorkspaceID: "mock-ws",
		Status:      1, // running
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Store) GetJobCapability(ctx context.Context, jobID string) (*domain.JobCapability, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.capabilities[jobID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (s *Store) GetJobExecutionPlan(ctx context.Context, jobID string) (*domain.JobExecutionPlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.plans[jobID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (s *Store) UpsertJobExecutionPlan(ctx context.Context, jobID string, plan *domain.JobExecutionPlan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plans[jobID] = plan
	return nil
}

func (s *Store) UpsertJobEvaluation(ctx context.Context, jobID string, result *domain.JobEvaluationResult) error {
	return nil
}

func (s *Store) EvaluateJob(ctx context.Context, jobID string) (*domain.JobEvaluationResult, error) {
	return &domain.JobEvaluationResult{JobID: jobID, Passed: true, Summary: "mock eval passed"}, nil
}

func (s *Store) ListJobApprovalRequests(ctx context.Context, jobID string) ([]*domain.JobApprovalRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.approvals[jobID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (s *Store) RequestJobApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error) {
	return &domain.JobApprovalRequest{JobID: jobID, Status: "pending"}, nil
}

func (s *Store) ApproveJobApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error {
	return nil
}
func (s *Store) RejectJobApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error {
	return nil
}

func (s *Store) CreateProcessingJob(ctx context.Context, docID, workspaceID string, jobType treev1.JobType) *domain.DocumentProcessingJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := &domain.DocumentProcessingJob{
		JobID:       "job-" + docID,
		DocumentID:  docID,
		WorkspaceID: workspaceID,
		JobType:     jobType,
		Status:      treev1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	s.jobs[j.JobID] = j
	return j
}

func (s *Store) MarkProcessingJobRunning(ctx context.Context, jobID string) error        { return nil }
func (s *Store) UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error { return nil }
func (s *Store) FailProcessingJob(ctx context.Context, jobID, errorMessage string) error { return nil }
func (s *Store) CompleteProcessingJob(ctx context.Context, jobID string) error           { return nil }

// CheckpointRepository
func (s *Store) UpsertStageRunning(ctx context.Context, jobID, stage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.checkpoints[jobID]; !ok {
		s.checkpoints[jobID] = make(map[string]domain.JobStageCheckpoint)
	}
	s.checkpoints[jobID][stage] = domain.JobStageCheckpoint{
		JobID:     jobID,
		Stage:     stage,
		Status:    "running",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return nil
}

func (s *Store) MarkStageSucceeded(ctx context.Context, jobID, stage, gcsRef string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.checkpoints[jobID]; !ok {
		s.checkpoints[jobID] = make(map[string]domain.JobStageCheckpoint)
	}
	s.checkpoints[jobID][stage] = domain.JobStageCheckpoint{
		JobID:     jobID,
		Stage:     stage,
		Status:    "succeeded",
		GCSRef:    gcsRef,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return nil
}

func (s *Store) MarkStageFailed(ctx context.Context, jobID, stage, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.checkpoints[jobID]; !ok {
		s.checkpoints[jobID] = make(map[string]domain.JobStageCheckpoint)
	}
	s.checkpoints[jobID][stage] = domain.JobStageCheckpoint{
		JobID:     jobID,
		Stage:     stage,
		Status:    "failed",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return nil
}

func (s *Store) ListStageCheckpoints(ctx context.Context, jobID string) ([]domain.JobStageCheckpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var res []domain.JobStageCheckpoint
	for _, cp := range s.checkpoints[jobID] {
		res = append(res, cp)
	}
	return res, nil
}

func (s *Store) ListAllJobs(ctx context.Context) ([]*domain.DocumentProcessingJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// If no jobs exist, return a set of realistic mock jobs for UI testing
	if len(s.jobs) == 0 {
		now := time.Now().UTC()
		return []*domain.DocumentProcessingJob{
			{
				JobID:      "job-audit-demo-1",
				DocumentID: "annual_report_2024.pdf",
				Status:     treev1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED,
				CreatedAt:  now.Add(-1 * time.Hour).Format(time.RFC3339),
			},
			{
				JobID:      "job-audit-demo-2",
				DocumentID: "technical_spec_v2.docx",
				Status:     treev1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING,
				CreatedAt:  now.Add(-30 * time.Minute).Format(time.RFC3339),
			},
			{
				JobID:      "job-audit-demo-3",
				DocumentID: "contract_legal_v1.pdf",
				Status:     treev1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED,
				CreatedAt:  now.Add(-2 * time.Hour).Format(time.RFC3339),
			},
		}, nil
	}

	var res []*domain.DocumentProcessingJob
	for _, j := range s.jobs {
		res = append(res, j)
	}
	return res, nil
}

func (s *Store) SaveDocumentChunks(ctx context.Context, documentID string, chunks []*domain.DocumentChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]*domain.DocumentChunk, len(chunks))
	copy(copied, chunks)
	s.chunks[documentID] = copied
	return nil
}

func (s *Store) LogToolCall(ctx context.Context, jobID, toolName, inputJSON, outputJSON string, durationMs int64) error {
	return nil
}

func (s *Store) SearchRelatedChunksByVector(ctx context.Context, workspaceID string, embedding []float32, limit int) ([]*domain.DocumentChunk, error) {
	return nil, nil
}

func (s *Store) ListJobMutationLogs(ctx context.Context, jobID string) ([]*domain.JobMutationLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return real mock data for any requested jobId to facilitate UI testing
	now := time.Now().UTC()
	logs := []*domain.JobMutationLog{
		{
			MutationID:   "m1",
			JobID:        jobID,
			TargetType:   "agent",
			TargetID:     "Orchestrator",
			MutationType: "start",
			CreatedAt:    now.Add(-10 * time.Minute).Format(time.RFC3339),
		},
		{
			MutationID:     "m2",
			JobID:          jobID,
			TargetType:     "tool_call",
			TargetID:       "manage_job_checklist",
			MutationType:   "execute",
			BeforeJSON:     `{"action": "add", "description": "Analyze document structure"}`,
			AfterJSON:      `{"status": "ok", "task_id": "T1"}`,
			ProvenanceJSON: `{"duration_ms": 150}`,
			CreatedAt:      now.Add(-9 * time.Minute).Format(time.RFC3339),
		},
		{
			MutationID:     "m3",
			JobID:          jobID,
			TargetType:     "tool_call",
			TargetID:       "extract_text",
			MutationType:   "execute",
			BeforeJSON:     `{"file_uri": "gs://bucket/doc.pdf"}`,
			AfterJSON:      `{"raw_text": "Extracted content..."}`,
			ProvenanceJSON: `{"duration_ms": 1200}`,
			CreatedAt:      now.Add(-8 * time.Minute).Format(time.RFC3339),
		},
		{
			MutationID:     "m4",
			JobID:          jobID,
			TargetType:     "tool_call",
			TargetID:       "semantic_chunking",
			MutationType:   "execute",
			BeforeJSON:     `{"raw_text": "..."}`,
			AfterJSON:      `{"chunks": [{"index":0, "text": "..."}]}`,
			ProvenanceJSON: `{"duration_ms": 2500}`,
			CreatedAt:      now.Add(-7 * time.Minute).Format(time.RFC3339),
		},
		{
			MutationID:     "m5",
			JobID:          jobID,
			TargetType:     "tool_call",
			TargetID:       "goal_driven_synthesis",
			MutationType:   "execute",
			BeforeJSON:     `{"document_brief": "Master blueprint..."}`,
			AfterJSON:      `{"items": [{"label": "Concept A", "level": 0}]}`,
			ProvenanceJSON: `{"duration_ms": 5000}`,
			CreatedAt:      now.Add(-5 * time.Minute).Format(time.RFC3339),
		},
		{
			MutationID:     "m6",
			JobID:          jobID,
			TargetType:     "tool_call",
			TargetID:       "quality_critique",
			MutationType:   "execute",
			BeforeJSON:     `{"target_data": "..."}`,
			AfterJSON:      `{"valid": true, "issues": []}`,
			ProvenanceJSON: `{"duration_ms": 3200}`,
			CreatedAt:      now.Add(-2 * time.Minute).Format(time.RFC3339),
		},
		{
			MutationID:     "m7",
			JobID:          jobID,
			TargetType:     "tool_call",
			TargetID:       "persist_knowledge_tree",
			MutationType:   "execute",
			BeforeJSON:     `{"items": [...]}`,
			AfterJSON:      `{"success": true}`,
			ProvenanceJSON: `{"duration_ms": 450}`,
			CreatedAt:      now.Add(-1 * time.Minute).Format(time.RFC3339),
		},
	}
	return logs, nil
}

func (s *Store) ListJobLogs(ctx context.Context, jobID string, pageToken string, limit int) ([]*domain.JobLog, string, error) {
	logs, _ := s.ListJobMutationLogs(ctx, jobID)
	out := make([]*domain.JobLog, 0, len(logs)+2)
	now := time.Now().UTC()
	job, _ := s.GetProcessingJob(ctx, jobID)
	out = append(out, &domain.JobLog{
		Timestamp:   now.Add(-11 * time.Minute).Format(time.RFC3339),
		Level:       "INFO",
		Event:       "job.running",
		Message:     "Job started",
		DetailJSON:  "{}",
		Source:      "system",
		SourceID:    "mock-job-running-" + jobID,
		JobID:       jobID,
		DocumentID:  job.DocumentID,
		WorkspaceID: job.WorkspaceID,
	})
	for _, log := range logs {
		out = append(out, mutationLogToJobLog(log, job.DocumentID))
	}
	out = append(out, &domain.JobLog{
		Timestamp:   now.Format(time.RFC3339),
		Level:       "INFO",
		Event:       "job.completed",
		Message:     "Job completed",
		DetailJSON:  "{}",
		Source:      "system",
		SourceID:    "mock-job-completed-" + jobID,
		JobID:       jobID,
		DocumentID:  job.DocumentID,
		WorkspaceID: job.WorkspaceID,
	})
	page, nextToken := paginateMockLogs(out, pageToken, limit)
	return page, nextToken, nil
}

func (s *Store) SearchJobLogs(ctx context.Context, filter domain.JobLogSearchFilter) ([]*domain.JobLog, string, error) {
	if filter.JobID != "" {
		logs, _, _ := s.ListJobLogs(ctx, filter.JobID, "", 0)
		filtered := filterMockJobLogs(logs, filter)
		page, nextToken := paginateMockLogs(filtered, filter.PageToken, filter.Limit)
		return page, nextToken, nil
	}
	jobs, _ := s.ListAllJobs(ctx)
	var out []*domain.JobLog
	for _, job := range jobs {
		if filter.DocumentID != "" && job.DocumentID != filter.DocumentID {
			continue
		}
		if filter.WorkspaceID != "" && job.WorkspaceID != "" && job.WorkspaceID != filter.WorkspaceID {
			continue
		}
		logs, _, _ := s.ListJobLogs(ctx, job.JobID, "", 0)
		out = append(out, filterMockJobLogs(logs, filter)...)
	}
	page, nextToken := paginateMockLogs(out, filter.PageToken, filter.Limit)
	return page, nextToken, nil
}

func (s *Store) ListRelatedJobLogs(ctx context.Context, scope domain.RelatedLogScope, workspaceID, documentID, jobID string, pageToken string, limit int) ([]*domain.JobLogGroup, string, error) {
	jobs, _ := s.ListAllJobs(ctx)
	if scope == domain.RelatedLogScopeJob {
		for _, job := range jobs {
			if job.JobID == jobID {
				documentID = job.DocumentID
				break
			}
		}
	}
	groupsByDocument := map[string]*domain.JobLogGroup{}
	for _, job := range jobs {
		if (scope == domain.RelatedLogScopeJob || scope == domain.RelatedLogScopeDocument) && job.DocumentID != documentID {
			continue
		}
		if scope == domain.RelatedLogScopeWorkspace && workspaceID != "" && job.WorkspaceID != "" && job.WorkspaceID != workspaceID {
			continue
		}
		group := groupsByDocument[job.DocumentID]
		if group == nil {
			group = &domain.JobLogGroup{WorkspaceID: job.WorkspaceID, DocumentID: job.DocumentID}
			groupsByDocument[job.DocumentID] = group
		}
		logs, _, _ := s.ListJobLogs(ctx, job.JobID, "", 50)
		group.Jobs = append(group.Jobs, &domain.JobLogJob{
			JobID:     job.JobID,
			Status:    job.Status,
			CreatedAt: job.CreatedAt,
			Logs:      logs,
		})
	}
	var groups []*domain.JobLogGroup
	for _, group := range groupsByDocument {
		groups = append(groups, group)
	}
	return groups, "", nil
}

func mutationLogToJobLog(log *domain.JobMutationLog, documentID string) *domain.JobLog {
	return &domain.JobLog{
		Timestamp:   log.CreatedAt,
		Level:       "INFO",
		Event:       "tool.call.completed",
		Message:     log.TargetID,
		DetailJSON:  fmt.Sprintf(`{"tool":%q,"target_type":%q,"mutation_type":%q,"risk_tier":%q,"input":%s,"output":%s,"provenance":%s}`, log.TargetID, log.TargetType, log.MutationType, log.RiskTier, emptyJSON(log.BeforeJSON), emptyJSON(log.AfterJSON), emptyJSON(log.ProvenanceJSON)),
		Source:      "tool",
		SourceID:    log.MutationID,
		JobID:       log.JobID,
		DocumentID:  documentID,
		WorkspaceID: log.WorkspaceID,
	}
}

func emptyJSON(v string) string {
	if strings.TrimSpace(v) == "" {
		return "{}"
	}
	return v
}

func filterMockJobLogs(logs []*domain.JobLog, filter domain.JobLogSearchFilter) []*domain.JobLog {
	query := strings.ToLower(filter.Query)
	var out []*domain.JobLog
	for _, log := range logs {
		if query != "" && !strings.Contains(strings.ToLower(log.Event+" "+log.Message+" "+log.DetailJSON), query) {
			continue
		}
		if len(filter.Levels) > 0 && !containsString(filter.Levels, log.Level) {
			continue
		}
		if len(filter.Events) > 0 && !containsString(filter.Events, log.Event) {
			continue
		}
		if filter.FromTimestamp != "" && log.Timestamp < filter.FromTimestamp {
			continue
		}
		if filter.ToTimestamp != "" && log.Timestamp > filter.ToTimestamp {
			continue
		}
		out = append(out, log)
	}
	return out
}

func paginateMockLogs(logs []*domain.JobLog, pageToken string, limit int) ([]*domain.JobLog, string) {
	if limit <= 0 || len(logs) <= limit {
		return logs, ""
	}

	end := len(logs)
	if pageToken != "" {
		if _, err := fmt.Sscanf(pageToken, "%d", &end); err != nil || end < 0 || end > len(logs) {
			end = len(logs)
		}
	}
	start := end - limit
	if start < 0 {
		start = 0
	}

	nextToken := ""
	if start > 0 {
		nextToken = fmt.Sprintf("%d", start)
	}
	return logs[start:end], nextToken
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// TreeRepository
func (s *Store) GetOrCreateTree(ctx context.Context, wsID string) (*domain.Tree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[wsID]; !ok {
		s.items[wsID] = map[string]*domain.Item{
			"nd_root": {
				ItemID:      "nd_root",
				WorkspaceID: wsID,
				ParentID:    "",
				Title:       "root",
			},
		}
	}
	return &domain.Tree{TreeID: wsID, WorkspaceID: wsID, Name: "default"}, nil
}

func (s *Store) GetTreeByWorkspace(ctx context.Context, wsID string) ([]*domain.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wsItems, ok := s.items[wsID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	var res []*domain.Item
	for _, n := range wsItems {
		res = append(res, n)
	}
	return res, nil
}

func (s *Store) GetWorkspaceRootItemID(ctx context.Context, wsID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wsItems, ok := s.items[wsID]
	if !ok {
		return "", domain.ErrNotFound
	}
	for _, n := range wsItems {
		if n.ParentID == "" {
			return n.ItemID, nil
		}
	}
	return "", domain.ErrNotFound
}

func (s *Store) FindPaths(ctx context.Context, wsID, sourceItemID, targetItemID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error) {
	items, err := s.GetTreeByWorkspace(ctx, wsID)
	if err != nil {
		return nil, nil, err
	}
	return items, nil, nil
}

func (s *Store) GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var res []*domain.SubtreeItem
	var root *domain.Item
	var workspaceID string

	// Find the root item and its workspace
	for wsID, wsItems := range s.items {
		if n, ok := wsItems[rootItemID]; ok {
			root = n
			workspaceID = wsID
			break
		}
	}

	if root == nil {
		return nil, fmt.Errorf("root item not found")
	}

	res = append(res, &domain.SubtreeItem{Item: *root})

	// Find direct children
	for _, n := range s.items[workspaceID] {
		if n.ParentID == rootItemID {
			res = append(res, &domain.SubtreeItem{Item: *n})
		}
	}

	return res, nil
}

// ItemRepository
func (s *Store) GetItem(ctx context.Context, itemID string) (*domain.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, wsItems := range s.items {
		if n, ok := wsItems[itemID]; ok {
			return n, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *Store) CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) *domain.Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[workspaceID]; !ok {
		s.items[workspaceID] = make(map[string]*domain.Item)
	}
	n := &domain.Item{
		ItemID:      "item-" + label,
		WorkspaceID: workspaceID,
		ParentID:    parentID,
		Title:       label,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	s.items[workspaceID][n.ItemID] = n
	return n
}

func (s *Store) CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) *domain.Item {
	return s.CreateItem(ctx, workspaceID, label, description, parentID, createdBy)
}

func (s *Store) UpsertItemSource(ctx context.Context, itemID, documentID, fileID, chunkID, sourceText string, confidence float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[itemID] = append(s.sources[itemID], &domain.ItemSource{
		ItemID:     itemID,
		DocumentID: documentID,
		FileID:     fileID,
		ChunkID:    chunkID,
		SourceText: sourceText,
		Confidence: confidence,
	})
	return nil
}

func (s *Store) UpdateItemSummaryHTMLWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, summaryHTML string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, wsItems := range s.items {
		if n, ok := wsItems[itemID]; ok {
			n.Content = summaryHTML
			return nil
		}
	}
	return domain.ErrNotFound
}

func (s *Store) ApproveAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error {
	return nil
}
func (s *Store) RejectAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error {
	return nil
}

func (s *Store) LogJobEvent(ctx context.Context, e joblog.Event) error {
	return nil
}

var _ repository.AccountRepository = (*Store)(nil)
var _ repository.WorkspaceRepository = (*Store)(nil)
var _ repository.DocumentRepository = (*Store)(nil)
var _ repository.TreeRepository = (*Store)(nil)
var _ repository.ItemRepository = (*Store)(nil)

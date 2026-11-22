package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	"github.com/synthify/backend/internal/gen/synthify/app/v1/appv1connect"
)

// intakeTestServer serves the document, job and billing services the way
// bootstrap wires them, over HTTP, so the interceptor sees real procedure
// names. Every request is made by the owner, standing in for the auth
// middleware.
type intakeTestServer struct {
	documents *recordingDocuments
	billing   *recordingBilling
	document  appv1connect.DocumentServiceClient
	job       appv1connect.JobServiceClient
	bill      appv1connect.BillingServiceClient
}

func newIntakeTestServer(t *testing.T, closed bool) *intakeTestServer {
	t.Helper()
	store := mock.NewStore()
	documents := &recordingDocuments{}
	billing := &recordingBilling{}
	opts := IntakeClosedHandlerOptions(closed)

	mux := http.NewServeMux()
	mux.Handle(appv1connect.NewDocumentServiceHandler(NewDocumentHandler(documents, store), opts...))
	mux.Handle(appv1connect.NewJobServiceHandler(NewJobHandler(store, store, store, store, store, nil), opts...))
	mux.Handle(appv1connect.NewBillingServiceHandler(NewBillingHandler(billing), opts...))
	asOwner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.ContextWithPrincipal(r.Context(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner", Email: "owner@example.com"})
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	srv := httptest.NewServer(asOwner)
	t.Cleanup(srv.Close)

	return &intakeTestServer{
		documents: documents,
		billing:   billing,
		document:  appv1connect.NewDocumentServiceClient(srv.Client(), srv.URL),
		job:       appv1connect.NewJobServiceClient(srv.Client(), srv.URL),
		bill:      appv1connect.NewBillingServiceClient(srv.Client(), srv.URL),
	}
}

type intakeCall struct {
	procedure string
	call      func(context.Context, *intakeTestServer) error
}

// refusedCalls has one call for each RPC that closed intake refuses, with
// arguments that would otherwise pass the handler's own validation.
var refusedCalls = map[string]intakeCall{
	"CreateDocument": {appv1connect.DocumentServiceCreateDocumentProcedure, func(ctx context.Context, s *intakeTestServer) error {
		_, err := s.document.CreateDocument(ctx, connect.NewRequest(&appv1.CreateDocumentRequest{WorkspaceId: "ws", Filename: "a.pdf", MimeType: "application/pdf", FileSize: 10}))
		return err
	}},
	"GetUploadURL": {appv1connect.DocumentServiceGetUploadURLProcedure, func(ctx context.Context, s *intakeTestServer) error {
		_, err := s.document.GetUploadURL(ctx, connect.NewRequest(&appv1.GetUploadURLRequest{WorkspaceId: "ws", Filename: "a.pdf", MimeType: "application/pdf", FileSize: 10}))
		return err
	}},
	"ConfirmUpload": {appv1connect.DocumentServiceConfirmUploadProcedure, func(ctx context.Context, s *intakeTestServer) error {
		_, err := s.document.ConfirmUpload(ctx, connect.NewRequest(&appv1.ConfirmUploadRequest{DocumentId: "doc"}))
		return err
	}},
	"StartProcessing": {appv1connect.DocumentServiceStartProcessingProcedure, func(ctx context.Context, s *intakeTestServer) error {
		_, err := s.document.StartProcessing(ctx, connect.NewRequest(&appv1.StartProcessingRequest{DocumentId: "doc"}))
		return err
	}},
	"ResumeProcessing": {appv1connect.DocumentServiceResumeProcessingProcedure, func(ctx context.Context, s *intakeTestServer) error {
		_, err := s.document.ResumeProcessing(ctx, connect.NewRequest(&appv1.ResumeProcessingRequest{DocumentId: "doc"}))
		return err
	}},
	"ApproveJobApproval": {appv1connect.JobServiceApproveJobApprovalProcedure, func(ctx context.Context, s *intakeTestServer) error {
		_, err := s.job.ApproveJobApproval(ctx, connect.NewRequest(&appv1.ApproveJobApprovalRequest{JobId: "job", ApprovalId: "approval"}))
		return err
	}},
	"CreateCheckoutSession": {appv1connect.BillingServiceCreateCheckoutSessionProcedure, func(ctx context.Context, s *intakeTestServer) error {
		_, err := s.bill.CreateCheckoutSession(ctx, connect.NewRequest(&appv1.CreateCheckoutSessionRequest{AccountId: "owner", Currency: "jpy"}))
		return err
	}},
}

func TestIntakeClosed_RefusesNewDocumentWorkAndCheckout(t *testing.T) {
	ctx := context.Background()
	for name, tc := range refusedCalls {
		t.Run(name, func(t *testing.T) {
			s := newIntakeTestServer(t, true)

			err := tc.call(ctx, s)

			want := intakeClosedDocumentsMessage
			if tc.procedure == appv1connect.BillingServiceCreateCheckoutSessionProcedure {
				want = intakeClosedPaidPlansMessage
			}
			assertIntakeClosed(t, err, want)
			assert.Empty(t, s.documents.called(), "document service must not run")
			assert.Empty(t, s.billing.called(), "billing service must not run")
		})
	}
}

// Every refused procedure has a test above, and every test is for a refused
// procedure.
func TestIntakeClosed_RefusedCallsMatchTheList(t *testing.T) {
	tested := map[string]bool{}
	for _, tc := range refusedCalls {
		tested[tc.procedure] = true
		assert.Contains(t, intakeClosedProcedures, tc.procedure)
	}
	for procedure := range intakeClosedProcedures {
		assert.True(t, tested[procedure], "no test for %s", procedure)
	}
}

func TestIntakeClosed_KeepsDeletionReadingAndThePortal(t *testing.T) {
	ctx := context.Background()
	s := newIntakeTestServer(t, true)

	_, err := s.document.DeleteDocument(ctx, connect.NewRequest(&appv1.DeleteDocumentRequest{DocumentId: "doc"}))
	require.NoError(t, err)

	_, err = s.document.ListDocuments(ctx, connect.NewRequest(&appv1.ListDocumentsRequest{WorkspaceId: "ws"}))
	require.NoError(t, err)

	portal, err := s.bill.CreatePortalSession(ctx, connect.NewRequest(&appv1.CreatePortalSessionRequest{AccountId: "owner"}))
	require.NoError(t, err)
	assert.Equal(t, "https://billing.example/portal", portal.Msg.GetPortalUrl())

	assert.Equal(t, []string{"DeleteDocument", "ListDocuments"}, s.documents.called())
	assert.Equal(t, []string{"CreatePortalSession"}, s.billing.called())
}

func TestIntakeOpen_RefusesNothing(t *testing.T) {
	assert.Empty(t, IntakeClosedHandlerOptions(false))

	ctx := context.Background()
	s := newIntakeTestServer(t, false)

	_, err := s.document.CreateDocument(ctx, connect.NewRequest(&appv1.CreateDocumentRequest{WorkspaceId: "ws", Filename: "a.pdf", MimeType: "application/pdf", FileSize: 10}))
	require.NoError(t, err)
	_, err = s.bill.CreateCheckoutSession(ctx, connect.NewRequest(&appv1.CreateCheckoutSessionRequest{AccountId: "owner", Currency: "jpy"}))
	require.NoError(t, err)

	assert.Equal(t, []string{"CreateDocument"}, s.documents.called())
	assert.Equal(t, []string{"CreateCheckoutSession"}, s.billing.called())
}

func TestUserHandler_SignInUserReportsIntake(t *testing.T) {
	for _, closed := range []bool{false, true} {
		store := mock.NewStore()
		userSvc := application.NewUserService(application.UserServiceDeps{Users: store, Accounts: store})
		h := NewUserHandler(userSvc).WithIntakeClosed(closed)
		ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner", Email: "owner@example.com"})

		resp, err := h.SignInUser(ctx, connect.NewRequest(&appv1.SignInUserRequest{}))

		require.NoError(t, err)
		assert.Equal(t, closed, resp.Msg.GetIntakeClosed())
	}
}

// assertIntakeClosed checks what the web relies on: FailedPrecondition, the
// message to show, and the INTAKE_CLOSED reason.
func assertIntakeClosed(t *testing.T, err error, message string) {
	t.Helper()
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, connect.CodeFailedPrecondition, ce.Code())
	assert.Equal(t, message, ce.Message())
	var reasons []appv1.ErrorReason
	for _, detail := range ce.Details() {
		value, valueErr := detail.Value()
		require.NoError(t, valueErr)
		if errorDetail, ok := value.(*appv1.ErrorDetail); ok {
			reasons = append(reasons, errorDetail.GetReason())
		}
	}
	assert.Equal(t, []appv1.ErrorReason{appv1.ErrorReason_ERROR_REASON_INTAKE_CLOSED}, reasons)
}

// recordingDocuments is a DocumentUsecase that records which methods ran.
// The server runs them on its own goroutines, hence the lock.
type recordingDocuments struct {
	mu    sync.Mutex
	calls []string
}

func (d *recordingDocuments) record(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, name)
}

func (d *recordingDocuments) called() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

func (d *recordingDocuments) ListDocuments(context.Context, string, string) ([]*domain.Document, error) {
	d.record("ListDocuments")
	return nil, nil
}

func (d *recordingDocuments) GetDocument(_ context.Context, documentID, _ string) (*domain.Document, error) {
	d.record("GetDocument")
	return &domain.Document{DocumentID: documentID}, nil
}

func (d *recordingDocuments) CreateDocument(context.Context, string, string, string, string, int64) (*domain.Document, repository.DocumentUploadTarget, error) {
	d.record("CreateDocument")
	return &domain.Document{DocumentID: "doc"}, repository.DocumentUploadTarget{URL: "https://upload.example/doc", Method: http.MethodPut}, nil
}

func (d *recordingDocuments) ConfirmUpload(_ context.Context, documentID, _ string) (*domain.Document, error) {
	d.record("ConfirmUpload")
	return &domain.Document{DocumentID: documentID}, nil
}

func (d *recordingDocuments) StartProcessing(_ context.Context, documentID, _ string, _ bool) (*domain.DocumentProcessingJob, error) {
	d.record("StartProcessing")
	return &domain.DocumentProcessingJob{JobID: "job", DocumentID: documentID}, nil
}

func (d *recordingDocuments) ResumeProcessing(_ context.Context, documentID, _ string) (*domain.DocumentProcessingJob, error) {
	d.record("ResumeProcessing")
	return &domain.DocumentProcessingJob{JobID: "job", DocumentID: documentID}, nil
}

func (d *recordingDocuments) IssueImageURL(context.Context, string, string) (repository.SignedURL, error) {
	d.record("IssueImageURL")
	return repository.SignedURL{URL: "https://image.example/file"}, nil
}

func (d *recordingDocuments) DeleteDocument(context.Context, string, string) error {
	d.record("DeleteDocument")
	return nil
}

// recordingBilling records the Stripe session RPCs. The test calls no other
// BillingUsecase method; the embedded nil interface would panic if one ran.
type recordingBilling struct {
	BillingUsecase
	mu    sync.Mutex
	calls []string
}

func (b *recordingBilling) record(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, name)
}

func (b *recordingBilling) called() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.calls...)
}

func (b *recordingBilling) CreateCheckoutSession(context.Context, string, string, domain.BillingPlan, domain.BillingCurrency) (*domain.BillingCheckoutSession, error) {
	b.record("CreateCheckoutSession")
	return &domain.BillingCheckoutSession{URL: "https://checkout.example/session"}, nil
}

func (b *recordingBilling) CreatePortalSession(context.Context, string, string) (*domain.BillingPortalSession, error) {
	b.record("CreatePortalSession")
	return &domain.BillingPortalSession{URL: "https://billing.example/portal"}, nil
}

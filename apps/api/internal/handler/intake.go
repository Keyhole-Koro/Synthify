package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	"github.com/synthify/backend/internal/gen/synthify/app/v1/appv1connect"
)

// The messages users see when intake is closed. The web shows them as they
// are.
const (
	intakeClosedDocumentsMessage = "新しい文書の受け付けは終了しました。"
	intakeClosedPaidPlansMessage = "有料プランの受け付けは終了しました。"
)

// intakeClosedProcedures are the RPCs refused while intake is closed, with
// the message each answers. They are every RPC that starts new document work
// or hands a job to the worker (and so to the LLM), and Stripe Checkout.
// ApproveJobApproval only records the approval; the plan runs when
// StartProcessing or ResumeProcessing dispatches it. It is refused too, so
// nobody approves a plan that can no longer run.
//
// Everything else stays open: reading, sharing, members, CreateItem (no
// LLM), every deletion, the billing portal (so subscribers can cancel), and
// the worker's own calls (RecordUsage) for jobs already running.
var intakeClosedProcedures = map[string]string{
	appv1connect.DocumentServiceCreateDocumentProcedure:       intakeClosedDocumentsMessage,
	appv1connect.DocumentServiceGetUploadURLProcedure:         intakeClosedDocumentsMessage,
	appv1connect.DocumentServiceConfirmUploadProcedure:        intakeClosedDocumentsMessage,
	appv1connect.DocumentServiceStartProcessingProcedure:      intakeClosedDocumentsMessage,
	appv1connect.DocumentServiceResumeProcessingProcedure:     intakeClosedDocumentsMessage,
	appv1connect.JobServiceApproveJobApprovalProcedure:        intakeClosedDocumentsMessage,
	appv1connect.BillingServiceCreateCheckoutSessionProcedure: intakeClosedPaidPlansMessage,
}

// IntakeClosedHandlerOptions returns the Connect handler options that refuse
// intakeClosedProcedures when closed is true (SYNTHIFY_INTAKE_CLOSED), and
// none otherwise. A refused call never reaches its handler; it fails with
// FailedPrecondition and an ErrorDetail whose reason is INTAKE_CLOSED, which
// the web recognises.
func IntakeClosedHandlerOptions(closed bool) []connect.HandlerOption {
	if !closed {
		return nil
	}
	return []connect.HandlerOption{connect.WithInterceptors(refuseClosedIntake())}
}

func refuseClosedIntake() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if message, refused := intakeClosedProcedures[req.Spec().Procedure]; refused {
				return nil, intakeClosedError(message)
			}
			return next(ctx, req)
		}
	}
}

func intakeClosedError(message string) *connect.Error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(message))
	// NewErrorDetail fails only when the message cannot be marshalled.
	if detail, detailErr := connect.NewErrorDetail(&appv1.ErrorDetail{Reason: appv1.ErrorReason_ERROR_REASON_INTAKE_CLOSED}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

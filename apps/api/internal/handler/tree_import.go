package handler

import (
	"context"
	"errors"
	"fmt"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/stif"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// maxStifSourceBytes caps what a single import may submit. STIF documents are
// hand- or model-authored notes; anything past this is a mistake (a pasted
// binary, a runaway generation) and parsing it would just burn request time.
const maxStifSourceBytes = 1 << 20 // 1 MiB

// TreeImportUsecase is the application API required by TreeImportHandler.
type TreeImportUsecase interface {
	ImportStif(ctx context.Context, workspaceID, source, userID string, dryRun bool) (*application.ImportResult, error)
}

type TreeImportHandler struct {
	service TreeImportUsecase
}

func NewTreeImportHandler(service TreeImportUsecase) *TreeImportHandler {
	return &TreeImportHandler{service: service}
}

func (h *TreeImportHandler) ImportStif(ctx context.Context, req *connect.Request[appv1.ImportStifRequest]) (*connect.Response[appv1.ImportStifResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	if req.Msg.GetSource() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source is required"))
	}
	if len(req.Msg.GetSource()) > maxStifSourceBytes {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("source is %d bytes; the limit is %d", len(req.Msg.GetSource()), maxStifSourceBytes))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}

	result, err := h.service.ImportStif(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetSource(), userID, req.Msg.GetDryRun())
	if err != nil {
		return nil, toError(err)
	}

	// A document the user still has to fix is a normal response carrying
	// diagnostics, not an RPC error: the import UI renders every problem at
	// once, and an error code would only carry the first one.
	return connect.NewResponse(&appv1.ImportStifResponse{
		Committed:     result.Committed,
		Mode:          result.Mode,
		Items:         toProtoStifItems(result.Items),
		Diagnostics:   toProtoStifDiagnostics(result.Diagnostics),
		ImportedCount: int32(result.ImportedCount),
	}), nil
}

func toProtoStifItems(items []application.ImportedItem) []*appv1.StifParsedItem {
	out := make([]*appv1.StifParsedItem, 0, len(items))
	for _, item := range items {
		out = append(out, &appv1.StifParsedItem{
			LocalPath:       item.LocalPath,
			ParentLocalPath: item.ParentLocalPath,
			Title:           item.Title,
			Description:     item.Description,
			Content:         item.Content,
			OverrideCss:     item.OverrideCSS,
			GovernanceState: domain.ItemGovernanceStateFromName(item.State),
			Scope:           domain.TreeProjectionScopeFromName(item.Scope),
			CrossDocument:   item.CrossDocument,
			Order:           int32(item.Order),
			Sources:         toProtoStifSources(item.Sources),
			ItemId:          item.ItemID,
			Depth:           int32(item.Depth),
		})
	}
	return out
}

func toProtoStifSources(sources []stif.Source) []*appv1.ItemSourceRef {
	out := make([]*appv1.ItemSourceRef, 0, len(sources))
	for _, source := range sources {
		out = append(out, &appv1.ItemSourceRef{
			Type:       source.Type,
			Url:        source.URL,
			Repo:       source.Repo,
			Ref:        source.Ref,
			Path:       source.Path,
			LineStart:  int32(source.LineStart),
			LineEnd:    int32(source.LineEnd),
			Kind:       source.Kind,
			ExternalId: source.ExternalID,
			Title:      source.Title,
			Confidence: source.Confidence,
		})
	}
	return out
}

func toProtoStifDiagnostics(diags []stif.Diagnostic) []*appv1.StifDiagnostic {
	out := make([]*appv1.StifDiagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, &appv1.StifDiagnostic{
			Severity:  toProtoStifSeverity(d.Severity),
			Line:      int32(d.Line),
			Code:      d.Code,
			Message:   d.Message,
			LocalPath: d.LocalPath,
		})
	}
	return out
}

func toProtoStifSeverity(severity stif.Severity) appv1.StifDiagnosticSeverity {
	switch severity {
	case stif.SeverityWarning:
		return appv1.StifDiagnosticSeverity_STIF_DIAGNOSTIC_SEVERITY_WARNING
	case stif.SeverityError:
		return appv1.StifDiagnosticSeverity_STIF_DIAGNOSTIC_SEVERITY_ERROR
	default:
		return appv1.StifDiagnosticSeverity_STIF_DIAGNOSTIC_SEVERITY_UNSPECIFIED
	}
}

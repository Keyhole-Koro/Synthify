package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func (h *DocumentHandler) DeleteDocument(ctx context.Context, req *connect.Request[appv1.DeleteDocumentRequest]) (*connect.Response[appv1.DeleteDocumentResponse], error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	if err := h.service.DeleteDocument(ctx, req.Msg.GetDocumentId(), userID); err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.DeleteDocumentResponse{}), nil
}

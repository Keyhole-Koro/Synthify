package handler

import (
	"context"
	"errors"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/internal/middleware"
	"github.com/synthify/backend/internal/repository/mock"
)

// assertConnectCode fails the test if err is nil or does not carry the expected connect code.
func assertConnectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %v, got nil", want)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	if ce.Code() != want {
		t.Errorf("connect code = %v, want %v", ce.Code(), want)
	}
}

// ── currentUser ──────────────────────────────────────────────────────────────

func TestCurrentUser_NoAuthInContext_ReturnsUnauthenticated(t *testing.T) {
	_, err := currentUser(context.Background())
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestCurrentUser_EmptyUserID_ReturnsUnauthenticated(t *testing.T) {
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "", Email: "x@y.com"})
	_, err := currentUser(ctx)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestCurrentUser_ValidUser_ReturnsUser(t *testing.T) {
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "u1", Email: "u@example.com"})
	user, err := currentUser(ctx)
	if err != nil {
		t.Fatalf("currentUser: unexpected error: %v", err)
	}
	if user.ID != "u1" {
		t.Errorf("user.ID = %q, want u1", user.ID)
	}
}

// ── authorizeWorkspace ────────────────────────────────────────────────────────

func TestAuthorizeWorkspace_Unauthenticated_ReturnsUnauthenticated(t *testing.T) {
	store := mock.NewStore()
	err := authorizeWorkspace(context.Background(), store, "any_ws")
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestAuthorizeWorkspace_NotMember_ReturnsPermissionDenied(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "stranger", Email: "s@example.com"})

	err := authorizeWorkspace(ctx, store, ws.WorkspaceID)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestAuthorizeWorkspace_Member_ReturnsNil(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "owner", Email: "o@example.com"})

	if err := authorizeWorkspace(ctx, store, ws.WorkspaceID); err != nil {
		t.Errorf("authorizeWorkspace: unexpected error: %v", err)
	}
}

// ── authorizeDocument ─────────────────────────────────────────────────────────

func TestAuthorizeDocument_DocumentNotFound_ReturnsNotFound(t *testing.T) {
	store := mock.NewStore()
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "u1", Email: "u@example.com"})

	err := authorizeDocument(ctx, store, store, "nonexistent_doc", "")
	assertConnectCode(t, err, connect.CodeNotFound)
}

func TestAuthorizeDocument_WrongWorkspace_ReturnsPermissionDenied(t *testing.T) {
	store := mock.NewStore()
	ws1 := store.CreateWorkspace("ws1", "owner", "o@example.com")
	ws2 := store.CreateWorkspace("ws2", "owner", "o@example.com")
	doc, _ := store.CreateDocument(ws1.WorkspaceID, "f.pdf", "application/pdf", 100)
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "owner", Email: "o@example.com"})

	err := authorizeDocument(ctx, store, store, doc.DocumentID, ws2.WorkspaceID)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestAuthorizeDocument_NotMember_ReturnsPermissionDenied(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")
	doc, _ := store.CreateDocument(ws.WorkspaceID, "f.pdf", "application/pdf", 100)
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "stranger", Email: "s@example.com"})

	err := authorizeDocument(ctx, store, store, doc.DocumentID, "")
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestAuthorizeDocument_Member_ReturnsNil(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")
	doc, _ := store.CreateDocument(ws.WorkspaceID, "f.pdf", "application/pdf", 100)
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "owner", Email: "o@example.com"})

	if err := authorizeDocument(ctx, store, store, doc.DocumentID, ""); err != nil {
		t.Errorf("authorizeDocument: unexpected error: %v", err)
	}
}

// ── authorizeNode ─────────────────────────────────────────────────────────────

func TestAuthorizeNode_NodeNotFound_ReturnsNotFound(t *testing.T) {
	store := mock.NewStore()
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "u1", Email: "u@example.com"})

	err := authorizeNode(ctx, store, store, store, "nonexistent_node", "")
	assertConnectCode(t, err, connect.CodeNotFound)
}

func TestAuthorizeNode_ValidNode_AuthorizesViaDocument(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")
	doc, _ := store.CreateDocument(ws.WorkspaceID, "f.pdf", "application/pdf", 100)
	store.StartProcessing(doc.DocumentID, false, "full")
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "owner", Email: "o@example.com"})

	if err := authorizeNode(ctx, store, store, store, "nd_root", ""); err != nil {
		t.Errorf("authorizeNode: unexpected error: %v", err)
	}
}

func TestAuthorizeNode_NotMember_ReturnsPermissionDenied(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")
	doc, _ := store.CreateDocument(ws.WorkspaceID, "f.pdf", "application/pdf", 100)
	store.StartProcessing(doc.DocumentID, false, "full")
	ctx := middleware.ContextWithUser(context.Background(), middleware.AuthUser{ID: "stranger", Email: "s@example.com"})

	err := authorizeNode(ctx, store, store, store, "nd_root", "")
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

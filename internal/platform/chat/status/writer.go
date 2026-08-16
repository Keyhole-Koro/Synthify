// Package chatstatus writes dialogue turn documents to Firestore, which is how
// a generated answer reaches the browser.
//
// It deliberately does not reuse the job-status notifier. Turns live under
// users/{uid}/chats/{chatId}/turns/{turnId} rather than under a workspace, so
// the security rules can authorize with request.auth.uid alone and need no
// mirror of the SQL membership semantics. The document shape and lifecycle
// differ too, so sharing the notifier would mean one type serving two
// unrelated schemas.
package chatstatus

import (
	"context"
	"log/slog"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
)

// Ref identifies one turn document.
type Ref struct {
	UserID      string
	WorkspaceID string
	ChatID      string
	TurnID      string
	PaperID     string
}

// Usage accumulates across the section calls that make up one turn.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// Writer publishes turn state. The worker holds one of these for the process.
type Writer interface {
	// Start creates the document in the running state so a browser that
	// subscribes before generation begins finds something to read.
	Start(ctx context.Context, ref Ref) error
	// Progress overwrites content with everything produced so far. It is
	// called once per section, never per token: Firestore sustains roughly
	// one write per second per document.
	Progress(ctx context.Context, ref Ref, contentJSON string, sectionCount int) error
	// Succeed finalizes the turn. commandsJSON and proposalsJSON are written
	// once here rather than per section, so the canvas does not move while
	// the answer is still being assembled.
	Succeed(ctx context.Context, ref Ref, res Success) error
	// Fail marks the turn failed, keeping whatever content was produced.
	Fail(ctx context.Context, ref Ref, f Failure) error
}

type Success struct {
	ContentJSON        string
	CommandsJSON       string
	ProposalsJSON      string
	SelectedContextIDs []string
	SectionCount       int
	FinishReason       FirestoreChatTurnFinishReason
	Usage              Usage
}

type Failure struct {
	ContentJSON  string
	SectionCount int
	ErrorMessage string
	Reason       FirestoreChatTurnReason
	Usage        Usage
}

// noopWriter stands in when Firestore is not configured, which is the case for
// local runs without a Firebase project. Generation still runs; the answer just
// has nowhere to go.
type noopWriter struct{}

func (noopWriter) Start(context.Context, Ref) error                 { return nil }
func (noopWriter) Progress(context.Context, Ref, string, int) error { return nil }
func (noopWriter) Succeed(context.Context, Ref, Success) error      { return nil }
func (noopWriter) Fail(context.Context, Ref, Failure) error         { return nil }

type firestoreWriter struct {
	client *firestore.Client
	logger *slog.Logger
}

func NewWriter(ctx context.Context, projectID string, logger *slog.Logger) Writer {
	if projectID == "" {
		return noopWriter{}
	}
	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	app, err := firebase.NewApp(initCtx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		logger.Error("chatstatus.firebase_init_failed", "error", err.Error(), "project_id", projectID)
		return noopWriter{}
	}
	client, err := app.Firestore(initCtx)
	if err != nil {
		logger.Error("chatstatus.firestore_init_failed", "error", err.Error(), "project_id", projectID)
		return noopWriter{}
	}
	return &firestoreWriter{client: client, logger: logger}
}

func (w *firestoreWriter) Start(ctx context.Context, ref Ref) error {
	return w.write(ctx, ref, map[string]any{
		FirestoreChatTurnFieldStatus:       string(FirestoreChatTurnStateRunning),
		FirestoreChatTurnFieldContent:      "",
		FirestoreChatTurnFieldErrorMessage: "",
		FirestoreChatTurnFieldSectionCount: 0,
		FirestoreChatTurnFieldCreatedAt:    nowRFC3339(),
		FirestoreChatTurnFieldUpdatedAt:    nowRFC3339(),
	})
}

func (w *firestoreWriter) Progress(ctx context.Context, ref Ref, contentJSON string, sectionCount int) error {
	return w.write(ctx, ref, map[string]any{
		FirestoreChatTurnFieldStatus:       string(FirestoreChatTurnStateRunning),
		FirestoreChatTurnFieldContent:      contentJSON,
		FirestoreChatTurnFieldErrorMessage: "",
		FirestoreChatTurnFieldSectionCount: sectionCount,
		FirestoreChatTurnFieldUpdatedAt:    nowRFC3339(),
	})
}

func (w *firestoreWriter) Succeed(ctx context.Context, ref Ref, res Success) error {
	fields := map[string]any{
		FirestoreChatTurnFieldStatus:           string(FirestoreChatTurnStateSucceeded),
		FirestoreChatTurnFieldContent:          res.ContentJSON,
		FirestoreChatTurnFieldErrorMessage:     "",
		FirestoreChatTurnFieldSectionCount:     res.SectionCount,
		FirestoreChatTurnFieldPromptTokens:     res.Usage.PromptTokens,
		FirestoreChatTurnFieldCompletionTokens: res.Usage.CompletionTokens,
		FirestoreChatTurnFieldUpdatedAt:        nowRFC3339(),
		FirestoreChatTurnFieldCompletedAt:      nowRFC3339(),
		// Only terminal turns carry an expiry; a running one must not be
		// collected out from under the browser watching it.
		FirestoreChatTurnFieldExpiresAt: ttlFromNow(),
	}
	if res.CommandsJSON != "" {
		fields[FirestoreChatTurnFieldCommands] = res.CommandsJSON
	}
	if res.ProposalsJSON != "" {
		fields[FirestoreChatTurnFieldProposals] = res.ProposalsJSON
	}
	if len(res.SelectedContextIDs) > 0 {
		fields[FirestoreChatTurnFieldSelectedContextIDs] = res.SelectedContextIDs
	}
	if res.FinishReason != "" {
		fields[FirestoreChatTurnFieldFinishReason] = string(res.FinishReason)
	}
	return w.write(ctx, ref, fields)
}

func (w *firestoreWriter) Fail(ctx context.Context, ref Ref, f Failure) error {
	fields := map[string]any{
		FirestoreChatTurnFieldStatus: string(FirestoreChatTurnStateFailed),
		// Whatever was produced before the failure is kept: a partial answer
		// is more useful than a blank one, and the status already says it is
		// incomplete.
		FirestoreChatTurnFieldContent:          f.ContentJSON,
		FirestoreChatTurnFieldErrorMessage:     f.ErrorMessage,
		FirestoreChatTurnFieldSectionCount:     f.SectionCount,
		FirestoreChatTurnFieldPromptTokens:     f.Usage.PromptTokens,
		FirestoreChatTurnFieldCompletionTokens: f.Usage.CompletionTokens,
		FirestoreChatTurnFieldUpdatedAt:        nowRFC3339(),
		FirestoreChatTurnFieldCompletedAt:      nowRFC3339(),
		FirestoreChatTurnFieldExpiresAt:        ttlFromNow(),
	}
	if f.Reason != "" {
		fields[FirestoreChatTurnFieldReason] = string(f.Reason)
	}
	return w.write(ctx, ref, fields)
}

func (w *firestoreWriter) write(ctx context.Context, ref Ref, fields map[string]any) error {
	if ref.UserID == "" || ref.ChatID == "" || ref.TurnID == "" {
		return nil
	}
	doc := map[string]any{
		FirestoreChatTurnFieldChatID:      ref.ChatID,
		FirestoreChatTurnFieldTurnID:      ref.TurnID,
		FirestoreChatTurnFieldWorkspaceID: ref.WorkspaceID,
		FirestoreChatTurnFieldPaperID:     ref.PaperID,
	}
	for key, value := range fields {
		doc[key] = value
	}
	if err := validateFirestoreChatTurn(doc); err != nil {
		w.logger.Error("chatstatus.schema_validation_failed",
			"error", err.Error(), "chat_id", ref.ChatID, "turn_id", ref.TurnID)
		return err
	}
	_, err := w.turnDoc(ref).Set(ctx, doc, firestore.MergeAll)
	if err != nil {
		w.logger.Error("chatstatus.write_failed",
			"error", err.Error(), "chat_id", ref.ChatID, "turn_id", ref.TurnID)
		return err
	}
	return nil
}

func (w *firestoreWriter) turnDoc(ref Ref) *firestore.DocumentRef {
	return w.client.
		Collection("users").Doc(ref.UserID).
		Collection("chats").Doc(ref.ChatID).
		Collection("turns").Doc(ref.TurnID)
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// chatTurnTTL is how long a finished turn survives before the Firestore TTL
// policy collects it. Longer than the job-status window: a finished job is
// spent once its outcome has been seen, but a conversation gets re-read. The
// expiresAt field is bound to the TTL policy in terraform; changing this only
// affects documents written afterwards.
const chatTurnTTL = 30 * 24 * time.Hour

func ttlFromNow() time.Time {
	return time.Now().UTC().Add(chatTurnTTL)
}

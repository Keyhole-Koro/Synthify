package domain

// ChatRole distinguishes who produced a message in the dialogue history.
type ChatRole string

const (
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
)

// ChatMessage is one entry of the conversation history. History is resent by
// the browser on every turn rather than persisted server-side, so this type
// only ever exists in transit.
type ChatMessage struct {
	Role ChatRole
	Text string
}

// ContextPaper is a candidate paper the model may ground its answer in. The
// browser collects these because it is the only side that knows what the user
// currently has open on the canvas.
type ContextPaper struct {
	PaperID     string
	Title       string
	Description string
	// Plain text: paper content may be rich HTML, a ContentNode tree or live
	// JSX, and the browser flattens it before sending.
	Content string
}

// ChatTurnRequest is what the API hands the worker to produce one turn.
//
// UserID is carried explicitly because the Firestore turn document lives under
// users/{uid}/..., so the worker needs the owner to address the write. That
// path is what lets the security rules authorize with request.auth.uid alone;
// workspace authorization has already happened in the handler.
type ChatTurnRequest struct {
	UserID      string
	WorkspaceID string
	ChatID      string
	TurnID      string
	PaperID     string

	History           []ChatMessage
	Candidates        []ContextPaper
	PinnedContextIDs  []string
	AutoSelectContext bool
}

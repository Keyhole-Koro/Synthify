package chatstatus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runningDoc() map[string]any {
	return map[string]any{
		FirestoreChatTurnFieldChatID:       "chat-1",
		FirestoreChatTurnFieldTurnID:       "turn-1",
		FirestoreChatTurnFieldWorkspaceID:  "ws-1",
		FirestoreChatTurnFieldPaperID:      "paper-1",
		FirestoreChatTurnFieldStatus:       string(FirestoreChatTurnStateRunning),
		FirestoreChatTurnFieldContent:      "",
		FirestoreChatTurnFieldErrorMessage: "",
		FirestoreChatTurnFieldSectionCount: 0,
		FirestoreChatTurnFieldUpdatedAt:    nowRFC3339(),
	}
}

func TestValidate_AcceptsRunningTurn(t *testing.T) {
	require.NoError(t, validateFirestoreChatTurn(runningDoc()))
}

// A required string left empty must still validate: content is empty while the
// first section is generating, and errorMessage is empty unless the turn fails.
func TestValidate_AcceptsEmptyRequiredStrings(t *testing.T) {
	doc := runningDoc()
	doc[FirestoreChatTurnFieldContent] = ""
	doc[FirestoreChatTurnFieldErrorMessage] = ""

	require.NoError(t, validateFirestoreChatTurn(doc))
}

// expiresAt is written as a native time.Time so the Firestore TTL field
// selector can match it; the validator only understands JSON kinds.
func TestValidate_AcceptsTimeExpiresAt(t *testing.T) {
	doc := runningDoc()
	doc[FirestoreChatTurnFieldStatus] = string(FirestoreChatTurnStateSucceeded)
	doc[FirestoreChatTurnFieldExpiresAt] = ttlFromNow()

	require.NoError(t, validateFirestoreChatTurn(doc))
}

func TestValidate_RejectsUnknownStatus(t *testing.T) {
	doc := runningDoc()
	// "queued" is a job-status state. Chat dispatches synchronously and never
	// observes one, so the enum omits it.
	doc[FirestoreChatTurnFieldStatus] = "queued"

	require.Error(t, validateFirestoreChatTurn(doc))
}

func TestValidate_RejectsMissingRequiredField(t *testing.T) {
	doc := runningDoc()
	delete(doc, FirestoreChatTurnFieldPaperID)

	require.Error(t, validateFirestoreChatTurn(doc))
}

// additionalProperties is false, so a typo becomes an error here rather than a
// silently ignored field the browser never sees.
func TestValidate_RejectsUnknownField(t *testing.T) {
	doc := runningDoc()
	doc["contnet"] = "typo"

	require.Error(t, validateFirestoreChatTurn(doc))
}

func TestValidate_AcceptsSectionCapFinishReason(t *testing.T) {
	doc := runningDoc()
	doc[FirestoreChatTurnFieldStatus] = string(FirestoreChatTurnStateSucceeded)
	doc[FirestoreChatTurnFieldFinishReason] = string(FirestoreChatTurnFinishReasonSectionCap)

	require.NoError(t, validateFirestoreChatTurn(doc))
}

// The doc handed to Firestore must keep its real values; only the copy given to
// the validator is normalized.
func TestValidatableDoc_DoesNotMutateInput(t *testing.T) {
	expires := time.Now().UTC()
	doc := map[string]any{
		FirestoreChatTurnFieldContent:   "",
		FirestoreChatTurnFieldExpiresAt: expires,
	}

	got := validatableDoc(doc)

	assert.Equal(t, "", doc[FirestoreChatTurnFieldContent], "original empty string preserved")
	assert.Equal(t, expires, doc[FirestoreChatTurnFieldExpiresAt], "original time.Time preserved")
	assert.Equal(t, "__empty__", got[FirestoreChatTurnFieldContent])
	assert.Equal(t, expires.Format(time.RFC3339), got[FirestoreChatTurnFieldExpiresAt])
}

package chatstatus

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
)

var (
	firestoreChatTurnSchemaOnce sync.Once
	firestoreChatTurnSchema     *jsonschema.Resolved
	firestoreChatTurnSchemaErr  error
)

// validateFirestoreChatTurn checks a document against the generated schema
// before it reaches Firestore, so a shape mistake surfaces here rather than as
// a browser-side parse failure on a document nothing can fix.
func validateFirestoreChatTurn(doc map[string]any) error {
	schema, err := resolvedFirestoreChatTurnSchema()
	if err != nil {
		return err
	}
	// Validate the JSON representation rather than the Go map: the validator
	// treats zero-value map entries as absent during reflection.
	raw, err := json.Marshal(validatableDoc(doc))
	if err != nil {
		return fmt.Errorf("marshal firestore chat turn: %w", err)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("decode firestore chat turn: %w", err)
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("firestore chat turn schema validation failed: %w", err)
	}
	return nil
}

// validatableDoc normalizes values the JSON-schema validator cannot introspect.
// Mirrors the job-status helper for the same two reasons: expiresAt is written
// as a native time.Time so the TTL field selector can match it, and the
// validator reports a required string with an empty value as missing.
//
// The original doc — time.Time and empty strings intact — is what gets written.
func validatableDoc(doc map[string]any) map[string]any {
	out := make(map[string]any, len(doc))
	for k, v := range doc {
		if t, ok := v.(time.Time); ok {
			out[k] = t.UTC().Format(time.RFC3339)
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			out[k] = "__empty__"
			continue
		}
		out[k] = v
	}
	return out
}

func resolvedFirestoreChatTurnSchema() (*jsonschema.Resolved, error) {
	firestoreChatTurnSchemaOnce.Do(func() {
		var schema jsonschema.Schema
		if err := json.Unmarshal([]byte(FirestoreChatTurnJSONSchema), &schema); err != nil {
			firestoreChatTurnSchemaErr = fmt.Errorf("parse firestore chat turn schema: %w", err)
			return
		}
		firestoreChatTurnSchema, firestoreChatTurnSchemaErr = schema.Resolve(nil)
	})
	return firestoreChatTurnSchema, firestoreChatTurnSchemaErr
}

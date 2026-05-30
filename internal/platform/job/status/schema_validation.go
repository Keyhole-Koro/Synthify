package jobstatus

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
)

var (
	firestoreJobStatusSchemaOnce sync.Once
	firestoreJobStatusSchema     *jsonschema.Resolved
	firestoreJobStatusSchemaErr  error
)

func validateFirestoreJobStatus(doc map[string]any) error {
	schema, err := resolvedFirestoreJobStatusSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(validatableDoc(doc)); err != nil {
		return fmt.Errorf("firestore job status schema validation failed: %w", err)
	}
	return nil
}

// validatableDoc returns a copy of doc with values the JSON-schema validator
// cannot introspect normalized into ones it can. Notably expiresAt is written
// as a native time.Time (a Go struct) so the Firestore TTL policy can match it,
// but the validator only understands JSON kinds (object/string/number/...) and
// rejects a bare struct with "cannot validate against a struct". We hand the
// validator an RFC3339 string for any time.Time; the original doc — with the
// time.Time intact — is what actually gets written to Firestore.
func validatableDoc(doc map[string]any) map[string]any {
	out := make(map[string]any, len(doc))
	for k, v := range doc {
		if t, ok := v.(time.Time); ok {
			out[k] = t.UTC().Format(time.RFC3339)
			continue
		}
		out[k] = v
	}
	return out
}

func resolvedFirestoreJobStatusSchema() (*jsonschema.Resolved, error) {
	firestoreJobStatusSchemaOnce.Do(func() {
		var schema jsonschema.Schema
		if err := json.Unmarshal([]byte(FirestoreJobStatusJSONSchema), &schema); err != nil {
			firestoreJobStatusSchemaErr = fmt.Errorf("parse firestore job status schema: %w", err)
			return
		}
		firestoreJobStatusSchema, firestoreJobStatusSchemaErr = schema.Resolve(nil)
	})
	return firestoreJobStatusSchema, firestoreJobStatusSchemaErr
}

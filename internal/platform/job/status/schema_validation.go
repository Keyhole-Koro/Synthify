package jobstatus

import (
	"encoding/json"
	"fmt"
	"sync"

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
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("firestore job status schema validation failed: %w", err)
	}
	return nil
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

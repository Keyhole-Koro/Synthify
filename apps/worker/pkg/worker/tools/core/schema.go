package core

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// ResolvedSchemaFor produces a resolved JSON schema for the Go type T. Used by
// builtin tools that want their schema derived from their Args/Result struct
// tags (single source of truth for the io_schema).
func ResolvedSchemaFor[T any]() (*jsonschema.Resolved, error) {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, err
	}
	return s.Resolve(nil)
}

// IOSchemaFor builds an IOSchema from two Go types — the standard pattern for
// every builtin tool. Both schemas are resolved once at construction.
func IOSchemaFor[Input, Output any]() (IOSchema, error) {
	in, err := ResolvedSchemaFor[Input]()
	if err != nil {
		return IOSchema{}, fmt.Errorf("input schema: %w", err)
	}
	out, err := ResolvedSchemaFor[Output]()
	if err != nil {
		return IOSchema{}, fmt.Errorf("output schema: %w", err)
	}
	return IOSchema{Input: in, Output: out}, nil
}

// IOSchemaFromJSON parses a dynamic tool's io_schema JSON string into an
// IOSchema. The convention adopted by this codebase is that the string is a
// single JSON Schema with two top-level properties named "input" and
// "output", each itself a JSON Schema. create_transform persists schemas in
// this shape; newDynamicTool consumes them through this function.
//
// This convention is documented here rather than in domain.DynamicTool
// because the wrapper shape is an eval-time concern; the DB column stays a
// free-form schema string.
func IOSchemaFromJSON(raw string) (IOSchema, error) {
	var wrapper struct {
		Input  json.RawMessage `json:"input"`
		Output json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return IOSchema{}, fmt.Errorf("parse io_schema wrapper: %w", err)
	}
	if len(wrapper.Input) == 0 || len(wrapper.Output) == 0 {
		return IOSchema{}, fmt.Errorf("io_schema wrapper missing 'input' or 'output'")
	}
	in, err := resolveRawSchema(wrapper.Input)
	if err != nil {
		return IOSchema{}, fmt.Errorf("input schema: %w", err)
	}
	out, err := resolveRawSchema(wrapper.Output)
	if err != nil {
		return IOSchema{}, fmt.Errorf("output schema: %w", err)
	}
	return IOSchema{Input: in, Output: out}, nil
}

func resolveRawSchema(raw json.RawMessage) (*jsonschema.Resolved, error) {
	var s jsonschema.Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return s.Resolve(nil)
}

// ValidateRaw checks that a JSON document conforms to a resolved schema.
// Callers use it for both input validation (before tool execution) and output
// validation (after). A nil schema is a programming error here, not a soft
// pass — every Tool is required to carry an IOSchema.
func ValidateRaw(schema *jsonschema.Resolved, raw json.RawMessage) error {
	if schema == nil {
		return fmt.Errorf("missing schema")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	return schema.Validate(v)
}

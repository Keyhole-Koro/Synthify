package core

import (
	"context"
	"encoding/json"
	"testing"
)

type coreInput struct {
	Name string `json:"name"`
}

type coreOutput struct {
	Count int `json:"count"`
}

func TestIOSchemaForAndValidateRaw(t *testing.T) {
	schema, err := IOSchemaFor[coreInput, coreOutput]()
	if err != nil {
		t.Fatalf("IOSchemaFor() error = %v", err)
	}
	if err := ValidateRaw(schema.Input, json.RawMessage(`{"name":"ok"}`)); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if err := ValidateRaw(schema.Input, json.RawMessage(`{"count":1}`)); err == nil {
		t.Fatal("invalid input accepted")
	}
	if err := ValidateRaw(nil, json.RawMessage(`{}`)); err == nil {
		t.Fatal("nil schema accepted")
	}
}

func TestIOSchemaFromJSON(t *testing.T) {
	raw := `{"input":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]},"output":{"type":"object","properties":{"count":{"type":"integer"}}}}`
	schema, err := IOSchemaFromJSON(raw)
	if err != nil {
		t.Fatalf("IOSchemaFromJSON() error = %v", err)
	}
	if err := ValidateRaw(schema.Output, json.RawMessage(`{"count":2}`)); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}
	if _, err := IOSchemaFromJSON(`{"input":{}}`); err == nil {
		t.Fatal("missing output schema accepted")
	}
	if _, err := IOSchemaFromJSON("not-json"); err == nil {
		t.Fatal("malformed wrapper accepted")
	}
}

func TestResolveTool(t *testing.T) {
	run := func(context.Context, json.RawMessage) (json.RawMessage, Usage, error) {
		return json.RawMessage(`{"ok":true}`), Usage{Model: "test"}, nil
	}
	table := map[string]Tool{"known": {Name: "known", Run: run}}
	got, err := ResolveTool(table, "known")
	if err != nil || got.Name != "known" {
		t.Fatalf("known tool = %#v, error = %v", got, err)
	}
	if _, err := ResolveTool(table, "missing"); err == nil || err.Error() != "unsupported tool: missing" {
		t.Fatalf("missing tool error = %v", err)
	}
}

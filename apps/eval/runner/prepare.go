package runner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/synthify/backend/packages/shared/domain"
	"gopkg.in/yaml.v3"
)

type preparedInput struct {
	Raw      json.RawMessage
	Snapshot *InputSnapshot
}

// UnmarshalYAML preserves case input as JSON. Fixture resolution is a prepare
// step, not tool execution: the runner keeps the tool body as input JSON ->
// output JSON, while this layer turns YAML conveniences such as chunks paths
// into the JSON payload the tool actually receives.
func (c *Case) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Name   string     `yaml:"name"`
		Tool   string     `yaml:"tool"`
		Input  any        `yaml:"input"`
		Expect CaseExpect `yaml:"expect"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	input, err := json.Marshal(normalizeYAMLValue(raw.Input))
	if err != nil {
		return fmt.Errorf("marshal case input as JSON: %w", err)
	}
	c.Name = raw.Name
	c.Tool = raw.Tool
	c.Input = input
	c.Expect = raw.Expect
	return nil
}

func normalizeYAMLValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[k] = normalizeYAMLValue(v)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[fmt.Sprint(k)] = normalizeYAMLValue(v)
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, v := range x {
			a[i] = normalizeYAMLValue(v)
		}
		return a
	default:
		return x
	}
}

func prepareToolInput(c Case, baseDir string) (preparedInput, error) {
	switch c.Tool {
	case ToolKnowledgeTree:
		return prepareKnowledgeTreeInput(c.Input, baseDir)
	default:
		return preparedInput{Raw: c.Input}, nil
	}
}

func prepareKnowledgeTreeInput(raw json.RawMessage, baseDir string) (preparedInput, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return preparedInput{}, fmt.Errorf("parse knowledge_tree input: %w", err)
	}
	documentID, _ := obj["document_id"].(string)
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return preparedInput{}, fmt.Errorf("knowledge_tree input.document_id is required")
	}
	instruction, _ := obj["instruction"].(string)

	chunksValue, ok := obj["chunks"]
	if !ok {
		return preparedInput{}, fmt.Errorf("knowledge_tree input.chunks is required")
	}

	var chunksPath string
	var chunks []domain.Chunk
	if path, ok := chunksValue.(string); ok {
		chunksPath = strings.TrimSpace(path)
		if chunksPath == "" {
			return preparedInput{}, fmt.Errorf("knowledge_tree input.chunks path is required")
		}
		resolved := resolvePath(baseDir, chunksPath)
		loaded, err := LoadChunks(resolved)
		if err != nil {
			return preparedInput{}, err
		}
		chunks = loaded
		obj["chunks"] = chunks
	} else {
		b, err := json.Marshal(chunksValue)
		if err != nil {
			return preparedInput{}, fmt.Errorf("marshal knowledge_tree chunks: %w", err)
		}
		if err := json.Unmarshal(b, &chunks); err != nil {
			return preparedInput{}, fmt.Errorf("parse knowledge_tree chunks: %w", err)
		}
	}

	prepared, err := json.Marshal(obj)
	if err != nil {
		return preparedInput{}, fmt.Errorf("marshal prepared knowledge_tree input: %w", err)
	}
	return preparedInput{
		Raw: prepared,
		Snapshot: &InputSnapshot{
			DocumentID:  documentID,
			Instruction: instruction,
			ChunksPath:  filepath.ToSlash(chunksPath),
			Chunks:      chunks,
		},
	}, nil
}

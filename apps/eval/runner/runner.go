package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
	"gopkg.in/yaml.v3"
)

const ToolKnowledgeTree = "knowledge_tree"

type Case struct {
	Name   string          `yaml:"name" json:"name"`
	Tool   string          `yaml:"tool" json:"tool"`
	Input  json.RawMessage `yaml:"input" json:"input"`
	Expect CaseExpect      `yaml:"expect" json:"expect"`
}

type Result struct {
	CaseName     string          `json:"case_name"`
	Tool         string          `json:"tool"`
	Passed       bool            `json:"passed"`
	SchemaValid  bool            `json:"schema_valid"`
	Output       json.RawMessage `json:"output,omitempty"`
	DurationMS   int64           `json:"duration_ms"`
	Model        string          `json:"model"`
	InputTokens  int64           `json:"input_tokens"`
	OutputTokens int64           `json:"output_tokens"`
	Error        string          `json:"error,omitempty"`
	FailedInput  *InputSnapshot  `json:"failed_input,omitempty"`
	PromptSource string          `json:"prompt_source"`
}

type InputSnapshot struct {
	DocumentID  string         `json:"document_id"`
	Instruction string         `json:"instruction,omitempty"`
	ChunksPath  string         `json:"chunks_path,omitempty"`
	Chunks      []domain.Chunk `json:"chunks"`
}

type Runner struct {
	LLM base.LLMClient

	// Renderer renders the knowledge_tree prompt. Nil means the production
	// embedded prompt. A non-nil renderer is a variant.
	Renderer *prompts.Renderer

	// PromptSource labels the report: "production" or "variant:{name}".
	PromptSource string

	// DynTools and TransformEngine are the dynamic-tool seam. Dynamic execution
	// is stubbed in this phase, but the runner resolves every tool through the
	// same Tool abstraction.
	DynTools        DynamicToolSource
	TransformEngine transform.Engine
}

func CaseFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat cases path %s: %w", path, err)
	}
	if !info.IsDir() {
		return []string{path}, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read cases dir %s: %w", path, err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(path, entry.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no YAML case files found in %s", path)
	}
	return files, nil
}

// resolveRenderer returns the prompt renderer for this run. A non-nil
// r.Renderer is a variant; otherwise the cached embedded production renderer
// is used. Resolving once per run keeps multi-case throughput flat.
func (r Runner) resolveRenderer() (*prompts.Renderer, string, error) {
	promptSource := r.PromptSource
	if promptSource == "" {
		promptSource = "production"
	}
	if r.Renderer != nil {
		return r.Renderer, promptSource, nil
	}
	p, err := prompts.Default()
	if err != nil {
		return nil, "", fmt.Errorf("load production prompts: %w", err)
	}
	return p, promptSource, nil
}

func (r Runner) RunCaseFile(ctx context.Context, path string) (Result, error) {
	c, err := LoadCase(path)
	if err != nil {
		return Result{}, err
	}
	renderer, promptSource, err := r.resolveRenderer()
	if err != nil {
		return Result{}, err
	}
	tools, err := r.toolTable(renderer)
	if err != nil {
		return Result{}, err
	}
	return r.runCase(ctx, c, filepath.Dir(path), promptSource, tools)
}

func (r Runner) RunCaseFiles(ctx context.Context, paths []string) ([]Result, error) {
	renderer, promptSource, err := r.resolveRenderer()
	if err != nil {
		return nil, err
	}
	tools, err := r.toolTable(renderer)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(paths))
	for _, path := range paths {
		c, err := LoadCase(path)
		if err != nil {
			return nil, err
		}
		res, err := r.runCase(ctx, c, filepath.Dir(path), promptSource, tools)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

// RunCase resolves the renderer and runs a single case. Prefer RunCaseFiles
// for multi-case runs so the renderer and tool table are built once.
func (r Runner) RunCase(ctx context.Context, c Case, baseDir string) (Result, error) {
	renderer, promptSource, err := r.resolveRenderer()
	if err != nil {
		return Result{}, err
	}
	tools, err := r.toolTable(renderer)
	if err != nil {
		return Result{}, err
	}
	return r.runCase(ctx, c, baseDir, promptSource, tools)
}

// toolTable builds the per-run Tool lookup. The runner does not know which
// concrete builtin tools exist — that enumeration lives in builtins.go.
// Dynamic tools are merged in on-demand by resolveRunTool (see DynamicToolSource).
func (r Runner) toolTable(renderer *prompts.Renderer) (map[string]Tool, error) {
	return builtinTable(r.LLM, renderer), nil
}

func (r Runner) runCase(ctx context.Context, c Case, baseDir string, promptSource string, tools map[string]Tool) (Result, error) {
	if strings.TrimSpace(c.Name) == "" {
		return Result{}, fmt.Errorf("case name is required")
	}
	tool, err := r.resolveRunTool(ctx, tools, c.Tool)
	if err != nil {
		return Result{}, err
	}

	prepared, err := prepareToolInput(c, baseDir)
	if err != nil {
		return Result{}, err
	}
	if err := validateRaw(tool.IOSchema.Input, prepared.Raw); err != nil {
		return Result{}, fmt.Errorf("%s input schema: %w", c.Tool, err)
	}

	res := Result{CaseName: c.Name, Tool: c.Tool, PromptSource: promptSource}
	start := time.Now()
	output, usage, toolErr := tool.Run(ctx, prepared.Raw)
	res.DurationMS = time.Since(start).Milliseconds()
	res.Model = usage.Model
	res.InputTokens = usage.InputTokens
	res.OutputTokens = usage.OutputTokens
	if toolErr != nil {
		res.Error = toolErr.Error()
		res.FailedInput = prepared.Snapshot
		return res, nil
	}

	res.Output = output
	res.SchemaValid = validateRaw(tool.IOSchema.Output, output) == nil
	passed, err := evaluateExpect(c.Expect, output, res.SchemaValid)
	if err != nil {
		return Result{}, fmt.Errorf("%s expectations: %w", c.Name, err)
	}
	res.Passed = passed && res.Error == ""
	if !res.Passed {
		res.FailedInput = prepared.Snapshot
	}
	return res, nil
}

func (r Runner) resolveRunTool(ctx context.Context, tools map[string]Tool, name string) (Tool, error) {
	tool, err := resolveTool(tools, name)
	if err == nil {
		return tool, nil
	}
	if r.DynTools == nil {
		return Tool{}, err
	}
	dt, ok, dynErr := r.DynTools.Resolve(ctx, name)
	if dynErr != nil {
		return Tool{}, dynErr
	}
	if !ok {
		return Tool{}, err
	}
	tool = newDynamicTool(dt, r.TransformEngine)
	tools[name] = tool
	return tool, nil
}

func validateRaw(schema *jsonschema.Resolved, raw json.RawMessage) error {
	if schema == nil {
		return fmt.Errorf("missing schema")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	return schema.Validate(v)
}

func LoadCase(path string) (Case, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Case{}, fmt.Errorf("read case %s: %w", path, err)
	}
	var c Case
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Case{}, fmt.Errorf("parse case %s: %w", path, err)
	}
	return c, nil
}

func LoadChunks(path string) ([]domain.Chunk, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read chunks %s: %w", path, err)
	}
	var chunks []domain.Chunk
	if err := json.Unmarshal(b, &chunks); err != nil {
		return nil, fmt.Errorf("parse chunks %s: %w", path, err)
	}
	return chunks, nil
}

func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) || baseDir == "" {
		return path
	}
	return filepath.Join(baseDir, path)
}

var _ base.LLMClient = llm.Client(nil)

package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/base"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/process"
	"github.com/synthify/backend/packages/shared/domain"
	"gopkg.in/yaml.v3"
)

const ToolKnowledgeTree = "knowledge_tree"

type Case struct {
	Name   string     `yaml:"name" json:"name"`
	Tool   string     `yaml:"tool" json:"tool"`
	Input  CaseInput  `yaml:"input" json:"input"`
	Expect CaseExpect `yaml:"expect" json:"expect"`
}

type CaseInput struct {
	DocumentID  string `yaml:"document_id" json:"document_id"`
	Instruction string `yaml:"instruction" json:"instruction"`
	Chunks      string `yaml:"chunks" json:"chunks"`
}

type CaseExpect struct {
	SchemaValid       bool     `yaml:"schema_valid" json:"schema_valid"`
	MinItems          int      `yaml:"min_items" json:"min_items"`
	MaxDepth          int      `yaml:"max_depth" json:"max_depth"`
	MustContainTitles []string `yaml:"must_contain_titles" json:"must_contain_titles"`
}

type Result struct {
	CaseName     string                     `json:"case_name"`
	Tool         string                     `json:"tool"`
	Passed       bool                       `json:"passed"`
	SchemaValid  bool                       `json:"schema_valid"`
	ItemCount    int                        `json:"item_count"`
	MaxDepth     int                        `json:"max_depth"`
	MissingTitle []string                   `json:"missing_titles"`
	DurationMS   int64                      `json:"duration_ms"`
	Model        string                     `json:"model"`
	InputTokens  int64                      `json:"input_tokens"`
	OutputTokens int64                      `json:"output_tokens"`
	Items        []domain.GeneratedTreeItem `json:"items,omitempty"`
	Error        string                     `json:"error,omitempty"`
	FailedInput  *InputSnapshot             `json:"failed_input,omitempty"`

	// Contract §6: prompt source and golden judgement. Additive; absent
	// fields keep the legacy report shape when --golden is not used.
	PromptSource  string      `json:"prompt_source"`
	GoldenChecked bool        `json:"golden_checked"`
	GoldenMatch   bool        `json:"golden_match"`
	GoldenDiff    *GoldenDiff `json:"golden_diff,omitempty"`
}

// GoldenDiff summarizes mismatched strict fields. It never includes full
// "content" HTML (contract §4.1, §6).
type GoldenDiff struct {
	Fields []GoldenFieldDiff `json:"fields"`
}

type GoldenFieldDiff struct {
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type InputSnapshot struct {
	DocumentID  string         `json:"document_id"`
	Instruction string         `json:"instruction,omitempty"`
	ChunksPath  string         `json:"chunks_path"`
	Chunks      []domain.Chunk `json:"chunks"`
}

type Runner struct {
	LLM base.LLMClient

	// Renderer renders the knowledge tree prompt. Nil means the production
	// (embedded) prompt. A non-nil renderer is a variant (contract §3).
	Renderer *prompts.Renderer

	// PromptSource labels the report: "production" or "variant:{name}"
	// (contract §6).
	PromptSource string

	// GoldenDir, when set, enables strict golden judgement (contract §4).
	GoldenDir string

	// UpdateGolden, when true, writes golden files instead of judging
	// against them (contract §4.2).
	UpdateGolden bool
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
	if len(files) == 0 {
		return nil, fmt.Errorf("no YAML case files found in %s", path)
	}
	return files, nil
}

// resolveRenderer returns the prompt renderer for this run. A non-nil
// r.Renderer is a variant; otherwise the cached embedded production renderer
// is used. Resolving once per run (not per case) keeps case throughput flat
// as the case set grows.
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
	return r.runCase(ctx, c, filepath.Dir(path), renderer, promptSource)
}

func (r Runner) RunCaseFiles(ctx context.Context, paths []string) ([]Result, error) {
	renderer, promptSource, err := r.resolveRenderer()
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(paths))
	for _, path := range paths {
		c, err := LoadCase(path)
		if err != nil {
			return nil, err
		}
		res, err := r.runCase(ctx, c, filepath.Dir(path), renderer, promptSource)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

// RunCase resolves the renderer and runs a single case. Prefer RunCaseFiles
// for multi-case runs so the renderer is resolved once.
func (r Runner) RunCase(ctx context.Context, c Case, baseDir string) (Result, error) {
	renderer, promptSource, err := r.resolveRenderer()
	if err != nil {
		return Result{}, err
	}
	return r.runCase(ctx, c, baseDir, renderer, promptSource)
}

func (r Runner) runCase(ctx context.Context, c Case, baseDir string, renderer *prompts.Renderer, promptSource string) (Result, error) {
	if strings.TrimSpace(c.Name) == "" {
		return Result{}, fmt.Errorf("case name is required")
	}
	if c.Tool != ToolKnowledgeTree {
		return Result{}, fmt.Errorf("unsupported tool %q: only %q is supported", c.Tool, ToolKnowledgeTree)
	}
	if strings.TrimSpace(c.Input.DocumentID) == "" {
		return Result{}, fmt.Errorf("input.document_id is required")
	}
	if strings.TrimSpace(c.Input.Chunks) == "" {
		return Result{}, fmt.Errorf("input.chunks is required")
	}

	chunksPath := resolvePath(baseDir, c.Input.Chunks)
	chunks, err := LoadChunks(chunksPath)
	if err != nil {
		return Result{}, err
	}
	input := InputSnapshot{
		DocumentID:  c.Input.DocumentID,
		Instruction: c.Input.Instruction,
		ChunksPath:  chunksPath,
		Chunks:      chunks,
	}

	res := Result{CaseName: c.Name, Tool: c.Tool, PromptSource: promptSource}
	start := time.Now()
	items, usage, err := process.GenerateKnowledgeTreeWithRenderer(ctx, r.LLM, renderer, process.GenerateKnowledgeTreeArgs{
		DocumentID:  c.Input.DocumentID,
		Chunks:      chunks,
		Instruction: c.Input.Instruction,
	})
	res.DurationMS = time.Since(start).Milliseconds()
	res.Model = usage.Model
	res.InputTokens = usage.InputTokens
	res.OutputTokens = usage.OutputTokens
	if err != nil {
		res.Error = err.Error()
		res.FailedInput = &input
		return res, nil
	}

	res.SchemaValid = true
	res.Items = items
	res.ItemCount = len(items)
	res.MaxDepth = maxDepth(items)
	res.MissingTitle = missingTitles(items, c.Expect.MustContainTitles)
	res.Passed = passes(c.Expect, res)

	if r.UpdateGolden {
		if err := writeGolden(r.GoldenDir, c.Name, items); err != nil {
			return Result{}, fmt.Errorf("update golden %s: %w", c.Name, err)
		}
	} else if r.GoldenDir != "" {
		res.GoldenChecked = true
		diff, err := compareGolden(r.GoldenDir, c.Name, items)
		if err != nil {
			return Result{}, fmt.Errorf("compare golden %s: %w", c.Name, err)
		}
		res.GoldenMatch = diff == nil
		if diff != nil {
			res.GoldenDiff = diff
			res.Passed = false
		}
	}

	if !res.Passed {
		res.FailedInput = &input
	}
	return res, nil
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

func maxDepth(items []domain.GeneratedTreeItem) int {
	max := 0
	byID := map[string]domain.GeneratedTreeItem{}
	for _, item := range items {
		byID[item.LocalID] = item
		if item.Level > max {
			max = item.Level
		}
	}
	if max > 0 {
		return max
	}
	for _, item := range items {
		depth := 1
		seen := map[string]bool{item.LocalID: true}
		parentID := item.ParentLocalID
		for parentID != "" && !seen[parentID] {
			seen[parentID] = true
			parent, ok := byID[parentID]
			if !ok {
				break
			}
			depth++
			parentID = parent.ParentLocalID
		}
		if depth > max {
			max = depth
		}
	}
	return max
}

func missingTitles(items []domain.GeneratedTreeItem, expected []string) []string {
	var missing []string
	for _, want := range expected {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}
		found := false
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Title), strings.ToLower(want)) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, want)
		}
	}
	return missing
}

func passes(expect CaseExpect, res Result) bool {
	if expect.SchemaValid && !res.SchemaValid {
		return false
	}
	if expect.MinItems > 0 && res.ItemCount < expect.MinItems {
		return false
	}
	if expect.MaxDepth > 0 && res.MaxDepth > expect.MaxDepth {
		return false
	}
	return len(res.MissingTitle) == 0 && res.Error == ""
}

var _ base.LLMClient = llm.Client(nil)

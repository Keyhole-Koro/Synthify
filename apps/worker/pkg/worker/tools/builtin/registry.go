// Package builtin provides the worker's ordered builtin tool set.
//
// The old eval multitool design doc was removed; the current eval-side
// contract lives in apps/eval/runner Tool doc comments.
//
// Factory signature note: many tool constructors take only *base.Context, but
// some need extra deps (journal / brief / glossary / analyzer+engine). Rather
// than widen the Factory signature (which would touch every constructor and
// risk changing production wiring), the caller registers a closure that
// captures those deps. From the registry's view every factory is the same
// shape; the closure carries what it needs.
package builtin

import (
	"fmt"
	"time"

	toolsio "github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin/io"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin/memory"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin/process"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
)

// Set is the builtin wiring product consumed by the orchestrator.
type Set struct {
	Tools    []core.Tool
	Memories []base.PromptMemory
	// DocumentMap is populated by the orchestrator's deterministic extract+chunk
	// pre-pass (not by a tool), so the orchestrator needs a direct handle.
	DocumentMap *memory.DocumentMap
}

// Build constructs the builtin tools in the production order and returns the
// prompt memories they share. Dynamic tools are resolved per job elsewhere.
func Build(b *base.Context) (Set, error) {
	glossary := memory.NewGlossary()
	journal := memory.NewJournal()
	brief := memory.NewBrief()
	documentMap := memory.NewDocumentMap()

	reg := NewRegistry()
	reg.Register("semantic_chunking", toolsio.NewChunkingTool)
	reg.Register("generate_knowledge_tree", process.NewGenerateKnowledgeTreeTool)
	reg.Register("persist_knowledge_tree", toolsio.NewPersistenceTool)
	reg.Register("journal_add_task", func(*base.Context) (core.Tool, error) {
		return memory.NewAddTaskTool(journal)
	})
	reg.Register("journal_update_task", func(*base.Context) (core.Tool, error) {
		return memory.NewUpdateTaskTool(journal)
	})
	reg.Register("analyze_dependencies", func(*base.Context) (core.Tool, error) {
		return toolsio.NewAnalysisTool()
	})
	reg.Register("glossary_register", func(*base.Context) (core.Tool, error) {
		return memory.NewRegisterTool(glossary)
	})
	reg.Register("glossary_lookup", func(*base.Context) (core.Tool, error) {
		return memory.NewLookupTool(glossary)
	})
	reg.Register("quality_critique", process.NewCritiqueTool)
	reg.Register("semantic_search", toolsio.NewSearchTool)
	reg.Register("deduplicate_and_merge", process.NewMergeTool)
	reg.Register("extract_table_data", func(*base.Context) (core.Tool, error) {
		return toolsio.NewTableTool()
	})
	reg.Register("repair_encoding", func(*base.Context) (core.Tool, error) {
		return toolsio.NewRepairTool()
	})
	reg.Register("extract_text", toolsio.NewExtractionTool)
	reg.Register("generate_brief", func(b *base.Context) (core.Tool, error) {
		return process.NewBriefTool(b, brief)
	})
	reg.Register("generate_html_summary", process.NewSummaryTool)
	reg.Register("grep_search", toolsio.NewGrepTool)
	reg.Register("create_transform", func(b *base.Context) (core.Tool, error) {
		return process.NewCreateTransformTool(
			b,
			transform.NewStarlarkAnalyzer(),
			transform.NewStarlarkEngine(10*time.Second),
		)
	})

	tools, err := reg.Build(b, []string{
		"semantic_chunking", "generate_knowledge_tree", "persist_knowledge_tree",
		"journal_add_task", "journal_update_task",
		"analyze_dependencies",
		"glossary_register", "glossary_lookup",
		"quality_critique", "semantic_search", "deduplicate_and_merge",
		"extract_table_data", "repair_encoding", "extract_text",
		"generate_brief", "generate_html_summary",
		"grep_search", "create_transform",
	})
	if err != nil {
		return Set{}, err
	}
	return Set{
		Tools: tools,
		// Document Map first: the agent should see "what the source contains"
		// before the derived state (brief/glossary/journal).
		Memories:    []base.PromptMemory{documentMap, brief, glossary, journal},
		DocumentMap: documentMap,
	}, nil
}

// Factory builds one tool from the shared base context. Extra dependencies are
// captured by the closure registered, not passed through this signature.
//
// The registry yields core.Tool values (道A); the orchestrator wraps
// them with adkadapter.Wrap before handing them to llmagent.New. This keeps
// the registry tool.Tool-free and the ADK type leak confined to the adapter.
type Factory func(*base.Context) (core.Tool, error)

// Registry is an explicit, non-global name → Factory map. It is constructed per
// orchestrator (not a package-level singleton) so the captured deps (journal,
// brief, …) belong to that orchestrator instance and tests stay isolated.
type Registry struct {
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register binds a tool name to its factory. Re-registering the same name is a
// programming error (the wiring is static and known) and panics, mirroring the
// contract's "registration is not idempotent" rule (§3).
func (r *Registry) Register(name string, f Factory) {
	if name == "" {
		panic("tools: Register with empty name")
	}
	if f == nil {
		panic(fmt.Sprintf("tools: Register %q with nil factory", name))
	}
	if _, dup := r.factories[name]; dup {
		panic(fmt.Sprintf("tools: duplicate Register %q", name))
	}
	r.factories[name] = f
}

// Build constructs the tools for names, in the given order. Order is preserved
// because the orchestrator's agent wiring is order-sensitive and the
// registry-based path must not change it. An unknown name is an error rather
// than a silent skip.
func (r *Registry) Build(b *base.Context, names []string) ([]core.Tool, error) {
	out := make([]core.Tool, 0, len(names))
	for _, name := range names {
		f, ok := r.factories[name]
		if !ok {
			return nil, fmt.Errorf("tools: unknown tool %q", name)
		}
		t, err := f(b)
		if err != nil {
			return nil, fmt.Errorf("tools: build %q: %w", name, err)
		}
		out = append(out, t)
	}
	return out, nil
}

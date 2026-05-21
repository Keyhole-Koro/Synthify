package runner

import (
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// builtinAll returns every builtin tool the runner should offer for a run.
//
// Why a dedicated file instead of building the map inline in runner.go: the
// runner must not know which specific builtin tools exist — its job is "look
// up a Tool by name and call its Run". Concentrating the builtin enumeration
// here keeps runner.go tool-agnostic (it just calls builtinAll), and makes
// "add a new builtin tool" a one-line change in this file rather than an edit
// to runner.go. Dynamic tools resolve through DynamicToolSource (dynamic.go)
// and merge into the same Tool table at lookup time; they don't appear here.
//
// Every builtin factory captures whatever it needs (e.g. the LLM client and
// the prompt renderer for --variant) in a closure, so the Tool type stays
// uniform regardless of tool-specific dependencies.
func builtinAll(llm base.LLMClient, renderer *prompts.Renderer) []Tool {
	return []Tool{
		newKnowledgeTreeTool(llm, renderer),
	}
}

// builtinTable indexes builtinAll by Tool.Name for runner lookup. The map is
// the runner's view of "which tools exist for this run"; the runner never
// iterates the file list, only this table.
func builtinTable(llm base.LLMClient, renderer *prompts.Renderer) map[string]Tool {
	all := builtinAll(llm, renderer)
	out := make(map[string]Tool, len(all))
	for _, t := range all {
		out[t.Name] = t
	}
	return out
}

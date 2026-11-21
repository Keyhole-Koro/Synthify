package memory

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

// Entry represents a single glossary entry with a term and its definition.
type Entry struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// Glossary manages an in-memory dictionary of domain terms and their definitions.
type Glossary struct {
	mu      sync.RWMutex
	entries map[string]string
}

// NewGlossary initializes an empty Glossary.
func NewGlossary() *Glossary {
	return &Glossary{entries: make(map[string]string)}
}

// RenderForPrompt formats the glossary terms and definitions as Markdown for prompt injection.
func (g *Glossary) RenderForPrompt() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if len(g.entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("### Glossary\n")
	for term, def := range g.entries {
		sb.WriteString("- **")
		sb.WriteString(term)
		sb.WriteString("**: ")
		sb.WriteString(def)
		sb.WriteString("\n")
	}
	return sb.String()
}

// registerArgs specifies the term and definition to register in the glossary.
type registerArgs struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// registerResult contains confirmation of the registered glossary term.
type registerResult struct {
	Message string `json:"message"`
}

// NewRegisterTool creates a core.Tool that allows an agent to register domain terms into the glossary.
func NewRegisterTool(g *Glossary) (core.Tool, error) {
	schema, err := core.IOSchemaFor[registerArgs, registerResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args registerArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		g.entries[args.Term] = args.Definition
		out, mErr := json.Marshal(registerResult{Message: "Term registered: " + args.Term})
		return out, core.Usage{}, mErr
	}
	return core.Tool{
		Name:        "glossary_register",
		Description: "Registers a domain-specific term and its definition for consistent interpretation across the document.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

// lookupArgs specifies the term to look up in the glossary.
type lookupArgs struct {
	Term string `json:"term"`
}

// lookupResult returns the definition if found, or an explanatory message if missing.
type lookupResult struct {
	Entry   *Entry `json:"entry,omitempty"`
	Message string `json:"message"`
}

// NewLookupTool creates a core.Tool that allows an agent to look up defined terms from the glossary.
func NewLookupTool(g *Glossary) (core.Tool, error) {
	schema, err := core.IOSchemaFor[lookupArgs, lookupResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args lookupArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		g.mu.RLock()
		defer g.mu.RUnlock()
		var result lookupResult
		if def, ok := g.entries[args.Term]; ok {
			result = lookupResult{Entry: &Entry{Term: args.Term, Definition: def}, Message: "Definition found"}
		} else {
			result = lookupResult{Message: "Term not found in glossary"}
		}
		out, mErr := json.Marshal(result)
		return out, core.Usage{}, mErr
	}
	return core.Tool{
		Name:        "glossary_lookup",
		Description: "Looks up a term in the glossary.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

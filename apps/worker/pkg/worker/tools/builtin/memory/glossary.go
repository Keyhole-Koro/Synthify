package memory

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

type Entry struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

type Glossary struct {
	mu      sync.RWMutex
	entries map[string]string
}

func NewGlossary() *Glossary {
	return &Glossary{entries: make(map[string]string)}
}

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

type registerArgs struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

type registerResult struct {
	Message string `json:"message"`
}

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

type lookupArgs struct {
	Term string `json:"term"`
}

type lookupResult struct {
	Entry   *Entry `json:"entry,omitempty"`
	Message string `json:"message"`
}

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

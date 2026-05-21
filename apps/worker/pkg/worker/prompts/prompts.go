// Package prompts is the single source of truth for production worker prompts.
//
// Templates are embedded via go:embed; the runtime never reads the filesystem
// or a remote registry. The eval runner renders the same production prompt
// through this package so prompt changes are observable in eval.
//
// Contract: docs/contracts/prompt-variant-eval-contract.md
package prompts

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"text/template"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// toolKnowledgeTree is the tool key that binds a prompt template pair
// (knowledge_tree.system.tmpl / knowledge_tree.user.tmpl), the eval variant
// dir, and the case "tool:" value.
const toolKnowledgeTree = "knowledge_tree"

// RenderInput is the input for rendering the knowledge tree prompt. The fields
// mirror process.GenerateKnowledgeTreeArgs so the worker and the eval runner render
// identical prompts from identical inputs.
type RenderInput struct {
	DocumentID  string
	Instruction string
	Chunks      []domain.Chunk
}

// Prompt is a rendered system/user prompt pair.
type Prompt struct {
	System string
	User   string
}

// Renderer renders a tool's prompt from its template set. The embedded
// renderer uses the production templates; the eval runner swaps in a variant
// renderer backed by a different template directory. Callers never learn
// whether the templates came from embed or disk — that is the storage
// boundary this type hides.
type Renderer struct {
	system *template.Template
	user   *template.Template
}

var (
	embeddedOnce     sync.Once
	embeddedRenderer *Renderer
	embeddedErr      error
)

// Default returns the renderer backed by the embedded production templates.
//
// The embedded templates are immutable, so the parsed renderer is built once
// and reused. Render is called once per chunk batch, so re-parsing on every
// call would be pure waste.
func Default() (*Renderer, error) {
	embeddedOnce.Do(func() {
		embeddedRenderer, embeddedErr = fromFS(templatesFS, "templates")
	})
	return embeddedRenderer, embeddedErr
}

// FromDir returns a renderer backed by an on-disk template directory. It is
// used by the eval runner for variant prompts. dir must contain
// knowledge_tree.system.tmpl and knowledge_tree.user.tmpl.
func FromDir(dir string) (*Renderer, error) {
	return fromFS(nil, dir)
}

func fromFS(efs fs.FS, dir string) (*Renderer, error) {
	read := func(name string) ([]byte, error) {
		if efs != nil {
			return fs.ReadFile(efs, dir+"/"+name)
		}
		return os.ReadFile(dir + "/" + name)
	}

	systemFile := toolKnowledgeTree + ".system.tmpl"
	userFile := toolKnowledgeTree + ".user.tmpl"

	sysRaw, err := read(systemFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", systemFile, err)
	}
	userRaw, err := read(userFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", userFile, err)
	}

	sysTmpl, err := template.New(systemFile).Parse(string(sysRaw))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", systemFile, err)
	}
	userTmpl, err := template.New(userFile).Parse(string(userRaw))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", userFile, err)
	}

	return &Renderer{system: sysTmpl, user: userTmpl}, nil
}

// Render builds the knowledge tree system/user prompt for the given input.
//
// The chunk block and the empty-instruction default ("none") reproduce the
// previous hardcoded behaviour byte-for-byte (contract §2.2).
func (r *Renderer) Render(in RenderInput) (Prompt, error) {
	instruction := in.Instruction
	if instruction == "" {
		instruction = "none"
	}

	var chunks strings.Builder
	for _, chunk := range in.Chunks {
		fmt.Fprintf(&chunks, "[%d] %s\n%s\n\n", chunk.ChunkIndex, chunk.Heading, chunk.Text)
	}

	var sysBuf strings.Builder
	if err := r.system.Execute(&sysBuf, nil); err != nil {
		return Prompt{}, fmt.Errorf("render system prompt: %w", err)
	}

	var userBuf strings.Builder
	err := r.user.Execute(&userBuf, struct {
		DocumentID  string
		Instruction string
		Chunks      string
	}{
		DocumentID:  in.DocumentID,
		Instruction: instruction,
		Chunks:      chunks.String(),
	})
	if err != nil {
		return Prompt{}, fmt.Errorf("render user prompt: %w", err)
	}

	return Prompt{System: sysBuf.String(), User: userBuf.String()}, nil
}

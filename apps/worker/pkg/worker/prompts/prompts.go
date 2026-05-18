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
	"text/template"

	"github.com/synthify/backend/packages/shared/domain"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// SynthesisInput is the render input for the synthesis prompt. The fields
// mirror process.SynthesisArgs so the worker and the eval runner render
// identical prompts from identical inputs.
type SynthesisInput struct {
	DocumentID  string
	Instruction string
	Chunks      []domain.Chunk
}

// Prompt is a rendered system/user prompt pair.
type Prompt struct {
	System string
	User   string
}

// Provider renders prompts from a template set. The default provider uses the
// embedded production templates; the eval runner swaps in a variant provider
// backed by a different template directory.
type Provider struct {
	system *template.Template
	user   *template.Template
}

// Default returns the provider backed by the embedded production templates.
func Default() (*Provider, error) {
	return fromFS(templatesFS, "templates")
}

// FromDir returns a provider backed by an on-disk template directory. It is
// used by the eval runner for variant prompts. dir must contain
// synthesis.system.tmpl and synthesis.user.tmpl.
func FromDir(dir string) (*Provider, error) {
	return fromFS(nil, dir)
}

func fromFS(efs fs.FS, dir string) (*Provider, error) {
	read := func(name string) ([]byte, error) {
		if efs != nil {
			return fs.ReadFile(efs, dir+"/"+name)
		}
		return os.ReadFile(dir + "/" + name)
	}

	sysRaw, err := read("synthesis.system.tmpl")
	if err != nil {
		return nil, fmt.Errorf("read synthesis.system.tmpl: %w", err)
	}
	userRaw, err := read("synthesis.user.tmpl")
	if err != nil {
		return nil, fmt.Errorf("read synthesis.user.tmpl: %w", err)
	}

	sysTmpl, err := template.New("synthesis.system").Parse(string(sysRaw))
	if err != nil {
		return nil, fmt.Errorf("parse synthesis.system.tmpl: %w", err)
	}
	userTmpl, err := template.New("synthesis.user").Parse(string(userRaw))
	if err != nil {
		return nil, fmt.Errorf("parse synthesis.user.tmpl: %w", err)
	}

	return &Provider{system: sysTmpl, user: userTmpl}, nil
}

// Synthesis renders the synthesis system/user prompt for the given input.
//
// The chunk block and the empty-instruction default ("none") reproduce the
// previous hardcoded behaviour byte-for-byte (contract §2.2).
func (p *Provider) Synthesis(in SynthesisInput) (Prompt, error) {
	instruction := in.Instruction
	if instruction == "" {
		instruction = "none"
	}

	var chunks strings.Builder
	for _, chunk := range in.Chunks {
		fmt.Fprintf(&chunks, "[%d] %s\n%s\n\n", chunk.ChunkIndex, chunk.Heading, chunk.Text)
	}

	var sysBuf strings.Builder
	if err := p.system.Execute(&sysBuf, nil); err != nil {
		return Prompt{}, fmt.Errorf("render synthesis system prompt: %w", err)
	}

	var userBuf strings.Builder
	err := p.user.Execute(&userBuf, struct {
		DocumentID  string
		Instruction string
		Chunks      string
	}{
		DocumentID:  in.DocumentID,
		Instruction: instruction,
		Chunks:      chunks.String(),
	})
	if err != nil {
		return Prompt{}, fmt.Errorf("render synthesis user prompt: %w", err)
	}

	return Prompt{System: sysBuf.String(), User: userBuf.String()}, nil
}

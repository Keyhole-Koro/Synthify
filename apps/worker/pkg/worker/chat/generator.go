package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	chatstatus "github.com/synthify/backend/internal/platform/chat/status"
)

// maxSections bounds one turn. Without a cap a model that never sets done would
// keep the turn open indefinitely, burning quota and holding the browser on a
// running document.
const maxSections = 8

// maxOutputTokensPerSection keeps a single write well inside Firestore's 1 MiB
// document limit, since content is rewritten in full on every section.
const maxOutputTokensPerSection = 4096

// Request is one dialogue turn to generate. It mirrors the worker RPC, already
// normalized by the API (size caps applied, history trimmed).
type Request struct {
	UserID      string
	WorkspaceID string
	ChatID      string
	TurnID      string
	PaperID     string

	History           []Message
	Candidates        []ContextPaper
	PinnedContextIDs  []string
	AutoSelectContext bool
}

type Message struct {
	Role string
	Text string
}

type ContextPaper struct {
	PaperID     string
	Title       string
	Description string
	Content     string
}

// Generator produces turns and publishes them.
type Generator struct {
	llm    llm.Client
	writer chatstatus.Writer
	logger *slog.Logger
}

func NewGenerator(client llm.Client, writer chatstatus.Writer, logger *slog.Logger) *Generator {
	return &Generator{llm: client, writer: writer, logger: logger}
}

// Run generates a turn section by section, publishing after each one.
//
// Errors are reported through the turn document rather than returned: the API
// dispatches and returns immediately, so by the time generation fails there is
// no caller left to receive an error. The returned error is for logging only.
func (g *Generator) Run(ctx context.Context, req Request) error {
	ref := chatstatus.Ref{
		UserID:      req.UserID,
		WorkspaceID: req.WorkspaceID,
		ChatID:      req.ChatID,
		TurnID:      req.TurnID,
		PaperID:     req.PaperID,
	}

	if err := g.writer.Start(ctx, ref); err != nil {
		// Nothing can be published, so generating would only burn quota on an
		// answer with nowhere to go.
		return fmt.Errorf("start turn: %w", err)
	}

	allowed := allowedPaperIDs(req.Candidates)
	systemPrompt := buildSystemPrompt(req)

	var (
		nodes        []ContentNode
		commands     []Command
		usage        chatstatus.Usage
		selectedIDs  []string
		finishReason = chatstatus.FirestoreChatTurnFinishReasonStop
		sections     int
	)

	for sections < maxSections {
		raw, callUsage, err := g.llm.GenerateStructured(ctx, llm.StructuredRequest{
			SystemPrompt: systemPrompt,
			UserPrompt:   buildUserPrompt(req, nodes),
			Schema:       sectionSchema(),
		})
		usage.PromptTokens += int(callUsage.InputTokens)
		usage.CompletionTokens += int(callUsage.OutputTokens)

		if err != nil {
			// A section that fails after others succeeded keeps what is
			// already there: a partial answer beats discarding the turn.
			g.logger.Error("chat.section_failed",
				"error", err.Error(), "chat_id", req.ChatID, "turn_id", req.TurnID, "section", sections)
			return g.fail(ctx, ref, nodes, sections, usage, err)
		}

		section, err := parseSection(raw)
		if err != nil {
			g.logger.Error("chat.section_unparseable",
				"error", err.Error(), "chat_id", req.ChatID, "turn_id", req.TurnID, "section", sections)
			return g.fail(ctx, ref, nodes, sections, usage, err)
		}

		nodes = append(nodes, Sanitize(section.Nodes, allowed)...)
		commands = append(commands, section.Commands...)
		sections++

		if sections == 1 {
			selectedIDs = resolveSelectedContext(req, section.SelectedContextIDs, allowed)
		}

		contentJSON, err := marshalContent(nodes)
		if err != nil {
			return g.fail(ctx, ref, nodes, sections, usage, err)
		}
		if err := g.writer.Progress(ctx, ref, contentJSON, sections); err != nil {
			// The turn is still generating; a dropped progress write is not
			// fatal because the next one carries the full content anyway.
			g.logger.Warn("chat.progress_write_failed",
				"error", err.Error(), "chat_id", req.ChatID, "turn_id", req.TurnID)
		}

		if section.Done {
			break
		}
		if sections == maxSections {
			finishReason = chatstatus.FirestoreChatTurnFinishReasonSectionCap
			g.logger.Warn("chat.section_cap_reached",
				"chat_id", req.ChatID, "turn_id", req.TurnID, "sections", sections)
		}
	}

	contentJSON, err := marshalContent(nodes)
	if err != nil {
		return g.fail(ctx, ref, nodes, sections, usage, err)
	}

	// Classified once at the end rather than per section: view commands issued
	// mid-generation would move the canvas while the reader is still reading
	// the part that is already there.
	auto, proposals := ClassifyCommands(commands, allowed)
	if dropped := len(commands) - len(auto) - len(proposals); dropped > 0 {
		// Worth a line: a model proposing commands outside what it was offered
		// is either a prompt problem or an attempt at something it should not
		// be doing, and both are invisible otherwise.
		g.logger.Warn("chat.commands_dropped",
			"chat_id", req.ChatID, "turn_id", req.TurnID,
			"proposed", len(commands), "dropped", dropped)
	}
	commandsJSON, err := marshalCommands(auto)
	if err != nil {
		return g.fail(ctx, ref, nodes, sections, usage, err)
	}
	proposalsJSON, err := marshalCommands(proposals)
	if err != nil {
		return g.fail(ctx, ref, nodes, sections, usage, err)
	}

	return g.writer.Succeed(ctx, ref, chatstatus.Success{
		ContentJSON:        contentJSON,
		CommandsJSON:       commandsJSON,
		ProposalsJSON:      proposalsJSON,
		SelectedContextIDs: selectedIDs,
		SectionCount:       sections,
		FinishReason:       finishReason,
		Usage:              usage,
	})
}

func (g *Generator) fail(
	ctx context.Context,
	ref chatstatus.Ref,
	nodes []ContentNode,
	sections int,
	usage chatstatus.Usage,
	cause error,
) error {
	contentJSON, marshalErr := marshalContent(nodes)
	if marshalErr != nil {
		contentJSON = "[]"
	}
	if err := g.writer.Fail(ctx, ref, chatstatus.Failure{
		ContentJSON:  contentJSON,
		SectionCount: sections,
		ErrorMessage: cause.Error(),
		Reason:       chatstatus.FirestoreChatTurnReasonInternal,
		Usage:        usage,
	}); err != nil {
		g.logger.Error("chat.fail_write_failed", "error", err.Error())
	}
	return cause
}

func marshalContent(nodes []ContentNode) (string, error) {
	if nodes == nil {
		// An empty array, not "null": the browser parses this straight into
		// ContentNode[] and null would crash the render.
		return "[]", nil
	}
	raw, err := json.Marshal(nodes)
	if err != nil {
		return "", fmt.Errorf("marshal content: %w", err)
	}
	return string(raw), nil
}

func allowedPaperIDs(candidates []ContextPaper) map[string]bool {
	out := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		if c.PaperID != "" {
			out[c.PaperID] = true
		}
	}
	return out
}

// resolveSelectedContext reports which papers the answer drew on. Pinned ids
// are always included because the user chose them explicitly; model-selected
// ids are kept only when they name a real candidate.
func resolveSelectedContext(req Request, modelSelected []string, allowed map[string]bool) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(req.PinnedContextIDs)+len(modelSelected))
	for _, id := range req.PinnedContextIDs {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if !req.AutoSelectContext {
		return out
	}
	for _, id := range modelSelected {
		if allowed[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func buildSystemPrompt(req Request) string {
	var b strings.Builder
	b.WriteString(`You are helping a reader explore a knowledge tree of papers.

Answer in structured nodes, not prose blobs. Reply with JSON:
{"nodes": [...], "done": bool, "selectedContextIds": [...]}

Produce ONE section per reply. Set "done": true when the answer is complete;
you will be called again while it is false. Prefer finishing in one or two
sections — only continue when there is genuinely more to say.

Node types: text, paragraph, bold, section, list, table, callout, and for
references paper-link {"type":"paper-link","paperId":"...","label":"..."} and
card {"type":"card","paperId":"...","title":"...","description":"..."}.

Reference a paper ONLY by an id listed in the context below. Never invent an
id: a link to a paper that does not exist is worse than no link.

You may also return "commands" to act on the canvas, using only ids from the
context:
  OPEN_NODE {"type":"OPEN_NODE","parentId":"...","childId":"..."}
  FOCUS_NODE / PIN_NODE / UNPIN_NODE / CLOSE_NODE with "nodeId"
Use these to show the reader the paper you are talking about instead of only
naming it. Prefer none over a guess — an unnecessary command moves their view
for no reason.

You may propose CREATE_CHILD_NODE, PATCH_NODE or MOVE_NODE. These are shown to
the reader for approval and are never applied automatically, so propose them
only when the change is clearly worth making.
`)
	if req.AutoSelectContext {
		b.WriteString(`
In your FIRST reply, set "selectedContextIds" to the candidate ids you actually
used.
`)
	}
	return b.String()
}

func buildUserPrompt(req Request, produced []ContentNode) string {
	var b strings.Builder

	if len(req.Candidates) > 0 {
		b.WriteString("## Context papers\n\n")
		for _, c := range req.Candidates {
			fmt.Fprintf(&b, "### %s (id: %s)\n", c.Title, c.PaperID)
			if c.Description != "" {
				fmt.Fprintf(&b, "%s\n", c.Description)
			}
			if c.Content != "" {
				fmt.Fprintf(&b, "\n%s\n", c.Content)
			}
			b.WriteString("\n")
		}
	}
	if len(req.PinnedContextIDs) > 0 {
		fmt.Fprintf(&b, "The reader pinned these papers, so use them: %s\n\n",
			strings.Join(req.PinnedContextIDs, ", "))
	}

	b.WriteString("## Conversation\n\n")
	for _, m := range req.History {
		fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Text)
	}

	// Later sections need to know what has already been said, or they repeat
	// the opening. Titles alone are enough to continue from without resending
	// the whole answer each time.
	if len(produced) > 0 {
		fmt.Fprintf(&b, "\n## Already written (%d section(s))\n\n", len(produced))
		b.WriteString("Continue from here; do not repeat it.\n")
		if summary := summarizeNodes(produced); summary != "" {
			fmt.Fprintf(&b, "%s\n", summary)
		}
	}

	return b.String()
}

// summarizeNodes lists the headings and opening text already produced, so a
// follow-up section can continue without the full prior content in the prompt.
func summarizeNodes(nodes []ContentNode) string {
	var lines []string
	for _, n := range nodes {
		typ, _ := n["type"].(string)
		switch typ {
		case "section":
			if title, ok := n["title"].(string); ok && title != "" {
				lines = append(lines, "- section: "+title)
			}
		case "text":
			if v, ok := n["value"].(string); ok && v != "" {
				lines = append(lines, "- text: "+truncate(v, 80))
			}
		default:
			lines = append(lines, "- "+typ)
		}
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut] + "…"
}

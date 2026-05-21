package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/synthify/backend/apps/eval/runner"
)

func Write(w io.Writer, format string, results []runner.Result) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(results)
	case "table":
		return writeTable(w, results)
	default:
		return fmt.Errorf("unsupported format %q: use table or json", format)
	}
}

func writeTable(w io.Writer, results []runner.Result) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CASE\tPASS\tSCHEMA\tPROMPT_SRC\tFAILED_INPUT\tDURATION_MS\tMODEL\tTOKENS_IN\tTOKENS_OUT\tERROR")
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%t\t%t\t%s\t%s\t%d\t%s\t%d\t%d\t%s\n",
			r.CaseName,
			r.Passed,
			r.SchemaValid,
			r.PromptSource,
			failedInputSummary(r),
			r.DurationMS,
			r.Model,
			r.InputTokens,
			r.OutputTokens,
			r.Error,
		)
	}
	return tw.Flush()
}

func failedInputSummary(r runner.Result) string {
	if r.FailedInput == nil {
		return ""
	}
	headings := make([]string, 0, len(r.FailedInput.Chunks))
	for _, chunk := range r.FailedInput.Chunks {
		if strings.TrimSpace(chunk.Heading) != "" {
			headings = append(headings, chunk.Heading)
		}
	}
	return fmt.Sprintf("document_id=%s chunks=%s count=%d headings=%s",
		r.FailedInput.DocumentID,
		r.FailedInput.ChunksPath,
		len(r.FailedInput.Chunks),
		strings.Join(headings, "|"),
	)
}

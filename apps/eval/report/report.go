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
		return enc.Encode(results)
	case "table":
		return writeTable(w, results)
	default:
		return fmt.Errorf("unsupported format %q: use table or json", format)
	}
}

func writeTable(w io.Writer, results []runner.Result) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CASE\tPASS\tSCHEMA\tITEMS\tMAX_DEPTH\tMISSING_TITLES\tDURATION_MS\tMODEL\tTOKENS_IN\tTOKENS_OUT\tERROR")
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%t\t%t\t%d\t%d\t%s\t%d\t%s\t%d\t%d\t%s\n",
			r.CaseName,
			r.Passed,
			r.SchemaValid,
			r.ItemCount,
			r.MaxDepth,
			strings.Join(r.MissingTitle, ","),
			r.DurationMS,
			r.Model,
			r.InputTokens,
			r.OutputTokens,
			r.Error,
		)
	}
	return tw.Flush()
}

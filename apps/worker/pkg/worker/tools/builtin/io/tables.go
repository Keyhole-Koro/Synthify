package io

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

// TableArgs specifies parameters for extracting tabular data from text.
//
// @property ChunkID - Source chunk identifier included in the output payload.
// @property Text - Markdown table text to parse.
type TableArgs struct {
	// ChunkID identifies the source chunk included in the serialized table payload.
	ChunkID string `json:"chunk_id"`
	// Text contains Markdown table syntax to parse.
	Text string `json:"text,omitempty"`
}

// TableResult contains extracted table data formatted as a JSON string.
//
// @property TableJSON - Serialized object containing the chunk ID and parsed rows.
type TableResult struct {
	// TableJSON is a JSON object containing the source chunk ID and parsed rows.
	TableJSON string `json:"table_json"`
}

// NewTableTool creates a core.Tool for parsing Markdown tables into structured JSON.
//
// The tool accepts a chunk ID and Markdown text, extracts pipe-delimited rows,
// and returns a serialized JSON payload suitable for downstream processing.
//
// Returns:
//   - a configured extract_table_data tool;
//   - an error when the input/output schema cannot be generated.
//
// @return Configured extract_table_data tool and optional schema-generation error.
func NewTableTool() (core.Tool, error) {
	schema, err := core.IOSchemaFor[TableArgs, TableResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args TableArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		rows := parseMarkdownTable(args.Text)
		payload, err := json.Marshal(map[string]any{
			"chunk_id": args.ChunkID,
			"rows":     rows,
		})
		if err != nil {
			return nil, core.Usage{}, err
		}
		out, err := json.Marshal(TableResult{TableJSON: string(payload)})
		return out, core.Usage{}, err
	}
	return core.Tool{
		Name:        "extract_table_data",
		Description: "Specially parses complex tables from document chunks into structured JSON format.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

// parseMarkdownTable extracts rows and columns from Markdown table syntax in text.
func parseMarkdownTable(text string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "|") || strings.Trim(line, "|-: ") == "" {
			continue
		}
		parts := strings.Split(strings.Trim(line, "|"), "|")
		row := make([]string, 0, len(parts))
		for _, part := range parts {
			row = append(row, strings.TrimSpace(part))
		}
		rows = append(rows, row)
	}
	return rows
}

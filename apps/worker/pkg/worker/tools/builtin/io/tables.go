package io

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

type TableArgs struct {
	ChunkID string `json:"chunk_id"`
	Text    string `json:"text,omitempty"`
}

type TableResult struct {
	TableJSON string `json:"table_json"`
}

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

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/synthify/backend/apps/eval/runner"
	"github.com/synthify/backend/apps/eval/store/sqlcgen"
	"github.com/synthify/backend/internal/platform/database"
)

type Run struct {
	ID           string
	PromptSource string
	ArtifactURI  string
	StartedAt    time.Time
	CompletedAt  time.Time
	Results      []runner.Result
}

type Summary struct {
	CaseCount    int64
	PassedCount  int64
	FailedCount  int64
	PassRate     float64
	Model        string
	InputTokens  int64
	OutputTokens int64
	Status       string
}

func Summarize(results []runner.Result) Summary {
	s := Summary{CaseCount: int64(len(results)), Status: "succeeded"}
	models := make(map[string]struct{})
	for _, result := range results {
		if result.Passed {
			s.PassedCount++
		} else {
			s.FailedCount++
			s.Status = "failed"
		}
		s.InputTokens += result.InputTokens
		s.OutputTokens += result.OutputTokens
		if model := strings.TrimSpace(result.Model); model != "" {
			models[model] = struct{}{}
		}
	}
	if s.CaseCount > 0 {
		s.PassRate = float64(s.PassedCount) / float64(s.CaseCount)
	}
	switch len(models) {
	case 0:
		s.Model = ""
	case 1:
		for model := range models {
			s.Model = model
		}
	default:
		s.Model = "mixed"
	}
	return s
}

func Persist(ctx context.Context, dsn string, run Run) error {
	if strings.TrimSpace(dsn) == "" {
		return fmt.Errorf("database DSN is empty")
	}
	runID, err := uuid.Parse(strings.TrimSpace(run.ID))
	if err != nil {
		return fmt.Errorf("run ID: %w", err)
	}

	db, err := database.OpenDB(dsn, database.PoolConfig{MaxOpenConns: 2, MaxIdleConns: 2}, nil)
	if err != nil {
		return fmt.Errorf("open eval database: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping eval database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin eval transaction: %w", err)
	}
	defer tx.Rollback()
	q := sqlcgen.New(tx)

	summary := Summarize(run.Results)
	durationMS := run.CompletedAt.Sub(run.StartedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	if err := q.InsertEvalRun(ctx, sqlcgen.InsertEvalRunParams{
		RunID:        runID,
		PromptSource: run.PromptSource,
		ArtifactUri:  run.ArtifactURI,
		Status:       summary.Status,
		CaseCount:    summary.CaseCount,
		PassedCount:  summary.PassedCount,
		FailedCount:  summary.FailedCount,
		PassRate:     summary.PassRate,
		DurationMs:   durationMS,
		Model:        summary.Model,
		InputTokens:  summary.InputTokens,
		OutputTokens: summary.OutputTokens,
		StartedAt:    run.StartedAt,
		CompletedAt:  run.CompletedAt,
	}); err != nil {
		return fmt.Errorf("insert eval run: %w", err)
	}

	for index, result := range run.Results {
		outputJSON, err := rawJSONOrNil(result.Output)
		if err != nil {
			return fmt.Errorf("marshal output for case %q: %w", result.CaseName, err)
		}
		failedInputJSON, err := jsonOrNil(result.FailedInput)
		if err != nil {
			return fmt.Errorf("marshal failed input for case %q: %w", result.CaseName, err)
		}
		if err := q.InsertEvalCaseResult(ctx, sqlcgen.InsertEvalCaseResultParams{
			RunID:           runID,
			CaseIndex:       int64(index),
			CaseName:        result.CaseName,
			Tool:            result.Tool,
			Passed:          result.Passed,
			SchemaValid:     result.SchemaValid,
			DurationMs:      result.DurationMS,
			Model:           result.Model,
			InputTokens:     result.InputTokens,
			OutputTokens:    result.OutputTokens,
			Error:           result.Error,
			OutputJson:      outputJSON,
			FailedInputJson: failedInputJSON,
			PromptSource:    result.PromptSource,
		}); err != nil {
			return fmt.Errorf("insert eval case %q: %w", result.CaseName, err)
		}

		if err := insertTraceEvents(ctx, q, runID, index, result.Trace); err != nil {
			return fmt.Errorf("insert trace for case %q: %w", result.CaseName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit eval transaction: %w", err)
	}
	return nil
}

// insertTraceEvents writes the per-case LLM/tool execution spans captured by
// the runner's tracing client. case_index ties each event to its eval_case_results
// row (the foreign key eval_trace_events_case_fk).
func insertTraceEvents(ctx context.Context, q *sqlcgen.Queries, runID uuid.UUID, caseIndex int, events []runner.TraceEvent) error {
	for _, ev := range events {
		eventID, err := uuid.Parse(strings.TrimSpace(ev.EventID))
		if err != nil {
			return fmt.Errorf("event ID (event %q): %w", ev.Name, err)
		}
		var parent uuid.NullUUID
		if p := strings.TrimSpace(ev.ParentEventID); p != "" {
			parsed, err := uuid.Parse(p)
			if err != nil {
				return fmt.Errorf("parent event ID (event %q): %w", ev.Name, err)
			}
			parent = uuid.NullUUID{UUID: parsed, Valid: true}
		}
		inputJSON, err := rawJSONOrNil(ev.Input)
		if err != nil {
			return fmt.Errorf("marshal trace input (event %q): %w", ev.Name, err)
		}
		outputJSON, err := rawJSONOrNil(ev.Output)
		if err != nil {
			return fmt.Errorf("marshal trace output (event %q): %w", ev.Name, err)
		}
		if err := q.InsertEvalTraceEvent(ctx, sqlcgen.InsertEvalTraceEventParams{
			RunID:         runID,
			CaseIndex:     int64(caseIndex),
			EventID:       eventID,
			ParentEventID: parent,
			Sequence:      int64(ev.Sequence),
			Kind:          ev.Kind,
			Name:          ev.Name,
			StartedAt:     ev.StartedAt,
			CompletedAt:   ev.CompletedAt,
			DurationMs:    ev.DurationMS,
			Model:         ev.Model,
			InputTokens:   ev.InputTokens,
			OutputTokens:  ev.OutputTokens,
			InputJson:     inputJSON,
			OutputJson:    outputJSON,
			Error:         ev.Error,
		}); err != nil {
			return fmt.Errorf("insert trace event %q: %w", ev.Name, err)
		}
	}
	return nil
}

func rawJSONOrNil(raw json.RawMessage) (pqtype.NullRawMessage, error) {
	if len(raw) == 0 {
		return pqtype.NullRawMessage{}, nil
	}
	if !json.Valid(raw) {
		return pqtype.NullRawMessage{}, fmt.Errorf("invalid JSON")
	}
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}, nil
}

func jsonOrNil(value any) (pqtype.NullRawMessage, error) {
	if value == nil {
		return pqtype.NullRawMessage{}, nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return pqtype.NullRawMessage{}, err
	}
	return pqtype.NullRawMessage{RawMessage: b, Valid: true}, nil
}

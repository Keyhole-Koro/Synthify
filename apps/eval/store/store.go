package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/synthify/backend/apps/eval/runner"
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
	if strings.TrimSpace(run.ID) == "" {
		return fmt.Errorf("run ID is empty")
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

	summary := Summarize(run.Results)
	durationMS := run.CompletedAt.Sub(run.StartedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO eval_runs (
			run_id, prompt_source, artifact_uri, status,
			case_count, passed_count, failed_count, pass_rate,
			duration_ms, model, input_tokens, output_tokens,
			started_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, run.ID, run.PromptSource, run.ArtifactURI, summary.Status,
		summary.CaseCount, summary.PassedCount, summary.FailedCount, summary.PassRate,
		durationMS, summary.Model, summary.InputTokens, summary.OutputTokens,
		run.StartedAt, run.CompletedAt)
	if err != nil {
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
		_, err = tx.ExecContext(ctx, `
			INSERT INTO eval_case_results (
				run_id, case_index, case_name, tool, passed, schema_valid,
				duration_ms, model, input_tokens, output_tokens, error,
				output_json, failed_input_json, prompt_source
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, run.ID, index, result.CaseName, result.Tool, result.Passed, result.SchemaValid,
			result.DurationMS, result.Model, result.InputTokens, result.OutputTokens, result.Error,
			outputJSON, failedInputJSON, result.PromptSource)
		if err != nil {
			return fmt.Errorf("insert eval case %q: %w", result.CaseName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit eval transaction: %w", err)
	}
	return nil
}

func rawJSONOrNil(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("invalid JSON")
	}
	return string(raw), nil
}

func jsonOrNil(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

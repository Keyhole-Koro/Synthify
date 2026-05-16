package starlarkrt

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
	"go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

const (
	defaultMaxExecutionSteps = 100_000
	defaultTimeout           = 500 * time.Millisecond
)

// Engine executes the phase-1 embedded Starlark transform runtime. It is a
// stepping stone until the isolated executor service owns all languages.
type Engine struct {
	MaxExecutionSteps uint64
	Timeout           time.Duration
	MaxOutputBytes    int64
}

type Analysis struct {
	Status      treev1.ExecuteStatus
	FloorTier   treev1.RiskTier
	Diagnostics []Diagnostic
}

type Diagnostic struct {
	Tier    treev1.RiskTier
	Message string
	Line    int32
	Column  int32
}

type ExecuteRequest struct {
	Code           string
	Input          []byte
	MaxOutputBytes int64
	Timeout        time.Duration
}

type ExecuteResult struct {
	Status    treev1.ExecuteStatus
	Stderr    string
	Output    []byte
	FloorTier treev1.RiskTier
	Duration  time.Duration
}

func New() *Engine {
	return &Engine{
		MaxExecutionSteps: defaultMaxExecutionSteps,
		Timeout:           defaultTimeout,
	}
}

func (e *Engine) Analyze(code string) Analysis {
	opts := fileOptions()
	file, err := opts.Parse("transform.star", code, 0)
	if err != nil {
		return Analysis{
			Status:    treev1.ExecuteStatus_EXECUTE_STATUS_SYNTAX_ERROR,
			FloorTier: treev1.RiskTier_RISK_TIER_3,
			Diagnostics: []Diagnostic{{
				Tier:    treev1.RiskTier_RISK_TIER_3,
				Message: err.Error(),
			}},
		}
	}

	analyzer := staticAnalyzer{
		floor: treev1.RiskTier_RISK_TIER_1,
	}
	analyzer.walk(file)
	status := treev1.ExecuteStatus_EXECUTE_STATUS_OK
	for _, d := range analyzer.diagnostics {
		if strings.Contains(d.Message, "load statements are not allowed") {
			status = treev1.ExecuteStatus_EXECUTE_STATUS_ANALYZER_REJECTED
			break
		}
	}
	return Analysis{
		Status:      status,
		FloorTier:   analyzer.floor,
		Diagnostics: analyzer.diagnostics,
	}
}

func (e *Engine) Execute(ctx context.Context, req ExecuteRequest) ExecuteResult {
	start := time.Now()
	analysis := e.Analyze(req.Code)
	if analysis.Status != treev1.ExecuteStatus_EXECUTE_STATUS_OK {
		return ExecuteResult{
			Status:    analysis.Status,
			Stderr:    diagnosticsString(analysis.Diagnostics),
			FloorTier: analysis.FloorTier,
			Duration:  time.Since(start),
		}
	}

	timeout := firstPositiveDuration(req.Timeout, e.Timeout, defaultTimeout)
	maxOutput := firstPositiveInt64(req.MaxOutputBytes, e.MaxOutputBytes)
	thread := &starlark.Thread{Name: "synthify-starlark-transform"}
	thread.SetMaxExecutionSteps(firstPositiveUint64(e.MaxExecutionSteps, defaultMaxExecutionSteps))
	thread.Print = func(thread *starlark.Thread, msg string) {}

	var timedOut atomic.Bool
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan struct{})
	go func() {
		select {
		case <-runCtx.Done():
			timedOut.Store(true)
			thread.Cancel("timeout")
		case <-done:
		}
	}()
	defer close(done)

	opts := fileOptions()
	globals, err := starlark.ExecFileOptions(&opts, thread, "transform.star", req.Code, starlark.StringDict{
		"json": json.Module,
	})
	if err != nil {
		return evalFailure(err, timedOut.Load(), analysis.FloorTier, start)
	}
	fn, ok := globals["transform"]
	if !ok {
		return ExecuteResult{
			Status:    treev1.ExecuteStatus_EXECUTE_STATUS_NONZERO_EXIT,
			Stderr:    "missing required transform(input) function",
			FloorTier: analysis.FloorTier,
			Duration:  time.Since(start),
		}
	}
	if _, ok := fn.(starlark.Callable); !ok {
		return ExecuteResult{
			Status:    treev1.ExecuteStatus_EXECUTE_STATUS_NONZERO_EXIT,
			Stderr:    "global transform is not callable",
			FloorTier: analysis.FloorTier,
			Duration:  time.Since(start),
		}
	}
	value, err := starlark.Call(thread, fn, starlark.Tuple{starlark.String(string(req.Input))}, nil)
	if err != nil {
		return evalFailure(err, timedOut.Load(), analysis.FloorTier, start)
	}
	output, err := valueToBytes(value)
	if err != nil {
		return ExecuteResult{
			Status:    treev1.ExecuteStatus_EXECUTE_STATUS_NONZERO_EXIT,
			Stderr:    err.Error(),
			FloorTier: analysis.FloorTier,
			Duration:  time.Since(start),
		}
	}
	if len(output) == 0 {
		return ExecuteResult{
			Status:    treev1.ExecuteStatus_EXECUTE_STATUS_OUTPUT_MISSING,
			Stderr:    "transform returned empty output",
			FloorTier: analysis.FloorTier,
			Duration:  time.Since(start),
		}
	}
	if maxOutput > 0 && int64(len(output)) > maxOutput {
		return ExecuteResult{
			Status:    treev1.ExecuteStatus_EXECUTE_STATUS_OUTPUT_TOO_LARGE,
			Stderr:    fmt.Sprintf("output size %d exceeds limit %d", len(output), maxOutput),
			FloorTier: analysis.FloorTier,
			Duration:  time.Since(start),
		}
	}
	return ExecuteResult{
		Status:    treev1.ExecuteStatus_EXECUTE_STATUS_OK,
		Output:    output,
		FloorTier: analysis.FloorTier,
		Duration:  time.Since(start),
	}
}

func fileOptions() syntax.FileOptions {
	return syntax.FileOptions{While: true}
}

func evalFailure(err error, timedOut bool, floor treev1.RiskTier, start time.Time) ExecuteResult {
	status := treev1.ExecuteStatus_EXECUTE_STATUS_NONZERO_EXIT
	if timedOut || strings.Contains(err.Error(), "too many steps") {
		status = treev1.ExecuteStatus_EXECUTE_STATUS_TIMEOUT
	}
	return ExecuteResult{
		Status:    status,
		Stderr:    err.Error(),
		FloorTier: floor,
		Duration:  time.Since(start),
	}
}

func valueToBytes(value starlark.Value) ([]byte, error) {
	if value == starlark.None {
		return nil, nil
	}
	if s, ok := starlark.AsString(value); ok {
		return []byte(s), nil
	}
	return nil, fmt.Errorf("transform must return string or bytes, got %s", value.Type())
}

func diagnosticsString(ds []Diagnostic) string {
	if len(ds) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ds))
	for _, d := range ds {
		if d.Line > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d: %s", d.Line, d.Column, d.Message))
		} else {
			parts = append(parts, d.Message)
		}
	}
	return strings.Join(parts, "\n")
}

func firstPositiveDuration(values ...time.Duration) time.Duration {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func firstPositiveInt64(values ...int64) int64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func firstPositiveUint64(values ...uint64) uint64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

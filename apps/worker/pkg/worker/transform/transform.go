// Package transform runs LLM-generated transform code and computes its risk
// tier. Stage 1 ("Starlark 踏み台", see docs/improvements/dynamic-tool-synthesis.md)
// implements the Starlark engine + static analyzer in-process; Python is a
// later stage behind a separate executor service.
package transform

// Status mirrors synthify.tree.v1.ExecuteStatus. The transform package keeps
// its own value type so the engine layer does not depend on generated proto.
type Status string

const (
	StatusOK               Status = "ok"
	StatusSyntaxError      Status = "syntax_error"
	StatusAnalyzerRejected Status = "analyzer_rejected"
	StatusTimeout          Status = "timeout"
	StatusOOM              Status = "oom" // out of memory
	StatusNonzeroExit      Status = "nonzero_exit"
	StatusOutputTooLarge   Status = "output_too_large"
	StatusOutputMissing    Status = "output_missing"
)

// Tier is the risk tier string used across the codebase (util.NormalizeRiskTier).
type Tier string

const (
	Tier1 Tier = "tier_1"
	Tier2 Tier = "tier_2"
	Tier3 Tier = "tier_3"
)

// Language selects the runtime. Only Starlark is wired in stage 1; Python is
// dispatched to the executor service in a later stage.
type Language string

const (
	LangStarlark Language = "starlark"
	LangPython   Language = "python"
)

// AnalysisResult is the output of static analysis. Rejected is true when the
// code uses a construct that is never allowed (e.g. Starlark load()), in which
// case FloorTier is meaningless and the caller must not execute the code.
type AnalysisResult struct {
	Rejected bool   // analyzer_rejected: do not execute
	Reason   string // human/LLM-facing explanation when Rejected or SyntaxErr
	SyntaxErr bool  // code failed to parse (syntax_error)
	// FloorTier is the machine-derived lower bound. Defined only when neither
	// Rejected nor SyntaxErr is set. Defaults to Tier3 on analyzer failure
	// (fail-secure).
	FloorTier Tier
}

// Analyzer parses code and computes its floor tier without executing it.
// Parsing happens once and serves both syntax validation and the AST walk
// (see "判定パイプライン" step 0/a in the design doc).
type Analyzer interface {
	Analyze(code string) AnalysisResult
}

// ExecRequest is one transform execution. The code must define a top-level
// function `transform(input)` returning a string. Input/output are strings;
// io_schema validation is a separate worker-side layer, not the engine's job.
type ExecRequest struct {
	Code  string
	Input string
}

// ExecResult is the engine outcome. On Status != StatusOK, Output is empty and
// Stderr carries the diagnostic the worker relays to the LLM.
type ExecResult struct {
	Status Status
	Output string
	Stderr string
}

// Engine executes transform code for one language in an unprivileged,
// deterministic sandbox with a hard timeout. Stage 1 only wires Starlark.
type Engine interface {
	Language() Language
	Execute(req ExecRequest) ExecResult
}

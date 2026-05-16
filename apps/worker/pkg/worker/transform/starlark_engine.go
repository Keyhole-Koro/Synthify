package transform

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// StarlarkEngine runs Starlark transforms in-process. The sandbox is
// unprivileged by construction: only the json module is predeclared, load()
// is disabled, and there is no filesystem/network builtin. A watchdog cancels
// the thread after timeout (Starlark has no goroutines, so cancel is safe).
type StarlarkEngine struct {
	timeout time.Duration
}

func NewStarlarkEngine(timeout time.Duration) *StarlarkEngine {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &StarlarkEngine{timeout: timeout}
}

func (e *StarlarkEngine) Language() Language { return LangStarlark }

func (e *StarlarkEngine) Execute(req ExecRequest) ExecResult {
	thread := &starlark.Thread{Name: "transform"}

	// load() is rejected by the analyzer before we get here; disable it at
	// runtime too as defense in depth.
	thread.Load = func(*starlark.Thread, string) (starlark.StringDict, error) {
		return nil, errors.New("load() is not permitted")
	}

	predeclared := starlark.StringDict{"json": json.Module}

	// Watchdog: Starlark execution is single-goroutine and cooperatively
	// cancellable, so calling Cancel from a timer goroutine is safe.
	done := make(chan struct{})
	timedOut := false
	timer := time.AfterFunc(e.timeout, func() {
		timedOut = true
		thread.Cancel("timeout")
	})
	defer timer.Stop()
	defer close(done)

	opts := &syntax.FileOptions{}
	globals, err := starlark.ExecFileOptions(opts, thread, "transform.star", req.Code, predeclared)
	if err != nil {
		if timedOut {
			return ExecResult{Status: StatusTimeout, Stderr: "execution timed out"}
		}
		return ExecResult{Status: StatusNonzeroExit, Stderr: evalErr(err)}
	}

	fn, ok := globals["transform"].(starlark.Callable)
	if !ok {
		return ExecResult{
			Status: StatusNonzeroExit,
			Stderr: "code must define a top-level function transform(input) -> string",
		}
	}

	ret, err := starlark.Call(thread, fn, starlark.Tuple{starlark.String(req.Input)}, nil)
	if err != nil {
		if timedOut {
			return ExecResult{Status: StatusTimeout, Stderr: "execution timed out"}
		}
		return ExecResult{Status: StatusNonzeroExit, Stderr: evalErr(err)}
	}

	out, ok := starlark.AsString(ret)
	if !ok {
		return ExecResult{
			Status: StatusNonzeroExit,
			Stderr: fmt.Sprintf("transform() must return a string, got %s", ret.Type()),
		}
	}
	return ExecResult{Status: StatusOK, Output: out}
}

// evalErr renders a Starlark error with its backtrace when available so the
// LLM gets an actionable diagnostic for the one-shot fix.
func evalErr(err error) string {
	var ee *starlark.EvalError
	if errors.As(err, &ee) {
		return strings.TrimSpace(ee.Backtrace())
	}
	return err.Error()
}

var _ Engine = (*StarlarkEngine)(nil)

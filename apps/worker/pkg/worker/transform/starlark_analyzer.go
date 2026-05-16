package transform

import (
	"go.starlark.net/syntax"
)

// dangerousIdents floor at tier_3: pseudo-IO / dynamic-execution builtins.
var dangerousIdents = map[string]bool{
	"open": true,
	"eval": true,
	"exec": true,
}

// dangerousModules floor at tier_3 when used as `mod.attr` (e.g. os.environ).
var dangerousModules = map[string]bool{
	"os":     true,
	"socket": true,
}

// StarlarkAnalyzer parses Starlark source once and walks the AST to derive a
// floor tier. The single parse serves both syntax validation (step 0) and the
// static-analysis walk (step a) — see "判定パイプライン" in the design doc.
type StarlarkAnalyzer struct{}

func NewStarlarkAnalyzer() *StarlarkAnalyzer { return &StarlarkAnalyzer{} }

func (a *StarlarkAnalyzer) Analyze(code string) AnalysisResult {
	f, err := syntax.Parse("transform.star", code, 0)
	if err != nil {
		// Unparseable: reject before any execution so the LLM gets a precise
		// fix target (see syntax_error handling in the design doc).
		return AnalysisResult{SyntaxErr: true, Reason: err.Error()}
	}

	floor := Tier1
	var rejected bool
	var reason string

	bump := func(t Tier) {
		if tierRank(t) > tierRank(floor) {
			floor = t
		}
	}

	syntax.Walk(f, func(n syntax.Node) bool {
		switch v := n.(type) {
		case *syntax.LoadStmt:
			// External module loading is never allowed.
			rejected = true
			reason = "load() is not permitted in dynamic tools"
			return false
		case *syntax.CallExpr:
			if id, ok := v.Fn.(*syntax.Ident); ok && dangerousIdents[id.Name] {
				bump(Tier3)
			}
		case *syntax.DotExpr:
			if x, ok := v.X.(*syntax.Ident); ok {
				switch {
				case dangerousModules[x.Name]:
					bump(Tier3)
				case x.Name == "json" && (v.Name.Name == "decode" || v.Name.Name == "encode"):
					bump(Tier2)
				}
			}
		}
		return true
	})

	if rejected {
		return AnalysisResult{Rejected: true, Reason: reason}
	}
	return AnalysisResult{FloorTier: floor}
}

func tierRank(t Tier) int {
	switch t {
	case Tier3:
		return 3
	case Tier2:
		return 2
	default:
		return 1
	}
}

// ensure interface satisfaction at compile time.
var _ Analyzer = (*StarlarkAnalyzer)(nil)

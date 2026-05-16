package starlarkrt

import (
	"fmt"
	"strings"

	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
	"go.starlark.net/syntax"
)

type staticAnalyzer struct {
	floor       treev1.RiskTier
	diagnostics []Diagnostic
}

func (a *staticAnalyzer) walk(file *syntax.File) {
	syntax.Walk(file, func(n syntax.Node) bool {
		switch node := n.(type) {
		case *syntax.LoadStmt:
			a.raise(node.Load, treev1.RiskTier_RISK_TIER_3, "load statements are not allowed in embedded Starlark transforms")
		case *syntax.CallExpr:
			a.inspectCall(node)
		}
		return true
	})
}

func (a *staticAnalyzer) inspectCall(call *syntax.CallExpr) {
	name := callName(call.Fn)
	if name == "" {
		return
	}
	switch {
	case isTier3Call(name):
		a.raise(syntax.Start(call), treev1.RiskTier_RISK_TIER_3, fmt.Sprintf("call to %s requires tier_3", name))
	case isTier2Call(name):
		a.raise(syntax.Start(call), treev1.RiskTier_RISK_TIER_2, fmt.Sprintf("call to %s requires tier_2", name))
	}
}

func callName(expr syntax.Expr) string {
	switch x := expr.(type) {
	case *syntax.Ident:
		return x.Name
	case *syntax.DotExpr:
		prefix := callName(x.X)
		if prefix == "" || x.Name == nil {
			return ""
		}
		return prefix + "." + x.Name.Name
	default:
		return ""
	}
}

func isTier3Call(name string) bool {
	switch name {
	case "eval", "exec", "compile", "__import__", "open", "read", "write", "delete", "remove", "glob":
		return true
	}
	prefixes := []string{
		"os.", "sys.", "socket.", "http.", "urllib.", "requests.", "subprocess.", "shutil.", "pathlib.",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func isTier2Call(name string) bool {
	switch name {
	case "json.decode", "json.encode", "json.indent":
		return true
	}
	prefixes := []string{"csv.", "re."}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func (a *staticAnalyzer) raise(pos syntax.Position, tier treev1.RiskTier, message string) {
	if tierRank(tier) > tierRank(a.floor) {
		a.floor = tier
	}
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Tier:    tier,
		Message: message,
		Line:    pos.Line,
		Column:  pos.Col,
	})
}

func tierRank(tier treev1.RiskTier) int {
	switch tier {
	case treev1.RiskTier_RISK_TIER_3:
		return 3
	case treev1.RiskTier_RISK_TIER_2:
		return 2
	default:
		return 1
	}
}

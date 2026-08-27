package stif

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Imported content is rendered in an iframe that runs a small bootstrap script,
// so the iframe's own origin is not a sufficient boundary: anything that can
// execute or phone home has to be removed before the content is stored.
// Sanitizing happens at import time (not at render time) so what the preview
// shows and what the database holds are the same bytes.

// allowedElements is an allowlist. Anything outside it is either dropped with
// its subtree (dropWithSubtree) or unwrapped, keeping its text.
var allowedElements = map[string]bool{
	"article": true, "section": true, "div": true, "span": true, "p": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"hgroup": true, "header": true, "footer": true, "main": true, "aside": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true, "tr": true,
	"th": true, "td": true, "caption": true, "colgroup": true, "col": true,
	"code": true, "pre": true, "blockquote": true, "q": true, "cite": true,
	"strong": true, "em": true, "b": true, "i": true, "u": true, "s": true,
	"del": true, "ins": true, "small": true, "sup": true, "sub": true,
	"mark": true, "abbr": true, "time": true, "kbd": true, "samp": true, "var": true,
	"a": true, "img": true, "br": true, "hr": true,
	"figure": true, "figcaption": true, "details": true, "summary": true,
}

// dropWithSubtree lists elements whose contents are as dangerous as the tag —
// unwrapping them would paste script or style text into the document body.
var dropWithSubtree = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true,
	"embed": true, "applet": true, "frame": true, "frameset": true,
	"noscript": true, "template": true, "svg": true, "math": true,
	"form": true, "input": true, "button": true, "select": true,
	"textarea": true, "option": true, "link": true, "meta": true, "base": true,
}

// globalAttributes are allowed on any permitted element. data-* is allowed
// too (handled in isAllowedAttribute): the paper renderer drives child-paper
// navigation off data-paper-id and lazy images off data-file-id.
var globalAttributes = map[string]bool{
	"class": true, "id": true, "title": true, "lang": true, "dir": true, "role": true,
}

// elementAttributes are allowed only on specific elements.
var elementAttributes = map[string]map[string]bool{
	"a":       {"href": true, "rel": true, "target": true},
	"img":     {"src": true, "alt": true, "width": true, "height": true, "loading": true},
	"td":      {"colspan": true, "rowspan": true, "headers": true},
	"th":      {"colspan": true, "rowspan": true, "headers": true, "scope": true},
	"col":     {"span": true},
	"ol":      {"start": true, "type": true, "reversed": true},
	"time":    {"datetime": true},
	"details": {"open": true},
	"q":       {"cite": true},
}

func sanitizeItem(item *Item, diags *diagList) {
	if item.Content != "" {
		content, removed := sanitizeHTML(item.Content)
		item.Content = content
		reportRemovals(diags, item, "unsafe_html", removed)
	}
	if item.OverrideCSS != "" {
		css, removed := sanitizeCSS(item.OverrideCSS)
		item.OverrideCSS = css
		reportRemovals(diags, item, "unsafe_css", removed)
	}
}

func reportRemovals(diags *diagList, item *Item, code string, removed []string) {
	for _, what := range dedupe(removed) {
		diags.warnf(item.Line, item.LocalPath, code, fmt.Sprintf("removed %s during import sanitizing", what))
	}
}

func dedupe(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// sanitizeHTML parses the fragment, filters it against the allowlist and
// re-renders it. Re-rendering rather than string-editing means malformed input
// cannot smuggle a tag past the filter: whatever the parser saw is what gets
// written back out.
func sanitizeHTML(fragment string) (string, []string) {
	context := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), context)
	if err != nil {
		// The fragment could not be parsed at all; drop the markup rather than
		// store something no parser agreed on.
		return "", []string{"unparseable HTML content"}
	}

	var removed []string
	var out strings.Builder
	for _, node := range nodes {
		for _, clean := range sanitizeNode(node, &removed) {
			if err := html.Render(&out, clean); err != nil {
				return "", []string{"unrenderable HTML content"}
			}
		}
	}
	return out.String(), removed
}

// sanitizeNode returns the nodes that should replace n: n itself when it is
// allowed, its sanitized children when n is unwrapped, or nothing when n is
// dropped outright.
func sanitizeNode(n *html.Node, removed *[]string) []*html.Node {
	switch n.Type {
	case html.TextNode:
		return []*html.Node{{Type: html.TextNode, Data: n.Data}}
	case html.ElementNode:
		// fall through
	default:
		// Comments, doctypes and anything else carry no content worth keeping.
		return nil
	}

	tag := strings.ToLower(n.Data)
	if dropWithSubtree[tag] {
		*removed = append(*removed, fmt.Sprintf("<%s> and its contents", tag))
		return nil
	}

	children := sanitizeChildren(n, removed)
	if !allowedElements[tag] {
		*removed = append(*removed, fmt.Sprintf("<%s> (kept its contents)", tag))
		return children
	}

	clean := &html.Node{Type: html.ElementNode, Data: tag, DataAtom: n.DataAtom}
	clean.Attr = sanitizeAttributes(tag, n.Attr, removed)
	for _, child := range children {
		clean.AppendChild(child)
	}
	return []*html.Node{clean}
}

func sanitizeChildren(n *html.Node, removed *[]string) []*html.Node {
	var out []*html.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		out = append(out, sanitizeNode(child, removed)...)
	}
	return out
}

func sanitizeAttributes(tag string, attrs []html.Attribute, removed *[]string) []html.Attribute {
	out := make([]html.Attribute, 0, len(attrs))
	for _, attr := range attrs {
		name := strings.ToLower(attr.Key)
		if !isAllowedAttribute(tag, name) {
			*removed = append(*removed, fmt.Sprintf("the %s attribute", name))
			continue
		}
		if isURLAttribute(name) && !isSafeURL(attr.Val) {
			*removed = append(*removed, fmt.Sprintf("an unsafe %s URL", name))
			continue
		}
		if name == "style" {
			cleaned, dropped := sanitizeCSSDeclarations(attr.Val)
			*removed = append(*removed, dropped...)
			if strings.TrimSpace(cleaned) == "" {
				continue
			}
			attr.Val = cleaned
		}
		attr.Key = name
		attr.Namespace = ""
		out = append(out, attr)
	}
	return out
}

func isAllowedAttribute(tag, name string) bool {
	// on* handlers are the single most important thing to strip, and they are
	// never legitimate in imported content.
	if strings.HasPrefix(name, "on") {
		return false
	}
	if strings.HasPrefix(name, "data-") || strings.HasPrefix(name, "aria-") {
		return true
	}
	if name == "style" {
		return true
	}
	if globalAttributes[name] {
		return true
	}
	return elementAttributes[tag][name]
}

func isURLAttribute(name string) bool {
	return name == "href" || name == "src" || name == "cite"
}

// isSafeURL allows same-document and http(s) references plus inline images.
// Anything whose scheme can execute (javascript:, vbscript:) or impersonate a
// document (data:text/html) is rejected.
func isSafeURL(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	// Strip characters browsers ignore when resolving a scheme.
	trimmed = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r == 0 {
			return -1
		}
		return r
	}, trimmed)

	scheme, _, hasScheme := strings.Cut(trimmed, ":")
	if !hasScheme || strings.ContainsAny(scheme, "/?#") {
		// Relative URL, fragment or query: no scheme to worry about.
		return true
	}
	switch scheme {
	case "http", "https", "mailto":
		return true
	case "data":
		return strings.HasPrefix(trimmed, "data:image/")
	default:
		return false
	}
}

// cssHazards are substrings that make a CSS declaration or at-rule capable of
// fetching, executing or binding behaviour.
var cssHazards = []string{
	"javascript:", "vbscript:", "expression(", "behavior", "-moz-binding",
	"@import", "@charset", "@namespace",
}

// sanitizeCSS cleans an ::css override block. It works rule by rule so a single
// bad declaration does not cost the author their whole stylesheet.
func sanitizeCSS(css string) (string, []string) {
	var removed []string
	css, comments := stripCSSComments(css)
	removed = append(removed, comments...)

	var out strings.Builder
	rest := css
	for {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			// Trailing text outside any rule: bare at-rules land here.
			trailing, dropped := sanitizeRulePrelude(rest)
			removed = append(removed, dropped...)
			out.WriteString(trailing)
			break
		}
		close := strings.IndexByte(rest[open:], '}')
		if close < 0 {
			removed = append(removed, "an unterminated CSS rule")
			break
		}
		prelude := rest[:open]
		body := rest[open+1 : open+close]
		rest = rest[open+close+1:]

		// The prelude holds the selector, but it can also carry `;`-terminated
		// at-rules that were written before it (@import is the one that
		// matters). Filter those out individually so a bad @import does not
		// take the following rule down with it.
		selector, dropped := sanitizeRulePrelude(prelude)
		removed = append(removed, dropped...)
		if selector == "" {
			continue
		}
		cleanBody, dropped := sanitizeCSSDeclarations(body)
		removed = append(removed, dropped...)
		if strings.TrimSpace(cleanBody) == "" {
			continue
		}
		out.WriteString(selector)
		out.WriteString("{")
		out.WriteString(cleanBody)
		out.WriteString("}")
	}
	return strings.TrimSpace(out.String()), removed
}

// sanitizeRulePrelude filters the text before a rule's `{`, returning the
// selector to keep (empty when the whole rule must go) plus what was dropped.
// Statements before the selector are `;`-terminated at-rules.
func sanitizeRulePrelude(prelude string) (string, []string) {
	var removed []string
	statements := splitTopLevel(prelude, ';')
	// The last segment is the selector itself (or trailing whitespace when the
	// prelude was only at-rules); everything before it is a complete statement.
	selector := statements[len(statements)-1]
	kept := make([]string, 0, len(statements))
	for _, statement := range statements[:len(statements)-1] {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if hazard, found := findCSSHazard(statement); found {
			removed = append(removed, fmt.Sprintf("a CSS at-rule using %s", hazard))
			continue
		}
		kept = append(kept, strings.TrimSpace(statement)+";")
	}
	if hazard, found := findCSSHazard(selector); found {
		removed = append(removed, fmt.Sprintf("a CSS rule using %s", hazard))
		return "", removed
	}
	return strings.Join(kept, "") + selector, removed
}

// splitTopLevel splits on sep while ignoring separators inside parentheses or
// quotes. `background: url(data:image/png;base64,...)` is one declaration, not
// two, and splitting it in half would corrupt the value.
func splitTopLevel(s string, sep byte) []string {
	var out []string
	depth := 0
	var quote byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case c == sep && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// sanitizeCSSDeclarations filters a `a: b; c: d` declaration list.
func sanitizeCSSDeclarations(body string) (string, []string) {
	var removed []string
	kept := make([]string, 0, 4)
	for _, decl := range splitTopLevel(body, ';') {
		if strings.TrimSpace(decl) == "" {
			continue
		}
		if hazard, found := findCSSHazard(decl); found {
			removed = append(removed, fmt.Sprintf("a CSS declaration using %s", hazard))
			continue
		}
		if url, ok := findUnsafeCSSURL(decl); ok {
			removed = append(removed, fmt.Sprintf("a CSS url(%s) reference", url))
			continue
		}
		kept = append(kept, decl)
	}
	if len(kept) == 0 {
		return "", removed
	}
	return strings.Join(kept, ";") + ";", removed
}

func findCSSHazard(s string) (string, bool) {
	lowered := strings.ToLower(s)
	for _, hazard := range cssHazards {
		if strings.Contains(lowered, hazard) {
			return hazard, true
		}
	}
	return "", false
}

// findUnsafeCSSURL reports a url() target that would reach outside the allowed
// schemes. Remote url() in a stylesheet is a side channel (it fires a request
// the moment the rule matches), so only inline data images and https survive.
func findUnsafeCSSURL(decl string) (string, bool) {
	lowered := strings.ToLower(decl)
	for idx := 0; ; {
		found := strings.Index(lowered[idx:], "url(")
		if found < 0 {
			return "", false
		}
		start := idx + found + len("url(")
		end := strings.IndexByte(lowered[start:], ')')
		if end < 0 {
			return "unterminated", true
		}
		target := strings.Trim(strings.TrimSpace(lowered[start:start+end]), `"'`)
		if !isSafeURL(target) || strings.HasPrefix(target, "http:") {
			return truncate(target, 40), true
		}
		idx = start + end
	}
}

func stripCSSComments(css string) (string, []string) {
	var removed []string
	for {
		start := strings.Index(css, "/*")
		if start < 0 {
			return css, removed
		}
		end := strings.Index(css[start:], "*/")
		if end < 0 {
			removed = append(removed, "an unterminated CSS comment")
			return css[:start], removed
		}
		css = css[:start] + " " + css[start+end+2:]
	}
}

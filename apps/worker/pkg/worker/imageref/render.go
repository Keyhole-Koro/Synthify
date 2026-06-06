package imageref

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
)

// tokenPattern matches the [[img:N]] placeholders the LLM is told to emit.
// Whitespace around the number is tolerated since models drift on formatting.
var tokenPattern = regexp.MustCompile(`\[\[\s*img\s*:\s*(\d+)\s*\]\]`)

// rawImgPattern matches any <img ...> tag the model wrote directly, in
// violation of the prompt. We strip these wholesale rather than trust their
// src: the whole point of the registry is that the model never authors a URL.
var rawImgPattern = regexp.MustCompile(`(?is)<img\b[^>]*>`)

// Render rewrites image placeholders in LLM-authored HTML into data-file-id
// markers, using only file ids from the registry. It is the single chokepoint
// that guarantees no hallucinated image — and no auth-bearing URL — survives
// into saved HTML:
//
//   - [[img:N]] for a registered N becomes
//     <img data-file-id="..." alt="..." loading="lazy">.
//   - [[img:N]] for an unregistered N is dropped (replaced with "").
//   - Any <img> tag the model wrote itself is stripped.
//
// The marker has no src: the web client swaps data-file-id for a short-lived
// signed URL at display time. file id and alt are HTML-escaped. A nil/empty
// registry simply strips all tokens and raw <img> tags, leaving plain text.
func Render(htmlContent string, reg *Registry) string {
	// Strip raw <img> first so a model-authored tag can never leak a URL even
	// if it also happens to contain a [[img:N]]-looking string in an attribute.
	out := rawImgPattern.ReplaceAllString(htmlContent, "")

	out = tokenPattern.ReplaceAllStringFunc(out, func(match string) string {
		m := tokenPattern.FindStringSubmatch(match)
		if len(m) != 2 {
			return ""
		}
		id, err := strconv.Atoi(m[1])
		if err != nil {
			return ""
		}
		img, ok := reg.Lookup(id)
		if !ok || img.FileID == "" {
			return ""
		}
		return fmt.Sprintf(
			`<img data-file-id="%s" alt="%s" loading="lazy">`,
			html.EscapeString(img.FileID),
			html.EscapeString(img.Alt),
		)
	})

	return out
}

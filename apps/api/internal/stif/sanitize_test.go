package stif

import (
	"strings"
	"testing"
)

func sanitizeContent(t *testing.T, content string) string {
	t.Helper()
	src := "#STIF/1\n::item /a\ntitle: A\n---\n" + content + "\n::end\n"
	doc, _ := Parse(src)
	return findItem(t, doc, "/a").Content
}

func sanitizeOverrideCSS(t *testing.T, css string) string {
	t.Helper()
	src := "#STIF/1\n::item /a\ntitle: A\n::css\n" + css + "\n::end\n"
	doc, _ := Parse(src)
	return findItem(t, doc, "/a").OverrideCSS
}

func TestSanitizeDropsScriptAndItsContents(t *testing.T) {
	got := sanitizeContent(t, `<p>ok</p><script>alert('x')</script>`)
	if strings.Contains(got, "script") || strings.Contains(got, "alert") {
		t.Errorf("script survived sanitizing: %q", got)
	}
	if !strings.Contains(got, "<p>ok</p>") {
		t.Errorf("legitimate markup was lost: %q", got)
	}
}

func TestSanitizeDropsDangerousElementsWithTheirContents(t *testing.T) {
	for _, tag := range []string{"iframe", "object", "form", "style", "noscript", "template", "svg", "textarea", "select"} {
		content := "<" + tag + ">payload</" + tag + "><p>kept</p>"
		got := sanitizeContent(t, content)
		if strings.Contains(got, "<"+tag) || strings.Contains(got, "payload") {
			t.Errorf("%s survived sanitizing: %q", tag, got)
		}
		if !strings.Contains(got, "kept") {
			t.Errorf("%s: sanitizing ate the sibling content: %q", tag, got)
		}
	}
}

// Void elements have no contents to drop — only the tag itself has to go.
func TestSanitizeDropsDangerousVoidElements(t *testing.T) {
	for _, tag := range []string{"embed", "input", "link", "meta", "base"} {
		got := sanitizeContent(t, "<"+tag+" src=\"x\"><p>kept</p>")
		if strings.Contains(got, "<"+tag) {
			t.Errorf("%s survived sanitizing: %q", tag, got)
		}
		if !strings.Contains(got, "kept") {
			t.Errorf("%s: sanitizing ate the sibling content: %q", tag, got)
		}
	}
}

func TestSanitizeStripsEventHandlers(t *testing.T) {
	got := sanitizeContent(t, `<p onclick="steal()" onmouseover="x()">text</p>`)
	if strings.Contains(got, "onclick") || strings.Contains(got, "onmouseover") || strings.Contains(got, "steal") {
		t.Errorf("event handler survived: %q", got)
	}
	if !strings.Contains(got, "text") {
		t.Errorf("text was lost: %q", got)
	}
}

func TestSanitizeStripsJavascriptURLs(t *testing.T) {
	for _, href := range []string{
		`javascript:alert(1)`,
		`JaVaScRiPt:alert(1)`,
		"java\tscript:alert(1)",
		`vbscript:msgbox`,
		`data:text/html;base64,PHNjcmlwdD4=`,
	} {
		got := sanitizeContent(t, `<a href="`+href+`">link</a>`)
		if strings.Contains(got, "href") {
			t.Errorf("href %q survived: %q", href, got)
		}
		if !strings.Contains(got, "link") {
			t.Errorf("href %q: link text was lost: %q", href, got)
		}
	}
}

func TestSanitizeKeepsSafeURLs(t *testing.T) {
	for _, href := range []string{
		"https://example.com/docs",
		"http://example.com/docs",
		"mailto:someone@example.com",
		"#section",
		"/relative/path",
	} {
		got := sanitizeContent(t, `<a href="`+href+`">link</a>`)
		if !strings.Contains(got, "href") {
			t.Errorf("safe href %q was stripped: %q", href, got)
		}
	}
}

// The paper renderer navigates to child papers via data-paper-id and lazy-loads
// images via data-file-id, so imported content has to keep data-* attributes.
func TestSanitizeKeepsDataAndAriaAttributes(t *testing.T) {
	got := sanitizeContent(t, `<a data-paper-id="item-1" aria-label="open">child</a><img data-file-id="f1" alt="fig">`)
	for _, want := range []string{`data-paper-id="item-1"`, `aria-label="open"`, `data-file-id="f1"`} {
		if !strings.Contains(got, want) {
			t.Errorf("%s was stripped: %q", want, got)
		}
	}
}

func TestSanitizeUnwrapsUnknownElementsKeepingText(t *testing.T) {
	got := sanitizeContent(t, `<marquee><p>still here</p></marquee>`)
	if strings.Contains(got, "marquee") {
		t.Errorf("unknown element survived: %q", got)
	}
	if !strings.Contains(got, "<p>still here</p>") {
		t.Errorf("unwrapping lost the contents: %q", got)
	}
}

// Re-rendering from the parse tree, rather than editing the string, is what
// stops malformed markup from smuggling a tag past the filter.
func TestSanitizeNeutralisesMalformedMarkup(t *testing.T) {
	got := sanitizeContent(t, `<p>text<scr<script>ipt>alert(1)</script></p>`)
	if strings.Contains(strings.ToLower(got), "<script") {
		t.Errorf("malformed markup produced a script tag: %q", got)
	}
}

func TestSanitizeReportsRemovalsAsWarnings(t *testing.T) {
	src := "#STIF/1\n::item /a\ntitle: A\n---\n<script>x</script><p>ok</p>\n::end\n"
	_, diags := Parse(src)
	if !hasDiagnostic(diags, "unsafe_html") {
		t.Fatalf("want unsafe_html warning, got %+v", diags)
	}
	if HasErrors(diags) {
		t.Error("sanitizing must warn, not block the import")
	}
}

func TestSanitizeCSSDropsImport(t *testing.T) {
	got := sanitizeOverrideCSS(t, "@import url(https://evil.example/x.css);\np { color: red; }")
	if strings.Contains(got, "@import") {
		t.Errorf("@import survived: %q", got)
	}
	if !strings.Contains(got, "color: red") {
		t.Errorf("the legitimate rule was lost: %q", got)
	}
}

func TestSanitizeCSSDropsHazardousDeclarations(t *testing.T) {
	css := "p { width: expression(alert(1)); color: red; }\n" +
		"a { behavior: url(x.htc); font-size: 12px; }\n" +
		"b { background: url(javascript:alert(1)); }"
	got := sanitizeOverrideCSS(t, css)

	for _, hazard := range []string{"expression(", "behavior", "javascript:"} {
		if strings.Contains(strings.ToLower(got), hazard) {
			t.Errorf("%s survived: %q", hazard, got)
		}
	}
	// Sibling declarations in the same rule must survive: one bad line should
	// not cost the author their whole stylesheet.
	if !strings.Contains(got, "color: red") || !strings.Contains(got, "font-size: 12px") {
		t.Errorf("safe declarations were lost: %q", got)
	}
}

// A remote url() fires a request the moment the rule matches, which is a
// side channel out of the iframe.
func TestSanitizeCSSDropsRemoteURLs(t *testing.T) {
	got := sanitizeOverrideCSS(t, "p { background: url(http://tracker.example/pixel.png); color: blue; }")
	if strings.Contains(got, "tracker.example") {
		t.Errorf("remote url() survived: %q", got)
	}
	if !strings.Contains(got, "color: blue") {
		t.Errorf("safe declaration was lost: %q", got)
	}
}

func TestSanitizeCSSKeepsInlineDataImages(t *testing.T) {
	css := "p { background: url(data:image/png;base64,iVBORw0KGgo=); }"
	got := sanitizeOverrideCSS(t, css)
	if !strings.Contains(got, "data:image/png") {
		t.Errorf("inline data image was stripped: %q", got)
	}
}

func TestSanitizeCSSStripsComments(t *testing.T) {
	got := sanitizeOverrideCSS(t, "/* a note */ p { color: red; }")
	if strings.Contains(got, "a note") {
		t.Errorf("comment survived: %q", got)
	}
	if !strings.Contains(got, "color: red") {
		t.Errorf("rule was lost: %q", got)
	}
}

func TestSanitizeStyleAttributeIsFiltered(t *testing.T) {
	got := sanitizeContent(t, `<p style="color: red; width: expression(alert(1))">text</p>`)
	if strings.Contains(strings.ToLower(got), "expression(") {
		t.Errorf("hazardous style declaration survived: %q", got)
	}
	if !strings.Contains(got, "color: red") {
		t.Errorf("safe style declaration was lost: %q", got)
	}
}

func TestSanitizeDropsStyleAttributeWhenNothingSurvives(t *testing.T) {
	got := sanitizeContent(t, `<p style="behavior: url(x.htc)">text</p>`)
	if strings.Contains(got, "style") {
		t.Errorf("an empty style attribute was left behind: %q", got)
	}
}

func TestSanitizeLeavesCleanContentUntouched(t *testing.T) {
	content := `<article><h2>Key Points</h2><ul><li>one</li><li>two</li></ul><pre><code>x := 1</code></pre></article>`
	if got := sanitizeContent(t, content); got != content {
		t.Errorf("clean content was rewritten:\n got %q\nwant %q", got, content)
	}
}

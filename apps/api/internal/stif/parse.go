package stif

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse reads a STIF document and returns its items plus every problem found.
// The returned Document is usable only when HasErrors(diags) is false.
func Parse(src string) (*Document, []Diagnostic) {
	var diags diagList
	lines := splitLines(stripFence(strings.TrimPrefix(src, "\ufeff")))

	doc := &Document{
		Version: "1",
		Mode:    DefaultMode,
		Scope:   DefaultScope,
	}

	i := parseHeader(lines, doc, &diags)
	items := parseItemBlocks(lines, i, &diags)

	for idx := range items {
		validateSources(&items[idx], &diags)
		sanitizeItem(&items[idx], &diags)
	}

	doc.Items = resolve(items, &diags)
	return doc, diags.items
}

// stripFence removes a Markdown code fence wrapping the whole document. The
// conversion prompt tells the model not to emit one, but models do it anyway
// and the file is otherwise perfectly good.
func stripFence(src string) string {
	trimmed := strings.TrimSpace(src)
	if !strings.HasPrefix(trimmed, "```") {
		return src
	}
	newline := strings.IndexByte(trimmed, '\n')
	if newline < 0 {
		return src
	}
	// Only strip when the first line is a bare fence (```/```stif), not code.
	if strings.ContainsAny(strings.TrimSpace(trimmed[3:newline]), " \t") {
		return src
	}
	body := trimmed[newline+1:]
	if end := strings.LastIndex(body, "```"); end >= 0 {
		body = body[:end]
	}
	return body
}

func splitLines(src string) []string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	return strings.Split(src, "\n")
}

// parseHeader consumes `#STIF/1` and an optional `@tree` line, returning the
// index of the first line after them.
func parseHeader(lines []string, doc *Document, diags *diagList) int {
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		diags.errorf(0, "", "empty_document", "the document is empty")
		return i
	}

	header := strings.TrimSpace(lines[i])
	version, ok := strings.CutPrefix(header, "#STIF/")
	if !ok {
		diags.errorf(i+1, "", "missing_header", fmt.Sprintf("the first line must be %q, got %q", "#STIF/1", header))
		return i
	}
	if version != "1" {
		diags.errorf(i+1, "", "unsupported_version", fmt.Sprintf("unsupported STIF version %q; this server understands version 1", version))
		return i + 1
	}
	doc.Version = version
	i++

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if !strings.HasPrefix(line, "@tree") {
			break
		}
		parseTreeDirective(line, i+1, doc, diags)
		i++
	}
	return i
}

// parseTreeDirective reads `@tree key="value" ...`.
func parseTreeDirective(line string, lineNo int, doc *Document, diags *diagList) {
	for _, pair := range splitAttributes(strings.TrimSpace(strings.TrimPrefix(line, "@tree"))) {
		key, rawValue, ok := strings.Cut(pair, "=")
		if !ok {
			diags.warnf(lineNo, "", "malformed_tree_attribute", fmt.Sprintf("ignoring malformed @tree attribute %q; expected key=\"value\"", pair))
			continue
		}
		key = strings.TrimSpace(key)
		value := unquote(strings.TrimSpace(rawValue))
		switch key {
		case "workspace":
			doc.WorkspaceID = value
		case "mode":
			switch value {
			case ModeAppend:
				doc.Mode = value
			case ModeReplace, ModeMerge:
				diags.errorf(lineNo, "", "unsupported_mode", fmt.Sprintf("mode %q is not implemented yet; only %q is supported", value, ModeAppend))
			default:
				diags.errorf(lineNo, "", "unknown_mode", fmt.Sprintf("unknown mode %q", value))
			}
		case "scope":
			if !validScopes[value] {
				diags.errorf(lineNo, "", "unknown_scope", fmt.Sprintf("unknown scope %q; expected one of document, canonical", value))
				continue
			}
			doc.Scope = value
		default:
			diags.warnf(lineNo, "", "unknown_tree_attribute", fmt.Sprintf("ignoring unknown @tree attribute %q", key))
		}
	}
}

// splitAttributes splits `a="x y" b="z"` respecting double quotes.
func splitAttributes(s string) []string {
	var out []string
	var current strings.Builder
	inQuotes := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case (r == ' ' || r == '\t') && !inQuotes:
			if current.Len() > 0 {
				out = append(out, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if unquoted, err := strconv.Unquote(s); err == nil {
			return unquoted
		}
		return s[1 : len(s)-1]
	}
	return s
}

// blockSection tracks which part of an `::item` block the scanner is in.
type blockSection int

const (
	sectionMetadata blockSection = iota
	sectionContent
	sectionCSS
)

func parseItemBlocks(lines []string, start int, diags *diagList) []Item {
	var items []Item

	for i := start; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "::item") {
			diags.errorf(i+1, "", "unexpected_line", fmt.Sprintf("expected an ::item block, got %q", truncate(trimmed, 60)))
			continue
		}

		item, next := parseItemBlock(lines, i, diags)
		items = append(items, item)
		i = next
	}
	return items
}

// parseItemBlock reads one block starting at the `::item` header line and
// returns the item plus the index of its `::end` (or of the last line consumed
// when the block was left unterminated).
func parseItemBlock(lines []string, start int, diags *diagList) (Item, int) {
	header := strings.TrimSpace(lines[start])
	localPath := strings.TrimSpace(strings.TrimPrefix(header, "::item"))

	item := Item{
		LocalPath: localPath,
		State:     DefaultState,
		Scope:     DefaultScope,
		Line:      start + 1,
	}
	if localPath == "" {
		diags.errorf(start+1, "", "missing_local_path", "::item requires a local path, e.g. ::item /overview")
	} else if !strings.HasPrefix(localPath, "/") {
		diags.errorf(start+1, localPath, "invalid_local_path", fmt.Sprintf("local path %q must start with %q", localPath, "/"))
	} else if strings.ContainsAny(localPath, " \t") {
		diags.errorf(start+1, localPath, "invalid_local_path", fmt.Sprintf("local path %q must not contain whitespace", localPath))
	}

	var contentLines, cssLines []string
	section := sectionMetadata
	// inSource is true while consuming the indented entries of a `source:` key.
	inSource := false
	terminated := false

	i := start + 1
	for ; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)

		if trimmed == "::end" {
			terminated = true
			break
		}
		if trimmed == "::css" && section != sectionCSS {
			section = sectionCSS
			inSource = false
			continue
		}
		if section == sectionMetadata && trimmed == "---" {
			section = sectionContent
			inSource = false
			continue
		}
		if strings.HasPrefix(trimmed, "::item") {
			// An unterminated block: recover by ending it here so the rest of
			// the document still parses, and report the missing terminator.
			diags.errorf(item.Line, item.LocalPath, "unterminated_block", fmt.Sprintf("::item %s is missing its ::end", item.LocalPath))
			i--
			break
		}

		switch section {
		case sectionContent:
			contentLines = append(contentLines, raw)
		case sectionCSS:
			cssLines = append(cssLines, raw)
		case sectionMetadata:
			inSource = parseMetadataLine(raw, i+1, &item, inSource, diags)
		}
	}

	if !terminated && i >= len(lines) {
		diags.errorf(item.Line, item.LocalPath, "unterminated_block", fmt.Sprintf("::item %s is missing its ::end", item.LocalPath))
		i = len(lines) - 1
	}

	item.Content = trimBlankEdges(contentLines)
	item.OverrideCSS = trimBlankEdges(cssLines)

	if item.Title == "" {
		diags.errorf(item.Line, item.LocalPath, "missing_title", fmt.Sprintf("::item %s is missing the required title field", item.LocalPath))
	}
	return item, i
}

// parseMetadataLine handles one metadata line and returns the new inSource
// state. Source entries are indented under a `source:` key:
//
//	source:
//	  - type: github
//	    repo: owner/repo
func parseMetadataLine(raw string, lineNo int, item *Item, inSource bool, diags *diagList) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return inSource
	}
	indented := raw != trimmed && (raw[0] == ' ' || raw[0] == '\t')

	if inSource && indented {
		parseSourceLine(trimmed, lineNo, item, diags)
		return true
	}

	key, value, ok := strings.Cut(trimmed, ":")
	if !ok {
		diags.warnf(lineNo, item.LocalPath, "malformed_metadata", fmt.Sprintf("ignoring metadata line %q; expected key: value", truncate(trimmed, 60)))
		return false
	}
	key = strings.TrimSpace(key)
	value = unquote(strings.TrimSpace(value))

	switch key {
	case "title":
		item.Title = value
	case "description":
		item.Description = value
	case "parent":
		item.ParentLocalPath = value
	case "state":
		if !validStates[value] {
			diags.errorf(lineNo, item.LocalPath, "unknown_state", fmt.Sprintf("unknown state %q; expected one of system_generated, pending_review, human_curated, locked", value))
			break
		}
		item.State = value
	case "scope":
		if !validScopes[value] {
			diags.errorf(lineNo, item.LocalPath, "unknown_scope", fmt.Sprintf("unknown scope %q; expected one of document, canonical", value))
			break
		}
		item.Scope = value
	case "cross_document":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			diags.warnf(lineNo, item.LocalPath, "malformed_bool", fmt.Sprintf("ignoring cross_document %q; expected true or false", value))
			break
		}
		item.CrossDocument = parsed
	case "order":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			diags.warnf(lineNo, item.LocalPath, "malformed_int", fmt.Sprintf("ignoring order %q; expected an integer", value))
			break
		}
		item.Order = parsed
	case "merge_key":
		item.MergeKey = value
		diags.warnf(lineNo, item.LocalPath, "merge_key_ignored", "merge_key is recorded but has no effect: merge mode is not implemented yet")
	case "source":
		if value != "" {
			diags.warnf(lineNo, item.LocalPath, "malformed_source", "source takes an indented list of entries, not an inline value")
			return false
		}
		return true
	default:
		diags.warnf(lineNo, item.LocalPath, "unknown_metadata", fmt.Sprintf("ignoring unknown metadata key %q", key))
	}
	return false
}

func parseSourceLine(trimmed string, lineNo int, item *Item, diags *diagList) {
	if entry, ok := strings.CutPrefix(trimmed, "-"); ok {
		item.Sources = append(item.Sources, Source{})
		trimmed = strings.TrimSpace(entry)
		if trimmed == "" {
			return
		}
	}
	if len(item.Sources) == 0 {
		diags.warnf(lineNo, item.LocalPath, "malformed_source", fmt.Sprintf("ignoring source field %q: it does not belong to a %q entry", truncate(trimmed, 40), "- "))
		return
	}
	source := &item.Sources[len(item.Sources)-1]

	key, value, ok := strings.Cut(trimmed, ":")
	if !ok {
		diags.warnf(lineNo, item.LocalPath, "malformed_source", fmt.Sprintf("ignoring source line %q; expected key: value", truncate(trimmed, 60)))
		return
	}
	key = strings.TrimSpace(key)
	value = unquote(strings.TrimSpace(value))

	switch key {
	case "type":
		if !validSourceTypes[value] {
			diags.errorf(lineNo, item.LocalPath, "unknown_source_type", fmt.Sprintf("unknown source type %q; expected one of github, web, local_file, chat, document", value))
			return
		}
		source.Type = value
	case "url":
		source.URL = value
	case "repo":
		source.Repo = value
	case "ref":
		source.Ref = value
	case "path":
		source.Path = value
	case "kind":
		source.Kind = value
	case "id":
		source.ExternalID = value
	case "title":
		source.Title = value
	case "lines":
		start, end, ok := parseLineRange(value)
		if !ok {
			diags.warnf(lineNo, item.LocalPath, "malformed_line_range", fmt.Sprintf("ignoring lines %q; expected [start, end]", value))
			return
		}
		source.LineStart, source.LineEnd = start, end
	case "confidence":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			diags.warnf(lineNo, item.LocalPath, "malformed_float", fmt.Sprintf("ignoring confidence %q; expected a number between 0 and 1", value))
			return
		}
		if parsed < 0 || parsed > 1 {
			clamped := min(max(parsed, 0), 1)
			diags.warnf(lineNo, item.LocalPath, "confidence_out_of_range", fmt.Sprintf("clamping confidence %v into the 0..1 range", parsed))
			parsed = clamped
		}
		source.Confidence = parsed
	default:
		diags.warnf(lineNo, item.LocalPath, "unknown_source_field", fmt.Sprintf("ignoring unknown source field %q", key))
	}
}

// validateSources rejects source entries that cannot be stored. `type` is the
// one required field: it is a closed set the storage layer enforces too, so an
// entry without one would fail at insert time, after other items were written.
func validateSources(item *Item, diags *diagList) {
	for _, source := range item.Sources {
		if source.Type == "" {
			diags.errorf(item.Line, item.LocalPath, "missing_source_type",
				fmt.Sprintf("a source entry on %s has no type; expected one of github, web, local_file, chat, document", item.LocalPath))
		}
	}
}

// parseLineRange reads `[12, 48]`.
func parseLineRange(value string) (int, int, bool) {
	inner, ok := strings.CutPrefix(strings.TrimSpace(value), "[")
	if !ok {
		return 0, 0, false
	}
	inner, ok = strings.CutSuffix(inner, "]")
	if !ok {
		return 0, 0, false
	}
	startRaw, endRaw, ok := strings.Cut(inner, ",")
	if !ok {
		return 0, 0, false
	}
	start, err := strconv.Atoi(strings.TrimSpace(startRaw))
	if err != nil {
		return 0, 0, false
	}
	end, err := strconv.Atoi(strings.TrimSpace(endRaw))
	if err != nil {
		return 0, 0, false
	}
	return start, end, true
}

// trimBlankEdges drops leading and trailing blank lines while leaving inner
// indentation alone, so HTML and CSS keep the shape the author wrote.
func trimBlankEdges(lines []string) string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

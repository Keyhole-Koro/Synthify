// Package document provides text parsing, heading extraction, and semantic chunking utilities.
package document

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// NumberedHeadingPattern matches section headers starting with hierarchical numbers like "1.", "1.2", or "2.3.1".
var NumberedHeadingPattern = regexp.MustCompile(`^\s*(\d+(?:\.\d+)*)[\.\):]?\s+`)

// TextSection represents an extracted heading and its corresponding body text block.
type TextSection struct {
	Heading string
	Text    string
}

// BuildChunks decomposes raw document text into sequential DocumentChunk entities.
func BuildChunks(documentID, rawText string) []*domain.DocumentChunk {
	text := strings.TrimSpace(rawText)
	if text == "" {
		return nil
	}

	sections := SplitSections(text)
	chunks := make([]*domain.DocumentChunk, 0, len(sections))
	for i, section := range sections {
		heading := section.Heading
		if heading == "" {
			heading = fmt.Sprintf("Section %d", i+1)
		}
		chunks = append(chunks, &domain.DocumentChunk{
			ChunkID:    fmt.Sprintf("%s_chunk_%d", documentID, i),
			DocumentID: documentID,
			Heading:    heading,
			Text:       section.Text,
		})
	}
	return chunks
}

// SplitSections segments document text into sections based on recognized headings and character length bounds.
func SplitSections(text string) []TextSection {
	const maxRunes = 3500
	var sections []TextSection
	var currentHeading string
	var current strings.Builder
	sawHeading := false

	flush := func() {
		body := strings.TrimSpace(current.String())
		if body == "" {
			return
		}
		sections = append(sections, TextSection{Heading: currentHeading, Text: body})
		current.Reset()
	}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if LooksLikeHeading(trimmed) && current.Len() > 0 {
			flush()
			currentHeading = NormalizeHeading(trimmed)
			sawHeading = true
		}
		if currentHeading == "" && LooksLikeHeading(trimmed) {
			currentHeading = NormalizeHeading(trimmed)
			sawHeading = true
		}
		lineRunes := []rune(line)
		for len(lineRunes) > 0 {
			currentLen := utf8.RuneCountInString(current.String())
			if currentLen >= maxRunes {
				flush()
				currentLen = 0
			}
			take := maxRunes - currentLen
			if take > len(lineRunes) {
				take = len(lineRunes)
			}
			current.WriteString(string(lineRunes[:take]))
			lineRunes = lineRunes[take:]
			if len(lineRunes) > 0 {
				flush()
			}
		}
		current.WriteByte('\n')
	}
	flush()
	if !sawHeading && len(sections) > 0 {
		sections[0].Heading = "Introduction"
	}
	if len(sections) == 0 {
		sections = append(sections, TextSection{Heading: "Introduction", Text: text})
	}
	return sections
}

// LooksLikeHeading heuristic identifies if a text line resembles a section header based on markdown prefix, numbering, or length.
func LooksLikeHeading(line string) bool {
	if line == "" || utf8.RuneCountInString(line) > 120 {
		return false
	}
	if strings.HasPrefix(line, "#") {
		return true
	}
	if NumberedHeadingPattern.MatchString(line) {
		return true
	}
	return strings.HasSuffix(line, ":") && len(strings.Fields(line)) <= 8
}

// NormalizeHeading trims markdown formatting and punctuation from a heading string.
func NormalizeHeading(line string) string {
	return strings.Trim(strings.TrimSpace(line), "#: ")
}

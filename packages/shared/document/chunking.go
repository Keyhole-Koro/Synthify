package document

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/synthify/backend/packages/shared/domain"
)

var NumberedHeadingPattern = regexp.MustCompile(`^\s*(\d+(?:\.\d+)*)[\.\):]?\s+`)

type TextSection struct {
	Heading string
	Text    string
}

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

func SplitSections(text string) []TextSection {
	const maxRunes = 3500
	var sections []TextSection
	var currentHeading string
	var current strings.Builder

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
		}
		if currentHeading == "" && LooksLikeHeading(trimmed) {
			currentHeading = NormalizeHeading(trimmed)
		}
		if utf8.RuneCountInString(current.String())+utf8.RuneCountInString(line) > maxRunes {
			flush()
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	flush()
	if len(sections) == 0 {
		sections = append(sections, TextSection{Heading: "Introduction", Text: text})
	}
	return sections
}

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

func NormalizeHeading(line string) string {
	return strings.Trim(strings.TrimSpace(line), "#: ")
}

package io

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	shareddocument "github.com/synthify/backend/apps/worker/pkg/worker/document"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

// Dependency expresses an ordering constraint between two tasks or document sections.
//
// TaskID must identify the section that cannot be processed independently.
// DependsOn identifies the parent or prerequisite section that should be
// processed first, and Reason records why the dependency was inferred.
//
// @property TaskID - Generated task identifier for the dependent section.
// @property DependsOn - Generated task identifier for the prerequisite section.
// @property Reason - Explanation for the inferred dependency.
type Dependency struct {
	// TaskID is the generated task identifier for the dependent section.
	TaskID string `json:"task_id"`
	// DependsOn is the generated task identifier for the prerequisite section.
	DependsOn string `json:"depends_on"`
	// Reason explains whether the dependency came from outline nesting or semantic ordering.
	Reason string `json:"reason"`
}

// AnalysisArgs contains the outline section titles to analyze.
//
// The slice may contain Markdown-style headings, numbered headings, or plain
// titles. Empty titles are replaced with deterministic section names so that
// every input position can participate in dependency inference.
//
// @property Outline - Section titles in source-document order.
type AnalysisArgs struct {
	// Outline contains section titles in their source-document order.
	Outline []string `json:"outline"`
}

// AnalysisResult contains derived dependencies and processing priorities for sections.
//
// Dependencies describe the directed edges inferred from hierarchy and title
// semantics. Priorities is a stable processing order; when the inferred graph
// cannot be topologically ordered, it falls back to the original outline order.
//
// @property Dependencies - Parent and semantic prerequisite relationships.
// @property Priorities - Section titles in dependency-aware processing order.
type AnalysisResult struct {
	// Dependencies contains parent and semantic prerequisite relationships between sections.
	Dependencies []Dependency `json:"dependencies"`
	// Priorities contains section titles ordered for dependency-aware processing.
	Priorities []string `json:"priorities"`
}

// section represents an outline heading item with nesting level and position.
type section struct {
	// ID is the deterministic task identifier assigned from the outline position.
	ID string
	// Title is the trimmed source title, or a generated fallback for blank input.
	Title string
	// Level is the inferred heading depth; level 1 represents a top-level section.
	Level int
	// Index is the original zero-based position used for stable ordering.
	Index int
}

// NewAnalysisTool builds the analyze_dependencies tool for determining section ordering constraints.
//
// The returned core.Tool accepts AnalysisArgs encoded as JSON and returns an
// AnalysisResult containing inferred dependencies and a dependency-aware
// processing order. Input decoding and output encoding errors are returned by
// the tool's Run function rather than being converted into a partial result.
//
// Returns:
//   - a configured analysis tool when its input/output schema can be generated;
//   - an error when schema generation fails.
//
// @return A configured analyze_dependencies tool and an optional schema-generation error.
func NewAnalysisTool() (core.Tool, error) {
	schema, err := core.IOSchemaFor[AnalysisArgs, AnalysisResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args AnalysisArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		sections := normalizeOutline(args.Outline)
		dependencies := inferDependencies(sections)
		priorities := sortByPriority(sections, dependencies)
		out, err := json.Marshal(AnalysisResult{
			Dependencies: dependencies,
			Priorities:   priorities,
		})
		return out, core.Usage{}, err
	}
	return core.Tool{
		Name:        "analyze_dependencies",
		Description: "Analyzes document structure to identify logical dependencies between sections. Helps determine the optimal processing order.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

// normalizeOutline converts raw outline titles into deterministic section records.
// It preserves source order, trims surrounding whitespace, derives heading
// levels, and assigns generated names to blank titles.
//
// @param outline - Raw section titles in source-document order.
// @return Normalized section records with deterministic IDs and heading levels.
func normalizeOutline(outline []string) []section {
	sections := make([]section, 0, len(outline))
	for i, raw := range outline {
		title := strings.TrimSpace(raw)
		if title == "" {
			title = fmt.Sprintf("section_%d", i+1)
		}
		sections = append(sections, section{
			ID:    fmt.Sprintf("task_%d", i+1),
			Title: title,
			Level: detectHeadingLevel(title),
			Index: i,
		})
	}
	return sections
}

// detectHeadingLevel infers a section depth from a Markdown or numbered heading.
// Plain titles and malformed/empty heading markers are treated as level 1.
//
// @param title - Raw heading or section title.
// @return Inferred one-based heading depth.
func detectHeadingLevel(title string) int {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return 1
	}
	if strings.HasPrefix(trimmed, "#") {
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level > 0 {
			return level
		}
	}
	if matches := shareddocument.NumberedHeadingPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return strings.Count(matches[1], ".") + 1
	}
	return 1
}

// inferDependencies derives direct parent and semantic prerequisite edges for an outline.
// Parent edges are added for nested headings; titles such as "usage" or
// "implementation" additionally receive an edge to the nearest foundational
// section or, when none exists, the immediately preceding section.
//
// @param sections - Normalized outline sections to analyze.
// @return Inferred dependency edges in deterministic traversal order.
func inferDependencies(sections []section) []Dependency {
	var dependencies []Dependency
	for i, current := range sections {
		if parent := findParentSection(sections, i); parent != nil {
			dependencies = append(dependencies, Dependency{
				TaskID:    current.ID,
				DependsOn: parent.ID,
				Reason:    "subsection depends on its parent section",
			})
		}
		if prerequisite := findSemanticPrerequisite(sections, i); prerequisite != nil && !hasDependency(dependencies, current.ID, prerequisite.ID) {
			dependencies = append(dependencies, Dependency{
				TaskID:    current.ID,
				DependsOn: prerequisite.ID,
				Reason:    "section appears to rely on earlier background or definitions",
			})
		}
	}
	return dependencies
}

// findParentSection returns the nearest preceding section at a shallower heading level.
// It returns nil when the section is top-level or no valid parent precedes it.
//
// @param sections - Normalized outline sections.
// @param index - Zero-based section index whose parent should be found.
// @return Nearest shallower preceding section, or nil when no parent exists.
func findParentSection(sections []section, index int) *section {
	current := sections[index]
	for i := index - 1; i >= 0; i-- {
		if sections[i].Level < current.Level {
			return &sections[i]
		}
	}
	return nil
}

// findSemanticPrerequisite selects a likely conceptual prerequisite for a section.
// Foundational sections and sections without prerequisite-oriented titles do
// not receive a semantic edge. When a prerequisite is needed, the nearest
// foundational section wins, with the original position breaking ties; if no
// foundational section exists, the previous section is used.
//
// @param sections - Normalized outline sections.
// @param index - Zero-based section index requiring a prerequisite.
// @return Selected prerequisite section, or nil when no semantic edge applies.
func findSemanticPrerequisite(sections []section, index int) *section {
	current := sections[index]
	title := strings.ToLower(current.Title)
	if isFoundational(title) || !needsPrerequisite(title) {
		return nil
	}
	bestIndex := -1
	bestDistance := len(sections) + 1
	for i := range sections {
		if i == index {
			continue
		}
		if !isFoundational(strings.ToLower(sections[i].Title)) {
			continue
		}
		distance := index - i
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance || (distance == bestDistance && i < bestIndex) {
			bestDistance = distance
			bestIndex = i
		}
	}
	if bestIndex >= 0 {
		return &sections[bestIndex]
	}
	if index > 0 {
		return &sections[index-1]
	}
	return nil
}

// isFoundational reports whether a title suggests background material needed by later sections.
//
// @param title - Lowercase or otherwise normalized section title.
// @return True when the title contains a foundational keyword.
func isFoundational(title string) bool {
	keywords := []string{"overview", "introduction", "background", "definition", "definitions",
		"concept", "concepts", "terminology", "architecture", "context"}
	return containsAny(title, keywords)
}

// needsPrerequisite reports whether a title suggests an applied section that benefits from context.
//
// @param title - Lowercase or otherwise normalized section title.
// @return True when the title contains a prerequisite-oriented keyword.
func needsPrerequisite(title string) bool {
	keywords := []string{"usage", "workflow", "example", "examples", "implementation", "deployment",
		"configuration", "operation", "operations", "integration", "advanced",
		"troubleshooting", "how to", "guide"}
	return containsAny(title, keywords)
}

// containsAny reports whether title contains at least one keyword as a case-sensitive substring.
// Callers normalize titles before invoking this helper when case-insensitive
// matching is required.
//
// @param title - Text to inspect.
// @param keywords - Substrings that should be searched for.
// @return True when any keyword occurs in title.
func containsAny(title string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(title, keyword) {
			return true
		}
	}
	return false
}

// hasDependency reports whether the exact directed edge already exists.
//
// @param dependencies - Existing directed dependency edges.
// @param taskID - Dependent task identifier to match.
// @param dependsOn - Prerequisite task identifier to match.
// @return True when the exact task/prerequisite pair is already present.
func hasDependency(dependencies []Dependency, taskID, dependsOn string) bool {
	for _, dep := range dependencies {
		if dep.TaskID == taskID && dep.DependsOn == dependsOn {
			return true
		}
	}
	return false
}

// sortByPriority performs a stable topological sort of sections and dependencies.
// Sections with equal readiness retain their original outline order. If a
// cycle prevents all sections from being emitted, the function returns the
// original title order instead of returning a partial schedule.
//
// @param sections - Normalized outline sections to schedule.
// @param dependencies - Directed prerequisite edges between sections.
// @return Stable dependency-aware title order, or source order when cyclic.
func sortByPriority(sections []section, dependencies []Dependency) []string {
	indegree := make(map[string]int, len(sections))
	graph := make(map[string][]string, len(sections))
	byID := make(map[string]section, len(sections))
	for _, s := range sections {
		indegree[s.ID] = 0
		byID[s.ID] = s
	}
	for _, dep := range dependencies {
		graph[dep.DependsOn] = append(graph[dep.DependsOn], dep.TaskID)
		indegree[dep.TaskID]++
	}

	var ready []section
	for _, s := range sections {
		if indegree[s.ID] == 0 {
			ready = append(ready, s)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].Index < ready[j].Index })

	priorities := make([]string, 0, len(sections))
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		priorities = append(priorities, current.Title)
		for _, nextID := range graph[current.ID] {
			indegree[nextID]--
			if indegree[nextID] == 0 {
				ready = append(ready, byID[nextID])
			}
		}
		sort.Slice(ready, func(i, j int) bool { return ready[i].Index < ready[j].Index })
	}

	if len(priorities) != len(sections) {
		priorities = priorities[:0]
		for _, s := range sections {
			priorities = append(priorities, s.Title)
		}
	}
	return priorities
}

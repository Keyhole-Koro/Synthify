package io

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// GrepArgs specifies input parameters for executing a grep search across document files.
//
// @property Pattern - Literal or regular-expression pattern passed to grep.
// @property ContextLines - Number of context lines before and after a match.
// @property IgnoreCase - Whether matching ignores letter case.
// @property ExtendedRegex - Whether grep uses extended regular expressions.
// @property DocumentID - Document scope, optionally inherited from the job.
// @property WorkspaceID - Workspace scope, optionally inherited from the job.
type GrepArgs struct {
	// Pattern is the literal or regular-expression pattern passed to grep.
	Pattern string `json:"pattern"`
	// ContextLines is the number of lines included before and after each match; a non-positive value defaults to 5.
	ContextLines int `json:"context_lines,omitempty"`
	// IgnoreCase requests case-insensitive matching.
	IgnoreCase bool `json:"ignore_case,omitempty"`
	// ExtendedRegex requests grep's extended regular-expression mode.
	ExtendedRegex bool `json:"extended_regex,omitempty"`
	// DocumentID scopes the search to a document; the job context is used when empty.
	DocumentID string `json:"document_id,omitempty"`
	// WorkspaceID scopes filesystem lookup; the job context is used when empty.
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// GrepMatch represents a single matching line along with contextual lines before and after.
//
// @property FileID - Repository file identifier when path resolution succeeds.
// @property FilePath - Path reported by grep.
// @property LineNumber - One-based matching line number.
// @property Line - Matching line content.
// @property Before - Preceding context lines.
// @property After - Following context lines.
type GrepMatch struct {
	// FileID is the repository file identifier, when the file path resolves to a document file.
	FileID string `json:"file_id"`
	// FilePath is the path reported by grep relative to the extracted workspace tree.
	FilePath string `json:"file_path"`
	// LineNumber is the one-based matching line number.
	LineNumber int `json:"line_number"`
	// Line is the matching line content without the grep separator.
	Line string `json:"line"`
	// Before contains up to ContextLines preceding context lines.
	Before []string `json:"before,omitempty"`
	// After contains up to ContextLines following context lines.
	After []string `json:"after,omitempty"`
}

// GrepResult contains all matches discovered during a grep search operation.
//
// @property Matches - Matching lines in grep output order.
type GrepResult struct {
	// Matches contains matching lines in the order emitted by grep.
	Matches []GrepMatch `json:"matches"`
}

// NewGrepTool creates a core.Tool for searching document contents with regex or substring matching.
//
// The tool searches the extracted working tree, uses the filesystem cache when
// available, and best-effort resolves each match to its repository FileID.
//
// Parameters:
//   - b: worker dependencies used for filesystem access and file-ID resolution.
//
// Returns:
//   - a configured grep_search tool;
//   - an error when the input/output schema cannot be generated.
//
// @param b - Worker dependencies used for filesystem access and file-ID resolution.
// @return Configured grep_search tool and optional schema-generation error.
func NewGrepTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[GrepArgs, GrepResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args GrepArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		result, err := grepSearch(ctx, b, args)
		if err != nil {
			return nil, core.Usage{}, err
		}
		out, err := json.Marshal(result)
		return out, core.Usage{}, err
	}
	return core.Tool{
		Name:        "grep_search",
		Description: "Searches for a pattern in the document using grep. Returns matching lines and their context.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

// grepSearch performs the grep operation against cached or extracted files on disk.
func grepSearch(ctx context.Context, b *base.Context, args GrepArgs) (GrepResult, error) {
	docID := args.DocumentID
	if docID == "" && b != nil && b.Job != nil {
		docID = b.Job.DocumentID
	}
	wsID := args.WorkspaceID
	if wsID == "" && b != nil && b.Job != nil {
		wsID = b.Job.WorkspaceID
	}

	if docID == "" || wsID == "" {
		return GrepResult{}, fmt.Errorf("document_id and workspace_id are required")
	}

	contextLines := args.ContextLines
	if contextLines <= 0 {
		contextLines = 5
	}

	// 1. Check Cache
	cacheKeyParts := fmt.Sprintf("%s|%d|%v|%v", args.Pattern, contextLines, args.IgnoreCase, args.ExtendedRegex)
	cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(cacheKeyParts)))
	var cached GrepResult
	if b.FS != nil {
		found, err := b.FS.ReadCache(docID, "grep", cacheKey, &cached)
		if err == nil && found {
			return cached, nil
		}
	}

	// 2. Execute Grep
	if b.FS == nil || b.FS.MountPath == "" {
		return GrepResult{}, fmt.Errorf("FUSE mount is not available for grep_search")
	}

	// Search the extracted working tree, not the document root (which also holds
	// the read-only source/ original).
	targetPath := b.FS.ExtractedDir(wsID, docID)
	// We use -r (recursive) to support zip-extracted directories, and -n (line number)
	// -H (always print filename) for consistent parsing.
	// -B and -A for context.
	cmdArgs := []string{"-r", "-n", "-H"}
	if args.IgnoreCase {
		cmdArgs = append(cmdArgs, "-i")
	}
	if args.ExtendedRegex {
		cmdArgs = append(cmdArgs, "-E")
	}
	cmdArgs = append(cmdArgs,
		"-B", strconv.Itoa(contextLines),
		"-A", strconv.Itoa(contextLines),
		args.Pattern,
		targetPath,
	)

	out, err := exec.CommandContext(ctx, "grep", cmdArgs...).CombinedOutput()
	// grep returns exit code 1 if no matches found, which is not an "error" for us
	if err != nil && !strings.Contains(err.Error(), "exit status 1") {
		return GrepResult{}, fmt.Errorf("grep command failed: %w (output: %s)", err, string(out))
	}

	// 3. Parse Output
	result := parseGrepOutput(string(out), targetPath)

	// Resolve FileID for each match from the repository
	if b != nil && b.Repo != nil {
		for i := range result.Matches {
			file, err := b.Repo.GetDocumentFileByPath(ctx, docID, result.Matches[i].FilePath)
			if err == nil && file != nil {
				result.Matches[i].FileID = file.FileID
			}
		}
	}

	// 4. Save Cache
	if b.FS != nil {
		_ = b.FS.WriteCache(docID, "grep", cacheKey, result)
	}

	return result, nil
}

// parseGrepOutput parses standard grep output into a structured GrepResult with matches and context lines.
func parseGrepOutput(output, targetPath string) GrepResult {
	if output == "" {
		return GrepResult{Matches: []GrepMatch{}}
	}

	var matches []GrepMatch
	lines := strings.Split(output, "\n")

	var currentMatch *GrepMatch

	for _, line := range lines {
		if line == "" || line == "--" {
			continue
		}

		// Robust parsing: grep output format is <path><sep><line><sep><content>
		// where <sep> is ':' for matches and '-' for context.

		var separator string
		isContext := false

		// Find the separator after the file path.
		// If targetPath is a file, the line starts with targetPath.
		// If targetPath is a directory, the line starts with targetPath + "/".

		idx := -1
		if strings.HasPrefix(line, targetPath+":") {
			idx = len(targetPath)
			separator = ":"
			isContext = false
		} else if strings.HasPrefix(line, targetPath+"-") {
			idx = len(targetPath)
			separator = "-"
			isContext = true
		} else if strings.HasPrefix(line, targetPath+"/") {
			// Find the first ':' or '-' after targetPath + "/"
			cIdx := strings.Index(line[len(targetPath)+1:], ":")
			hIdx := strings.Index(line[len(targetPath)+1:], "-")

			if cIdx != -1 && (hIdx == -1 || cIdx < hIdx) {
				idx = len(targetPath) + 1 + cIdx
				separator = ":"
				isContext = false
			} else if hIdx != -1 {
				idx = len(targetPath) + 1 + hIdx
				separator = "-"
				isContext = true
			}
		}

		if idx == -1 {
			continue
		}

		filePath := line[:idx]
		remainder := line[idx+1:]

		// Remainder should be <line><sep><content>
		parts := strings.SplitN(remainder, separator, 2)
		if len(parts) < 2 {
			continue
		}

		lineNum, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		content := parts[1]

		// Clean up relative path if it's inside targetPath
		displayPath := strings.TrimPrefix(filePath, targetPath)
		displayPath = strings.TrimPrefix(displayPath, "/")

		if !isContext {
			// This is a primary match line
			match := GrepMatch{
				FilePath:   displayPath,
				LineNumber: lineNum,
				Line:       content,
			}
			matches = append(matches, match)
			currentMatch = &matches[len(matches)-1]
		} else if currentMatch != nil {
			// This is a context line for the last seen match
			if lineNum < currentMatch.LineNumber {
				currentMatch.Before = append(currentMatch.Before, content)
			} else {
				currentMatch.After = append(currentMatch.After, content)
			}
		}
	}

	return GrepResult{Matches: matches}
}

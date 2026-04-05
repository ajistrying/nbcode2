package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxListResults = 100

type ListFilesTool struct{}

type listFilesArgs struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern,omitempty"`
}

func (t *ListFilesTool) Name() string        { return "list_files" }
func (t *ListFilesTool) RiskTier() RiskTier   { return RiskSafe }
func (t *ListFilesTool) Description() string {
	return "List files in a directory. Optionally filter by glob pattern (e.g. \"**/*.go\"). Returns up to 100 results."
}

func (t *ListFilesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Directory path to list"
			},
			"pattern": {
				"type": "string",
				"description": "Optional glob pattern to filter files (e.g. \"*.go\", \"**/*.ts\")"
			}
		},
		"required": ["path"]
	}`)
}

func (t *ListFilesTool) Execute(args json.RawMessage) (string, error) {
	var a listFilesArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	info, err := os.Stat(a.Path)
	if err != nil {
		return "", fmt.Errorf("accessing path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", a.Path)
	}

	var results []string

	if a.Pattern != "" {
		// Glob pattern matching
		fullPattern := filepath.Join(a.Path, a.Pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return "", fmt.Errorf("invalid glob pattern: %w", err)
		}
		results = matches
	} else {
		// Simple directory listing
		entries, err := os.ReadDir(a.Path)
		if err != nil {
			return "", fmt.Errorf("reading directory: %w", err)
		}
		for _, e := range entries {
			prefix := "  "
			if e.IsDir() {
				prefix = "📁"
			}
			results = append(results, fmt.Sprintf("%s %s", prefix, e.Name()))
		}
	}

	if len(results) > maxListResults {
		truncated := results[:maxListResults]
		return fmt.Sprintf("%s\n\n[Truncated: showing %d of %d results]",
			strings.Join(truncated, "\n"), maxListResults, len(results)), nil
	}

	if len(results) == 0 {
		return "No files found.", nil
	}

	return strings.Join(results, "\n"), nil
}

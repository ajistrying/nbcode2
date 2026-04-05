package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const maxSearchResults = 100

type SearchTool struct{}

type searchArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Glob    string `json:"glob,omitempty"`
}

func (t *SearchTool) Name() string        { return "search" }
func (t *SearchTool) RiskTier() RiskTier   { return RiskSafe }
func (t *SearchTool) Description() string {
	return "Search file contents using ripgrep (rg). Supports regex patterns. Returns matching lines with file paths and line numbers."
}

func (t *SearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Regex pattern to search for"
			},
			"path": {
				"type": "string",
				"description": "Directory or file to search in"
			},
			"glob": {
				"type": "string",
				"description": "Optional glob to filter files (e.g. \"*.go\", \"*.ts\")"
			}
		},
		"required": ["pattern", "path"]
	}`)
}

func (t *SearchTool) Execute(args json.RawMessage) (string, error) {
	var a searchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	cmdArgs := []string{"-n", "--no-heading", "--color", "never"}

	if a.Glob != "" {
		cmdArgs = append(cmdArgs, "--glob", a.Glob)
	}

	cmdArgs = append(cmdArgs, a.Pattern, a.Path)

	cmd := exec.Command("rg", cmdArgs...)
	output, err := cmd.Output()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return "No matches found.", nil
			}
			return "", fmt.Errorf("search failed: %s", string(exitErr.Stderr))
		}
		// Check if rg is not installed, fall back to grep
		if _, lookErr := exec.LookPath("rg"); lookErr != nil {
			return t.fallbackGrep(a)
		}
		return "", fmt.Errorf("search error: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > maxSearchResults {
		truncated := lines[:maxSearchResults]
		return fmt.Sprintf("%s\n\n[Truncated: showing %d of %d matches]",
			strings.Join(truncated, "\n"), maxSearchResults, len(lines)), nil
	}

	return strings.Join(lines, "\n"), nil
}

func (t *SearchTool) fallbackGrep(a searchArgs) (string, error) {
	cmdArgs := []string{"-rn", "--color=never"}
	if a.Glob != "" {
		cmdArgs = append(cmdArgs, "--include", a.Glob)
	}
	cmdArgs = append(cmdArgs, a.Pattern, a.Path)

	cmd := exec.Command("grep", cmdArgs...)
	output, err := cmd.Output()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "No matches found.", nil
		}
		return "", fmt.Errorf("grep fallback failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > maxSearchResults {
		truncated := lines[:maxSearchResults]
		return fmt.Sprintf("%s\n\n[Truncated: showing %d of %d matches]",
			strings.Join(truncated, "\n"), maxSearchResults, len(lines)), nil
	}

	return strings.Join(lines, "\n"), nil
}

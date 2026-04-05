package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const maxFileLines = 2000

type ReadFileTool struct{}

type readFileArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) RiskTier() RiskTier   { return RiskSafe }
func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Returns numbered lines. Use offset and limit for large files."
}

func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Absolute path to the file to read"
			},
			"offset": {
				"type": "integer",
				"description": "Line number to start reading from (0-based). Default: 0"
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of lines to read. Default: 2000"
			}
		},
		"required": ["path"]
	}`)
}

func (t *ReadFileTool) Execute(args json.RawMessage) (string, error) {
	var a readFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	total := len(lines)

	offset := a.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return fmt.Sprintf("File has %d lines, offset %d is past the end.", total, offset), nil
	}

	limit := a.Limit
	if limit <= 0 {
		limit = maxFileLines
	}

	end := offset + limit
	if end > total {
		end = total
	}

	var sb strings.Builder
	for i := offset; i < end; i++ {
		fmt.Fprintf(&sb, "%d\t%s\n", i+1, lines[i])
	}

	if end < total {
		fmt.Fprintf(&sb, "\n[Truncated: showing lines %d-%d of %d. Use offset/limit for more.]", offset+1, end, total)
	}

	return sb.String(), nil
}

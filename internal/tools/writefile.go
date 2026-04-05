package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type WriteFileTool struct{}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) RiskTier() RiskTier   { return RiskMedium }
func (t *WriteFileTool) Description() string {
	return "Write content to a file. Creates the file and parent directories if they don't exist. Overwrites existing content."
}

func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Absolute path to the file to write"
			},
			"content": {
				"type": "string",
				"description": "The content to write to the file"
			}
		},
		"required": ["path", "content"]
	}`)
}

func (t *WriteFileTool) Execute(args json.RawMessage) (string, error) {
	var a writeFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	dir := filepath.Dir(a.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating directories: %w", err)
	}

	if err := os.WriteFile(a.Path, []byte(a.Content), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(a.Content), a.Path), nil
}

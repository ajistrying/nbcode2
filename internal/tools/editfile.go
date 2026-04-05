package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type EditFileTool struct{}

type editFileArgs struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) RiskTier() RiskTier   { return RiskMedium }
func (t *EditFileTool) Description() string {
	return "Edit a file by replacing an exact string match. The old_string must appear exactly once in the file. Use this for targeted edits instead of rewriting entire files."
}

func (t *EditFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Absolute path to the file to edit"
			},
			"old_string": {
				"type": "string",
				"description": "The exact string to find and replace. Must be unique in the file."
			},
			"new_string": {
				"type": "string",
				"description": "The string to replace old_string with"
			}
		},
		"required": ["path", "old_string", "new_string"]
	}`)
}

func (t *EditFileTool) Execute(args json.RawMessage) (string, error) {
	var a editFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	content := string(data)
	count := strings.Count(content, a.OldString)

	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", a.Path)
	}
	if count > 1 {
		return "", fmt.Errorf("old_string appears %d times in %s — must be unique. Provide more surrounding context to make it unique", count, a.Path)
	}

	newContent := strings.Replace(content, a.OldString, a.NewString, 1)
	if err := os.WriteFile(a.Path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s", a.Path), nil
}

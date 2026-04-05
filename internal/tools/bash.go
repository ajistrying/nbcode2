package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	maxBashOutput  = 30000
	bashTimeout    = 2 * time.Minute
	truncKeepChars = 15000
)

type BashTool struct{}

type bashArgs struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // milliseconds
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) RiskTier() RiskTier   { return RiskHigh }
func (t *BashTool) Description() string {
	return "Execute a shell command and return its output. Commands run in the user's default shell. Long-running commands will timeout after 2 minutes by default."
}

func (t *BashTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute"
			},
			"timeout": {
				"type": "integer",
				"description": "Optional timeout in milliseconds (max 600000). Default: 120000 (2 minutes)"
			}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) Execute(args json.RawMessage) (string, error) {
	var a bashArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	timeout := bashTimeout
	if a.Timeout > 0 {
		ms := a.Timeout
		if ms > 600000 {
			ms = 600000
		}
		timeout = time.Duration(ms) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", a.Command)
	output, err := cmd.CombinedOutput()

	result := string(output)

	// Truncate if output is too large
	if len(result) > maxBashOutput {
		head := result[:truncKeepChars]
		tail := result[len(result)-truncKeepChars:]
		truncatedLines := strings.Count(result[truncKeepChars:len(result)-truncKeepChars], "\n")
		result = fmt.Sprintf("%s\n\n[%d lines truncated]\n\n%s", head, truncatedLines, tail)
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("command timed out after %s", timeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return result, fmt.Errorf("exit code %d", exitErr.ExitCode())
		}
		return result, fmt.Errorf("execution error: %w", err)
	}

	return result, nil
}

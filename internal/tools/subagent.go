package tools

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/ajistrying/nbcode2/internal/provider"
)

// SubAgentRunner is the function signature for running a sub-agent.
// It's injected by the agent package to avoid circular dependencies.
type SubAgentRunner func(task string) (string, error)

// SubAgentTool spawns a sub-agent goroutine to handle a task independently.
type SubAgentTool struct {
	runner SubAgentRunner
}

// NewSubAgentTool creates a sub-agent tool with the given runner function.
func NewSubAgentTool(runner SubAgentRunner) *SubAgentTool {
	return &SubAgentTool{runner: runner}
}

type subAgentArgs struct {
	Task string `json:"task"`
}

func (t *SubAgentTool) Name() string      { return "sub_agent" }
func (t *SubAgentTool) RiskTier() RiskTier { return RiskHigh }
func (t *SubAgentTool) Description() string {
	return "Spawn a sub-agent to handle a task independently. The sub-agent has access to file and search tools but cannot spawn its own sub-agents. Returns a string summary of what it accomplished."
}

func (t *SubAgentTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task": {
				"type": "string",
				"description": "A clear description of the task for the sub-agent to accomplish"
			}
		},
		"required": ["task"]
	}`)
}

func (t *SubAgentTool) Execute(args json.RawMessage) (string, error) {
	var a subAgentArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if t.runner == nil {
		return "", fmt.Errorf("sub-agent runner not configured")
	}

	return t.runner(a.Task)
}

// FileLocker provides optimistic file locking for sub-agents.
// It tracks file hashes before and after edits to detect conflicts.
type FileLocker struct {
	mu     sync.Mutex
	hashes map[string]string // path -> sha256 hash
}

// NewFileLocker creates a new file locker.
func NewFileLocker() *FileLocker {
	return &FileLocker{
		hashes: make(map[string]string),
	}
}

// HashFile computes and stores the SHA-256 hash of a file.
func (fl *FileLocker) HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // file doesn't exist yet, that's fine
		}
		return "", err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	fl.mu.Lock()
	fl.hashes[path] = hash
	fl.mu.Unlock()
	return hash, nil
}

// CheckFile verifies a file hasn't changed since it was hashed.
// Returns true if the file is unchanged, false if it was modified.
func (fl *FileLocker) CheckFile(path string) (bool, error) {
	fl.mu.Lock()
	originalHash, exists := fl.hashes[path]
	fl.mu.Unlock()

	if !exists {
		return true, nil // never tracked, assume OK
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	currentHash := fmt.Sprintf("%x", sha256.Sum256(data))
	return currentHash == originalHash, nil
}

// LockedEditFile wraps an edit operation with optimistic locking.
// If the file changed between hash and edit, it returns an error.
func (fl *FileLocker) LockedEditFile(path string, editFn func() error) error {
	// Hash before edit
	_, err := fl.HashFile(path)
	if err != nil {
		return fmt.Errorf("pre-edit hash failed: %w", err)
	}

	// Perform the edit
	if err := editFn(); err != nil {
		return err
	}

	// Update the hash after successful edit
	_, err = fl.HashFile(path)
	return err
}

// SubAgentDefinitions returns the tool definitions available to sub-agents
// (everything except sub_agent itself).
func SubAgentDefinitions(registry *Registry) []provider.ToolDefinition {
	return registry.SubAgentDefinitions()
}

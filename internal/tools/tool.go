package tools

import (
	"encoding/json"
	"fmt"

	"github.com/ajistrying/nbcode2/internal/provider"
)

// RiskTier classifies how dangerous a tool is.
type RiskTier int

const (
	RiskSafe   RiskTier = iota // auto-approve (reads)
	RiskMedium                 // confirm by default (writes)
	RiskHigh                   // always confirm by default (execute)
)

// Tool is the interface every tool must implement.
type Tool interface {
	// Name returns the tool's identifier (e.g. "read_file").
	Name() string

	// Description returns a human-readable description for the LLM.
	Description() string

	// Parameters returns the JSON Schema for the tool's parameters.
	Parameters() json.RawMessage

	// Execute runs the tool with the given arguments and returns the result.
	Execute(args json.RawMessage) (string, error)

	// RiskTier returns the tool's risk classification.
	RiskTier() RiskTier
}

// Registry holds all registered tools and provides lookup.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a registry with all built-in tools.
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
	}

	// Register all day-one tools
	builtins := []Tool{
		&ReadFileTool{},
		&WriteFileTool{},
		&EditFileTool{},
		&ListFilesTool{},
		&SearchTool{},
		&BashTool{},
		&WebSearchTool{},
	}

	for _, t := range builtins {
		r.tools[t.Name()] = t
	}

	return r
}

// Register adds a tool to the registry. Used for tools that need
// runtime dependencies (e.g. SubAgent needs a runner function).
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get returns a tool by name, or an error if not found.
func (r *Registry) Get(name string) (Tool, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return t, nil
}

// Definitions returns all tool definitions for the LLM.
func (r *Registry) Definitions() []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

// SubAgentDefinitions returns tool definitions excluding the sub-agent tool.
func (r *Registry) SubAgentDefinitions() []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		if t.Name() == "sub_agent" {
			continue
		}
		defs = append(defs, provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

package agent

import (
	"encoding/json"
	"fmt"
	"log"

	agentctx "github.com/ajistrying/nbcode2/internal/context"
	"github.com/ajistrying/nbcode2/internal/permission"
	"github.com/ajistrying/nbcode2/internal/provider"
	"github.com/ajistrying/nbcode2/internal/tools"
)

// ConfirmFunc is called when a tool needs user approval.
// Returns true if the user approves, false to deny.
type ConfirmFunc func(toolName string, args json.RawMessage) bool

// StatusFunc is called to update the TUI with the current action.
type StatusFunc func(status string)

// ToolCallFunc is called to notify the TUI about a tool invocation.
type ToolCallFunc func(toolName string, args json.RawMessage)

// Agent orchestrates the tool-use loop between the user, LLM, and tools.
type Agent struct {
	provider   provider.Provider
	tools      *tools.Registry
	context    *agentctx.Manager
	permission *permission.Checker
	onConfirm  ConfirmFunc
	onStatus   StatusFunc
	onToolCall ToolCallFunc
	systemMsg  provider.Message
}

// Config holds dependencies for creating an Agent.
type Config struct {
	Provider     provider.Provider
	Tools        *tools.Registry
	Context      *agentctx.Manager
	Permission   *permission.Checker
	SystemPrompt string
	OnConfirm    ConfirmFunc
	OnStatus     StatusFunc
	OnToolCall   ToolCallFunc
}

// New creates a new agent with the given configuration.
// It registers the sub-agent tool with a runner that creates
// a child agent loop (no nesting — sub-agents cannot spawn sub-agents).
func New(cfg Config) *Agent {
	systemMsg := provider.Message{
		Role:    provider.RoleSystem,
		Content: cfg.SystemPrompt,
	}

	ctx := cfg.Context
	ctx.Add(systemMsg)

	a := &Agent{
		provider:   cfg.Provider,
		tools:      cfg.Tools,
		context:    ctx,
		permission: cfg.Permission,
		onConfirm:  cfg.OnConfirm,
		onStatus:   cfg.OnStatus,
		onToolCall: cfg.OnToolCall,
		systemMsg:  systemMsg,
	}

	// Register the sub-agent tool with a runner that creates a child agent loop
	subAgentTool := tools.NewSubAgentTool(a.runSubAgent)
	cfg.Tools.Register(subAgentTool)

	return a
}

// runSubAgent executes a task in a child agent loop.
// The child has a restricted tool set (no sub-agent spawning).
func (a *Agent) runSubAgent(task string) (string, error) {
	a.setStatus("Spawning sub-agent...")

	// Create a fresh context for the sub-agent
	childCtx := agentctx.NewManager(a.provider.MaxContextTokens(), 0.8)

	childCtx.Add(provider.Message{
		Role:    provider.RoleSystem,
		Content: a.systemMsg.Content + "\n\nYou are a sub-agent. Complete the assigned task and provide a concise summary of what you accomplished. You cannot spawn sub-agents.",
	})

	childCtx.Add(provider.Message{
		Role:    provider.RoleUser,
		Content: task,
	})

	// Sub-agent tool definitions exclude sub_agent itself
	childToolDefs := a.tools.SubAgentDefinitions()

	// Run the child agent loop (max 20 iterations to prevent runaway)
	for i := 0; i < 20; i++ {
		a.setStatus(fmt.Sprintf("Sub-agent thinking (step %d)...", i+1))

		resp, err := a.provider.Chat(childCtx.Messages(), childToolDefs)
		if err != nil {
			return "", fmt.Errorf("sub-agent provider error: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		childCtx.Add(provider.Message{
			Role:      provider.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			// Sub-agents skip sub_agent tool calls as a safety measure
			if tc.Name == "sub_agent" {
				childCtx.Add(provider.Message{
					Role:       provider.RoleTool,
					Content:    "Error: sub-agents cannot spawn other sub-agents.",
					ToolCallID: tc.ID,
					Name:       tc.Name,
				})
				continue
			}

			tool, err := a.tools.Get(tc.Name)
			if err != nil {
				childCtx.Add(provider.Message{
					Role:       provider.RoleTool,
					Content:    fmt.Sprintf("Error: %s", err),
					ToolCallID: tc.ID,
					Name:       tc.Name,
				})
				continue
			}

			// Sub-agents use the same permission model
			decision := a.permission.Check(tool)
			if decision == permission.NeedsApproval {
				if a.onConfirm != nil && !a.onConfirm(tc.Name, tc.Arguments) {
					childCtx.Add(provider.Message{
						Role:       provider.RoleTool,
						Content:    "Tool execution denied by user.",
						ToolCallID: tc.ID,
						Name:       tc.Name,
					})
					continue
				}
			}

			a.setStatus(fmt.Sprintf("Sub-agent: running %s...", tc.Name))

			result, err := tool.Execute(tc.Arguments)
			if err != nil {
				childCtx.Add(provider.Message{
					Role:       provider.RoleTool,
					Content:    fmt.Sprintf("Error: %s", err),
					ToolCallID: tc.ID,
					Name:       tc.Name,
				})
				continue
			}

			childCtx.Add(provider.Message{
				Role:       provider.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
		}
	}

	return "Sub-agent reached maximum iteration limit (20 steps). Partial work may have been completed.", nil
}

// Run processes a user message through the tool-use loop and returns the
// final assistant text response.
func (a *Agent) Run(userMessage string) (string, error) {
	a.context.Add(provider.Message{
		Role:    provider.RoleUser,
		Content: userMessage,
	})

	for {
		a.setStatus("Thinking...")

		resp, err := a.provider.Chat(a.context.Messages(), a.tools.Definitions())
		if err != nil {
			return "", fmt.Errorf("provider error: %w", err)
		}

		a.context.UpdateUsage(resp.Usage)

		// No tool calls — we have a final response
		if len(resp.ToolCalls) == 0 {
			a.context.Add(provider.Message{
				Role:    provider.RoleAssistant,
				Content: resp.Content,
			})
			a.setStatus("")
			return resp.Content, nil
		}

		// Add assistant message with tool calls
		a.context.Add(provider.Message{
			Role:      provider.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			result := a.executeTool(tc)
			a.context.Add(provider.Message{
				Role:       provider.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
		}

		// Check if we should summarize before the next loop iteration
		if a.context.ShouldSummarize() {
			a.setStatus("Summarizing conversation...")
			a.summarize()
		}
	}
}

// executeTool runs a single tool call, handling permissions and errors.
func (a *Agent) executeTool(tc provider.ToolCall) string {
	tool, err := a.tools.Get(tc.Name)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}

	// Check permissions
	decision := a.permission.Check(tool)
	if decision == permission.NeedsApproval {
		if a.onConfirm != nil && !a.onConfirm(tc.Name, tc.Arguments) {
			return "Tool execution denied by user."
		}
	} else if decision == permission.Denied {
		return "Tool execution is not permitted."
	}

	if a.onToolCall != nil {
		a.onToolCall(tc.Name, tc.Arguments)
	}
	a.setStatus(fmt.Sprintf("Running %s...", tc.Name))

	result, err := tool.Execute(tc.Arguments)
	if err != nil {
		return fmt.Sprintf("Error executing %s: %s", tc.Name, err)
	}

	return result
}

// summarize compacts the conversation using a summarization call.
func (a *Agent) summarize() {
	summaryMessages := []provider.Message{
		{
			Role:    provider.RoleSystem,
			Content: "Provide a detailed but concise summary of the following conversation. Focus on: what was done, what is being worked on, which files are involved, and what the next steps are.",
		},
	}

	// Add a condensed view of the conversation
	for _, msg := range a.context.Messages() {
		if msg.Role == provider.RoleSystem {
			continue
		}
		if msg.Content != "" {
			summaryMessages = append(summaryMessages, provider.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	resp, err := a.provider.Chat(summaryMessages, nil)
	if err != nil {
		log.Printf("Summarization failed: %v", err)
		return
	}

	a.context.Compact(resp.Content, a.systemMsg)
}

// TotalTokens returns the current token usage.
func (a *Agent) TotalTokens() int {
	return a.context.TotalTokens()
}

// Messages returns the current conversation for session persistence.
func (a *Agent) Messages() []provider.Message {
	return a.context.Messages()
}

func (a *Agent) setStatus(status string) {
	if a.onStatus != nil {
		a.onStatus(status)
	}
}

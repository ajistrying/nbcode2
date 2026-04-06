package skills

import (
	"encoding/json"
	"fmt"

	agentctx "github.com/ajistrying/nbcode2/internal/context"
	"github.com/ajistrying/nbcode2/internal/permission"
	"github.com/ajistrying/nbcode2/internal/provider"
	"github.com/ajistrying/nbcode2/internal/tools"
)

// ExecutorConfig holds dependencies needed for skill execution.
type ExecutorConfig struct {
	Provider   provider.Provider
	Tools      *tools.Registry
	Permission *permission.Checker
	OnConfirm  func(toolName string, args json.RawMessage) bool
	OnStatus   func(status string)
	OnToolCall func(toolName string, args json.RawMessage)
	SystemMsg  provider.Message
}

// Executor handles running skills (inline or forked).
type Executor struct {
	config ExecutorConfig
}

// NewExecutor creates a skill executor with the given dependencies.
func NewExecutor(cfg ExecutorConfig) *Executor {
	return &Executor{config: cfg}
}

// Execute runs a skill with the given arguments.
// For forked skills: spawns an isolated sub-agent, returns its final response.
// For inline skills: returns the prepared body for injection into conversation.
func (e *Executor) Execute(skill *Skill, args string) (string, error) {
	body, err := PrepareBody(skill, args)
	if err != nil {
		return "", fmt.Errorf("preparing skill body: %w", err)
	}

	if skill.IsForked() {
		return e.executeForked(skill, body)
	}
	return body, nil
}

// executeForked spawns an isolated sub-agent for the skill.
// No iteration cap — skills run until the LLM stops making tool calls.
func (e *Executor) executeForked(skill *Skill, body string) (string, error) {
	e.setStatus(fmt.Sprintf("Running skill '%s'...", skill.Name()))

	// Fresh context for the forked agent
	childCtx := agentctx.NewManager(e.config.Provider.MaxContextTokens(), 0.8)

	systemContent := e.config.SystemMsg.Content +
		"\n\nYou are executing a skill. Follow the instructions precisely. " +
		"Complete all steps and provide a summary of what was accomplished."

	childCtx.Add(provider.Message{
		Role:    provider.RoleSystem,
		Content: systemContent,
	})

	childCtx.Add(provider.Message{
		Role:    provider.RoleUser,
		Content: body,
	})

	// Use full tool definitions (forked agents get same tools)
	// Exclude sub_agent and skill tools to prevent nesting
	toolDefs := e.config.Tools.SkillAgentDefinitions()

	// Run the agent loop — no iteration cap for skills
	for i := 0; ; i++ {
		e.setStatus(fmt.Sprintf("Skill '%s' step %d...", skill.Name(), i+1))

		resp, err := e.config.Provider.Chat(childCtx.Messages(), toolDefs)
		if err != nil {
			return "", fmt.Errorf("skill agent error at step %d: %w", i+1, err)
		}

		childCtx.UpdateUsage(resp.Usage)

		// No tool calls — skill is done
		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		childCtx.Add(provider.Message{
			Role:      provider.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			result := e.executeTool(tc)
			childCtx.Add(provider.Message{
				Role:       provider.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
		}

		// Check if context is getting full, summarize if needed
		if childCtx.ShouldSummarize() {
			e.setStatus(fmt.Sprintf("Skill '%s': summarizing context...", skill.Name()))
			e.summarizeChild(childCtx, systemContent)
		}
	}
}

// executeTool runs a single tool call with permission checking.
func (e *Executor) executeTool(tc provider.ToolCall) string {
	// Block skill and sub_agent tools in forked execution
	if tc.Name == "sub_agent" || tc.Name == "skill" {
		return fmt.Sprintf("Error: %s tool is not available in skill execution.", tc.Name)
	}

	tool, err := e.config.Tools.Get(tc.Name)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}

	decision := e.config.Permission.Check(tool)
	if decision == permission.NeedsApproval {
		if e.config.OnConfirm != nil && !e.config.OnConfirm(tc.Name, tc.Arguments) {
			return "Tool execution denied by user."
		}
	} else if decision == permission.Denied {
		return "Tool execution is not permitted."
	}

	if e.config.OnToolCall != nil {
		e.config.OnToolCall(tc.Name, tc.Arguments)
	}
	e.setStatus(fmt.Sprintf("Running %s...", tc.Name))

	result, err := tool.Execute(tc.Arguments)
	if err != nil {
		return fmt.Sprintf("Error executing %s: %s", tc.Name, err)
	}

	return result
}

// summarizeChild compacts the child context when it gets too large.
func (e *Executor) summarizeChild(ctx *agentctx.Manager, systemContent string) {
	summaryMessages := []provider.Message{
		{
			Role:    provider.RoleSystem,
			Content: "Provide a detailed but concise summary of the skill execution so far. Focus on: what steps were completed, what files were created/modified, and what remains to be done.",
		},
	}

	for _, msg := range ctx.Messages() {
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

	resp, err := e.config.Provider.Chat(summaryMessages, nil)
	if err != nil {
		return
	}

	systemMsg := provider.Message{
		Role:    provider.RoleSystem,
		Content: systemContent,
	}
	ctx.Compact(resp.Content, systemMsg)
}

func (e *Executor) setStatus(status string) {
	if e.config.OnStatus != nil {
		e.config.OnStatus(status)
	}
}

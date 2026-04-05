package provider

import "encoding/json"

// Role represents a message role in the conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a single message in the conversation.
type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolDefinition describes a tool the model can call.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Response is what a provider returns after a chat completion.
type Response struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     Usage      `json:"usage"`
}

// Usage tracks token consumption for a single response.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Provider is the core interface for LLM communication.
// All provider-specific details (API format, auth, retries) are
// encapsulated behind this interface.
type Provider interface {
	// Chat sends a conversation to the model and returns its response.
	Chat(messages []Message, tools []ToolDefinition) (*Response, error)

	// Name returns the provider's display name (e.g. "anthropic", "openai").
	Name() string

	// Model returns the current model identifier.
	Model() string

	// MaxContextTokens returns the model's context window size in tokens.
	MaxContextTokens() int
}

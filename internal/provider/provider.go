package provider

import (
	"encoding/json"
)

// StreamEventType identifies the kind of streaming event.
type StreamEventType string

const (
	EventTextDelta StreamEventType = "text_delta"
	EventToolStart StreamEventType = "tool_start"
	EventToolDelta StreamEventType = "tool_delta"
	EventDone      StreamEventType = "done"
	EventError     StreamEventType = "error"
)

// StreamEvent represents a single event from a streaming chat completion.
type StreamEvent struct {
	Type      StreamEventType // what kind of event this is
	Delta     string          // text content or argument fragment
	ToolCall  *ToolCall       // populated on EventToolStart (has ID + Name)
	ToolCalls []ToolCall      // all completed tool calls, populated on EventDone
	Usage     *Usage          // populated on EventDone
	Error     error           // populated on EventError
}

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

	// ChatStream sends a conversation and returns a channel of streaming events.
	// The channel is closed when the stream is complete.
	// Providers that don't support native streaming should wrap Chat() in a
	// single-shot channel (emit text + done events).
	ChatStream(messages []Message, tools []ToolDefinition) <-chan StreamEvent

	// Name returns the provider's display name (e.g. "anthropic", "openai").
	Name() string

	// Model returns the current model identifier.
	Model() string

	// MaxContextTokens returns the model's context window size in tokens.
	MaxContextTokens() int
}

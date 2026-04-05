package provider

import (
	"encoding/json"
	"testing"
)

// --- OpenAI Message Conversion Tests ---

func TestToOpenAIMessages(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "You are helpful."},
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi!"},
	}

	result := toOpenAIMessages(messages)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("expected 'system' role, got %q", result[0].Role)
	}
	if result[1].Content != "Hello" {
		t.Errorf("expected 'Hello', got %q", result[1].Content)
	}
}

func TestToOpenAIMessagesWithToolCalls(t *testing.T) {
	messages := []Message{
		{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{
				{
					ID:        "call_1",
					Name:      "read_file",
					Arguments: json.RawMessage(`{"path": "/tmp/test.txt"}`),
				},
			},
		},
		{
			Role:       RoleTool,
			Content:    "file contents here",
			ToolCallID: "call_1",
			Name:       "read_file",
		},
	}

	result := toOpenAIMessages(messages)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	// Assistant message should have tool calls
	if len(result[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result[0].ToolCalls))
	}
	if result[0].ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", result[0].ToolCalls[0].Function.Name)
	}

	// Tool result message
	if result[1].ToolCallID != "call_1" {
		t.Errorf("expected tool call ID 'call_1', got %q", result[1].ToolCallID)
	}
}

func TestToOpenAITools(t *testing.T) {
	tools := []ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read a file",
			Parameters:  json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string"}}}`),
		},
	}

	result := toOpenAITools(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Function.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", result[0].Function.Name)
	}
}

// --- Anthropic Message Conversion Tests ---

func TestToAnthropicMessages(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "You are helpful."},
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi!"},
	}

	system, result := toAnthropicMessages(messages)

	if system != "You are helpful." {
		t.Errorf("expected system prompt, got %q", system)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (system extracted), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("expected 'user' role, got %q", result[0].Role)
	}
}

func TestToAnthropicMessagesWithToolUse(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "system"},
		{Role: RoleUser, Content: "read the file"},
		{
			Role: RoleAssistant,
			Content: "I'll read that file.",
			ToolCalls: []ToolCall{
				{
					ID:        "tool_1",
					Name:      "read_file",
					Arguments: json.RawMessage(`{"path": "/tmp/test"}`),
				},
			},
		},
		{
			Role:       RoleTool,
			Content:    "file contents",
			ToolCallID: "tool_1",
		},
	}

	system, result := toAnthropicMessages(messages)
	if system != "system" {
		t.Errorf("expected system, got %q", system)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// Assistant message should have content blocks
	blocks, ok := result[1].Content.([]anthropicContentBlock)
	if !ok {
		t.Fatal("expected assistant content to be []anthropicContentBlock")
	}
	if len(blocks) != 2 { // text + tool_use
		t.Errorf("expected 2 blocks (text + tool_use), got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Errorf("expected text block first, got %q", blocks[0].Type)
	}
	if blocks[1].Type != "tool_use" {
		t.Errorf("expected tool_use block second, got %q", blocks[1].Type)
	}

	// Tool result should be a user message with tool_result block
	toolResult, ok := result[2].Content.([]anthropicContentBlock)
	if !ok {
		t.Fatal("expected tool result to be []anthropicContentBlock")
	}
	if toolResult[0].Type != "tool_result" {
		t.Errorf("expected tool_result type, got %q", toolResult[0].Type)
	}
	if toolResult[0].ToolUseID != "tool_1" {
		t.Errorf("expected tool_use_id 'tool_1', got %q", toolResult[0].ToolUseID)
	}
}

func TestToAnthropicToolResultMerging(t *testing.T) {
	// Multiple tool results in a row should merge into one user message
	messages := []Message{
		{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{
				{ID: "t1", Name: "bash", Arguments: json.RawMessage(`{}`)},
				{ID: "t2", Name: "bash", Arguments: json.RawMessage(`{}`)},
			},
		},
		{Role: RoleTool, Content: "result 1", ToolCallID: "t1"},
		{Role: RoleTool, Content: "result 2", ToolCallID: "t2"},
	}

	_, result := toAnthropicMessages(messages)

	// Should be: assistant, then one user message with two tool_result blocks
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (results merged), got %d", len(result))
	}

	blocks, ok := result[1].Content.([]anthropicContentBlock)
	if !ok {
		t.Fatal("expected merged tool results to be []anthropicContentBlock")
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 merged tool_result blocks, got %d", len(blocks))
	}
}

func TestToAnthropicTools(t *testing.T) {
	tools := []ToolDefinition{
		{
			Name:        "bash",
			Description: "Run a command",
			Parameters:  json.RawMessage(`{"type": "object"}`),
		},
	}

	result := toAnthropicTools(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Name != "bash" {
		t.Errorf("expected 'bash', got %q", result[0].Name)
	}
	if string(result[0].InputSchema) != `{"type": "object"}` {
		t.Errorf("expected input schema, got %q", string(result[0].InputSchema))
	}
}

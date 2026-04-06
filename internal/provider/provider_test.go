package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// --- Anthropic ChatStream Stub Tests ---

func TestAnthropicChatStreamStubTextOnly(t *testing.T) {
	// We can't call a real API, but we can test the stub by creating a
	// provider with a test HTTP server that returns a known response.
	// For now, test the stream event types and structure.
	t.Run("StreamEventTypes", func(t *testing.T) {
		// Verify event type constants are correctly defined
		if EventTextDelta != "text_delta" {
			t.Errorf("expected text_delta, got %q", EventTextDelta)
		}
		if EventToolStart != "tool_start" {
			t.Errorf("expected tool_start, got %q", EventToolStart)
		}
		if EventToolDelta != "tool_delta" {
			t.Errorf("expected tool_delta, got %q", EventToolDelta)
		}
		if EventDone != "done" {
			t.Errorf("expected done, got %q", EventDone)
		}
		if EventError != "error" {
			t.Errorf("expected error, got %q", EventError)
		}
	})

	t.Run("StreamEventStruct", func(t *testing.T) {
		// Verify StreamEvent can carry all expected data
		tc := ToolCall{ID: "t1", Name: "bash", Arguments: json.RawMessage(`{}`)}
		usage := Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}

		textEvent := StreamEvent{Type: EventTextDelta, Delta: "hello"}
		if textEvent.Delta != "hello" {
			t.Errorf("expected delta 'hello', got %q", textEvent.Delta)
		}

		toolStartEvent := StreamEvent{Type: EventToolStart, ToolCall: &tc}
		if toolStartEvent.ToolCall.Name != "bash" {
			t.Errorf("expected tool name 'bash', got %q", toolStartEvent.ToolCall.Name)
		}

		doneEvent := StreamEvent{
			Type:      EventDone,
			ToolCalls: []ToolCall{tc},
			Usage:     &usage,
		}
		if len(doneEvent.ToolCalls) != 1 {
			t.Errorf("expected 1 tool call in done event, got %d", len(doneEvent.ToolCalls))
		}
		if doneEvent.Usage.TotalTokens != 30 {
			t.Errorf("expected 30 total tokens, got %d", doneEvent.Usage.TotalTokens)
		}
	})
}

// TestAnthropicChatStreamStubIntegration tests the Anthropic stub by using
// a local HTTP test server.
func TestAnthropicChatStreamStubIntegration(t *testing.T) {
	// Create a test server that returns a known Anthropic response
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{
			"content": [
				{"type": "text", "text": "Hello from stub stream!"}
			],
			"usage": {"input_tokens": 10, "output_tokens": 5},
			"stop_reason": "end_turn"
		}`
		w.Write([]byte(resp))
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	})

	messages := []Message{
		{Role: RoleUser, Content: "Hello"},
	}

	ch := p.ChatStream(messages, nil)

	var events []StreamEvent
	for event := range ch {
		events = append(events, event)
	}

	// Stub should emit: text_delta, done
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(events), events)
	}

	if events[0].Type != EventTextDelta {
		t.Errorf("expected first event to be text_delta, got %q", events[0].Type)
	}
	if events[0].Delta != "Hello from stub stream!" {
		t.Errorf("expected delta text, got %q", events[0].Delta)
	}

	if events[1].Type != EventDone {
		t.Errorf("expected last event to be done, got %q", events[1].Type)
	}
	if events[1].Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", events[1].Usage.TotalTokens)
	}
}

// TestAnthropicChatStreamStubWithToolCalls tests the stub with tool calls.
func TestAnthropicChatStreamStubWithToolCalls(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{
			"content": [
				{"type": "text", "text": "Let me check."},
				{"type": "tool_use", "id": "t1", "name": "bash", "input": {"command": "ls"}}
			],
			"usage": {"input_tokens": 20, "output_tokens": 15},
			"stop_reason": "tool_use"
		}`
		w.Write([]byte(resp))
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	})

	ch := p.ChatStream([]Message{{Role: RoleUser, Content: "List files"}}, nil)

	var events []StreamEvent
	for event := range ch {
		events = append(events, event)
	}

	// Stub should emit: text_delta, tool_start, done
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %+v", len(events), events)
	}

	if events[0].Type != EventTextDelta {
		t.Errorf("expected text_delta, got %q", events[0].Type)
	}
	if events[1].Type != EventToolStart {
		t.Errorf("expected tool_start, got %q", events[1].Type)
	}
	if events[1].ToolCall.Name != "bash" {
		t.Errorf("expected tool name 'bash', got %q", events[1].ToolCall.Name)
	}
	if events[2].Type != EventDone {
		t.Errorf("expected done, got %q", events[2].Type)
	}
	if len(events[2].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call in done event, got %d", len(events[2].ToolCalls))
	}
}

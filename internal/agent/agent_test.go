package agent

import (
	"encoding/json"
	"testing"

	agentctx "github.com/ajistrying/nbcode2/internal/context"
	"github.com/ajistrying/nbcode2/internal/config"
	"github.com/ajistrying/nbcode2/internal/permission"
	"github.com/ajistrying/nbcode2/internal/provider"
	"github.com/ajistrying/nbcode2/internal/tools"
)

// mockProvider is a fake LLM that returns predetermined responses.
type mockProvider struct {
	responses []provider.Response
	callIndex int
}

func (m *mockProvider) Chat(messages []provider.Message, toolDefs []provider.ToolDefinition) (*provider.Response, error) {
	if m.callIndex >= len(m.responses) {
		return &provider.Response{Content: "No more responses configured."}, nil
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return &resp, nil
}

func (m *mockProvider) Name() string           { return "mock" }
func (m *mockProvider) Model() string           { return "mock-model" }
func (m *mockProvider) MaxContextTokens() int   { return 100000 }

// ChatStream wraps Chat() into a single-shot channel for testing.
func (m *mockProvider) ChatStream(messages []provider.Message, toolDefs []provider.ToolDefinition) <-chan provider.StreamEvent {
	ch := make(chan provider.StreamEvent)
	go func() {
		defer close(ch)
		resp, err := m.Chat(messages, toolDefs)
		if err != nil {
			ch <- provider.StreamEvent{Type: provider.EventError, Error: err}
			return
		}
		if resp.Content != "" {
			ch <- provider.StreamEvent{Type: provider.EventTextDelta, Delta: resp.Content}
		}
		for _, tc := range resp.ToolCalls {
			tc := tc
			ch <- provider.StreamEvent{Type: provider.EventToolStart, ToolCall: &tc}
		}
		ch <- provider.StreamEvent{
			Type:      provider.EventDone,
			ToolCalls: resp.ToolCalls,
			Usage:     &resp.Usage,
		}
	}()
	return ch
}

// TestSimpleResponse verifies the agent returns a text response
// when the LLM doesn't make any tool calls.
func TestSimpleResponse(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{Content: "Hello! How can I help?", Usage: provider.Usage{TotalTokens: 100}},
		},
	}

	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
	})

	result, err := a.Run("Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Hello! How can I help?" {
		t.Errorf("expected greeting, got: %q", result)
	}
}

// TestToolUseLoop verifies the agent executes a tool call and feeds
// the result back to the LLM for a final response.
func TestToolUseLoop(t *testing.T) {
	tmpDir := t.TempDir()

	mock := &mockProvider{
		responses: []provider.Response{
			// First response: LLM wants to read a file
			{
				ToolCalls: []provider.ToolCall{
					{
						ID:        "call_1",
						Name:      "bash",
						Arguments: json.RawMessage(`{"command":"echo test-output"}`),
					},
				},
				Usage: provider.Usage{TotalTokens: 200},
			},
			// Second response: LLM gives final answer after seeing tool result
			{
				Content: "The command output was: test-output",
				Usage:   provider.Usage{TotalTokens: 400},
			},
		},
	}

	_ = tmpDir // not needed for bash echo

	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
	})

	result, err := a.Run("Run echo test-output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "The command output was: test-output" {
		t.Errorf("expected final response, got: %q", result)
	}

	// Verify the LLM was called twice (once for tool call, once for final response)
	if mock.callIndex != 2 {
		t.Errorf("expected 2 LLM calls, got %d", mock.callIndex)
	}
}

// TestMultipleToolCalls verifies the agent can handle multiple tool calls
// in a single response.
func TestMultipleToolCalls(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{
				ToolCalls: []provider.ToolCall{
					{
						ID:        "call_1",
						Name:      "bash",
						Arguments: json.RawMessage(`{"command":"echo first"}`),
					},
					{
						ID:        "call_2",
						Name:      "bash",
						Arguments: json.RawMessage(`{"command":"echo second"}`),
					},
				},
				Usage: provider.Usage{TotalTokens: 300},
			},
			{
				Content: "Both commands executed successfully.",
				Usage:   provider.Usage{TotalTokens: 500},
			},
		},
	}

	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
	})

	result, err := a.Run("Run two commands")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Both commands executed successfully." {
		t.Errorf("expected final response, got: %q", result)
	}
}

// TestToolPermissionDenied verifies that denied tool calls get an
// appropriate error message fed back to the LLM.
func TestToolPermissionDenied(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{
				ToolCalls: []provider.ToolCall{
					{
						ID:        "call_1",
						Name:      "bash",
						Arguments: json.RawMessage(`{"command":"rm -rf /"}`),
					},
				},
				Usage: provider.Usage{TotalTokens: 200},
			},
			{
				Content: "The command was denied.",
				Usage:   provider.Usage{TotalTokens: 300},
			},
		},
	}

	a := New(Config{
		Provider:   mock,
		Tools:      tools.NewRegistry(),
		Context:    agentctx.NewManager(100000, 0.8),
		Permission: permission.NewChecker(config.PermissionRiskTiered),
		SystemPrompt: "You are helpful.",
		OnConfirm: func(toolName string, args json.RawMessage) bool {
			return false // always deny
		},
	})

	result, err := a.Run("Delete everything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "The command was denied." {
		t.Errorf("expected denial response, got: %q", result)
	}

	// Check that the tool result message contains the denial
	messages := a.Messages()
	foundDenial := false
	for _, msg := range messages {
		if msg.Role == provider.RoleTool && msg.Content == "Tool execution denied by user." {
			foundDenial = true
			break
		}
	}
	if !foundDenial {
		t.Error("expected denial message in conversation context")
	}
}

// TestUnknownToolError verifies the agent handles unknown tool calls gracefully.
func TestUnknownToolError(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{
				ToolCalls: []provider.ToolCall{
					{
						ID:        "call_1",
						Name:      "nonexistent_tool",
						Arguments: json.RawMessage(`{}`),
					},
				},
				Usage: provider.Usage{TotalTokens: 200},
			},
			{
				Content: "I see that tool doesn't exist.",
				Usage:   provider.Usage{TotalTokens: 300},
			},
		},
	}

	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
	})

	result, err := a.Run("Use a fake tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The error should have been fed back to the LLM
	if result != "I see that tool doesn't exist." {
		t.Errorf("expected error handling response, got: %q", result)
	}
}

// TestTokenTracking verifies token counts are updated correctly.
func TestTokenTracking(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{Content: "Response", Usage: provider.Usage{TotalTokens: 1500}},
		},
	}

	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
	})

	a.Run("Hello")

	if a.TotalTokens() != 1500 {
		t.Errorf("expected 1500 tokens, got %d", a.TotalTokens())
	}
}

// TestStatusCallbacks verifies status updates are sent during the agent loop.
func TestStatusCallbacks(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "call_1", Name: "bash", Arguments: json.RawMessage(`{"command":"echo hi"}`)},
				},
				Usage: provider.Usage{TotalTokens: 200},
			},
			{Content: "Done.", Usage: provider.Usage{TotalTokens: 300}},
		},
	}

	var statuses []string
	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
		OnStatus: func(status string) {
			statuses = append(statuses, status)
		},
	})

	a.Run("Do something")

	// Should have received status updates: "Thinking...", "Running bash...", "Thinking...", ""
	foundThinking := false
	foundRunning := false
	for _, s := range statuses {
		if s == "Thinking..." {
			foundThinking = true
		}
		if s == "Running bash..." {
			foundRunning = true
		}
	}

	if !foundThinking {
		t.Error("expected 'Thinking...' status")
	}
	if !foundRunning {
		t.Error("expected 'Running bash...' status")
	}
}

// ─── RunStream Tests ──────────────────────────────────────────────────────────

// TestRunStreamSimpleResponse verifies RunStream returns a text response
// and delivers deltas via the callback.
func TestRunStreamSimpleResponse(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{Content: "Hello from stream!", Usage: provider.Usage{TotalTokens: 100}},
		},
	}

	var deltas []string
	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
		OnTextDelta: func(delta string) {
			deltas = append(deltas, delta)
		},
	})

	result, err := a.RunStream("Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Hello from stream!" {
		t.Errorf("expected streaming response, got: %q", result)
	}

	// The mock emits the full content as a single delta
	if len(deltas) != 1 || deltas[0] != "Hello from stream!" {
		t.Errorf("expected one delta with full content, got: %v", deltas)
	}
}

// TestRunStreamWithToolCalls verifies RunStream handles the tool-use loop
// and delivers text deltas from both iterations.
func TestRunStreamWithToolCalls(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{
				Content: "Let me run that.",
				ToolCalls: []provider.ToolCall{
					{
						ID:        "call_1",
						Name:      "bash",
						Arguments: json.RawMessage(`{"command":"echo streamed"}`),
					},
				},
				Usage: provider.Usage{TotalTokens: 200},
			},
			{
				Content: "The output was: streamed",
				Usage:   provider.Usage{TotalTokens: 400},
			},
		},
	}

	var deltas []string
	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
		OnTextDelta: func(delta string) {
			deltas = append(deltas, delta)
		},
	})

	result, err := a.RunStream("Run echo streamed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "The output was: streamed" {
		t.Errorf("expected final response, got: %q", result)
	}

	// Should have received deltas from both iterations
	if len(deltas) != 2 {
		t.Errorf("expected 2 deltas (one per iteration), got %d: %v", len(deltas), deltas)
	}

	if mock.callIndex != 2 {
		t.Errorf("expected 2 LLM calls, got %d", mock.callIndex)
	}
}

// TestRunStreamNoDeltaCallback verifies RunStream works without OnTextDelta set.
func TestRunStreamNoDeltaCallback(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{Content: "No callback here.", Usage: provider.Usage{TotalTokens: 50}},
		},
	}

	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
		// OnTextDelta intentionally nil
	})

	result, err := a.RunStream("Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "No callback here." {
		t.Errorf("expected response, got: %q", result)
	}
}

// TestRunStreamTokenTracking verifies token counts are updated from stream events.
func TestRunStreamTokenTracking(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.Response{
			{Content: "Response", Usage: provider.Usage{TotalTokens: 2500}},
		},
	}

	a := New(Config{
		Provider:     mock,
		Tools:        tools.NewRegistry(),
		Context:      agentctx.NewManager(100000, 0.8),
		Permission:   permission.NewChecker(config.PermissionYolo),
		SystemPrompt: "You are helpful.",
	})

	a.RunStream("Hello")

	if a.TotalTokens() != 2500 {
		t.Errorf("expected 2500 tokens, got %d", a.TotalTokens())
	}
}

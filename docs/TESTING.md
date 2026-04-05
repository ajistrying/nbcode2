# Testing Guide

This document explains how nbcode's test suite works, how Go interfaces enable mocking, and how to write new tests.

## Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output (see individual test names)
go test ./... -v

# Run tests for a specific package
go test ./internal/tools/ -v

# Run a specific test by name
go test ./internal/agent/ -v -run TestToolUseLoop

# Run with race detector (catches concurrency bugs)
go test ./... -race

# Run with coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out  # opens in browser
```

## How Interface-Based Mocking Works

Go's testing philosophy avoids external mocking frameworks. Instead, it uses **interfaces** — any type that implements an interface can be substituted for the real implementation.

### The Pattern

1. Define an interface for the dependency:

```go
// In provider/provider.go
type Provider interface {
    Chat(messages []Message, tools []ToolDefinition) (*Response, error)
    Name() string
    Model() string
    MaxContextTokens() int
}
```

2. The real code depends on the interface, not a concrete type:

```go
// In agent/agent.go
type Agent struct {
    provider provider.Provider  // interface, not *OpenAIProvider
}
```

3. In tests, create a mock that implements the same interface:

```go
// In agent/agent_test.go
type mockProvider struct {
    responses []provider.Response
    callIndex int
}

func (m *mockProvider) Chat(messages []provider.Message, tools []provider.ToolDefinition) (*provider.Response, error) {
    resp := m.responses[m.callIndex]
    m.callIndex++
    return &resp, nil
}
func (m *mockProvider) Name() string           { return "mock" }
func (m *mockProvider) Model() string          { return "mock-model" }
func (m *mockProvider) MaxContextTokens() int  { return 100000 }
```

4. Pass the mock into the code under test:

```go
func TestSimpleResponse(t *testing.T) {
    mock := &mockProvider{
        responses: []provider.Response{
            {Content: "Hello!", Usage: provider.Usage{TotalTokens: 100}},
        },
    }

    a := agent.New(agent.Config{
        Provider: mock,  // mock satisfies the Provider interface
        // ...
    })

    result, err := a.Run("Hi")
    // assert result == "Hello!"
}
```

### Why This Works

Go's interfaces are **implicit** — a type satisfies an interface if it has the right methods. You don't need `implements` keywords. This means:

- The mock doesn't know about `Provider`
- `Provider` doesn't know about the mock
- The `Agent` doesn't know which one it's using

This is the core testing superpower of Go's type system.

### The Same Pattern Applies to Tools

The `Tool` interface lets us test the tool registry and permission system without executing real tools:

```go
type mockTool struct {
    name     string
    riskTier tools.RiskTier
}

func (m *mockTool) Name() string                            { return m.name }
func (m *mockTool) RiskTier() tools.RiskTier                { return m.riskTier }
func (m *mockTool) Execute(args json.RawMessage) (string, error) { return "mock result", nil }
// ... other interface methods
```

## Test Categories

### Unit Tests

Test a single function or method in isolation. Examples:

- `config/config_test.go` — Tests YAML parsing, env var overrides, defaults
- `context/context_test.go` — Tests token threshold detection, message compaction
- `permission/permission_test.go` — Tests risk tier classification

These tests have **no external dependencies** (no files, no network, no processes).

### Integration Tests

Test a module against real system resources. Examples:

- `tools/tools_test.go` — Creates real temp files and runs ReadFile/WriteFile/EditFile against them
- `session/session_test.go` — Creates real JSON files on disk and loads them back

These tests use `t.TempDir()` which creates a temporary directory that's automatically cleaned up when the test finishes. This is Go's standard pattern for filesystem integration tests.

### Mock LLM Tests

Test the agent orchestration loop with a fake LLM. Examples:

- `agent/agent_test.go` — Uses `mockProvider` with predetermined responses

These tests verify:
- The tool-use loop executes tools and feeds results back
- Permission denials are handled correctly
- Unknown tools produce error messages (not crashes)
- Token tracking updates correctly
- Status callbacks fire at the right times

### How to Simulate a Tool-Use Conversation

The mock provider returns responses in order. To simulate a conversation where the LLM calls a tool and then responds:

```go
mock := &mockProvider{
    responses: []provider.Response{
        // Response 1: LLM decides to call a tool
        {
            ToolCalls: []provider.ToolCall{
                {
                    ID:        "call_1",
                    Name:      "bash",
                    Arguments: json.RawMessage(`{"command":"echo hello"}`),
                },
            },
            Usage: provider.Usage{TotalTokens: 200},
        },
        // Response 2: LLM sees the tool result and gives a final answer
        {
            Content: "The command output was: hello",
            Usage:   provider.Usage{TotalTokens: 400},
        },
    },
}
```

The agent loop calls `mock.Chat()` twice:
1. First call → gets the tool call → executes `bash echo hello` → adds result to context
2. Second call → gets text response → returns to caller

## Writing New Tests

### Adding a Test for a New Tool

1. Create a temp directory for test files:

```go
func TestMyNewTool(t *testing.T) {
    tmpDir := t.TempDir()
    // create test fixtures in tmpDir
}
```

2. Instantiate the tool and call Execute with JSON args:

```go
tool := &MyNewTool{}
args, _ := json.Marshal(myToolArgs{Path: filepath.Join(tmpDir, "test.txt")})
result, err := tool.Execute(args)
```

3. Assert the result:

```go
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
if !strings.Contains(result, "expected content") {
    t.Errorf("expected content, got: %s", result)
}
```

### Adding a Test for Agent Behavior

1. Create a `mockProvider` with the response sequence you want to test
2. Create an agent with `agent.New(...)` using the mock
3. Call `a.Run("user message")`
4. Assert the returned string and/or inspect `a.Messages()`

### Test Naming Convention

Go convention: `TestFunctionName` or `TestFunctionName_Scenario`:

```go
func TestReadFile(t *testing.T) { ... }           // happy path
func TestReadFileNotFound(t *testing.T) { ... }    // error case
func TestReadFileWithOffset(t *testing.T) { ... }  // specific behavior
```

### Subtests

Use `t.Run()` for related test cases:

```go
func TestRiskTieredMode(t *testing.T) {
    tests := []struct {
        name     string
        riskTier tools.RiskTier
        expected Decision
    }{
        {"safe tool", tools.RiskSafe, Allowed},
        {"medium tool", tools.RiskMedium, NeedsApproval},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

### What NOT to Test

- **Don't test implementation details.** If you refactor how `EditFile` reads the file internally (e.g., switching from `os.ReadFile` to buffered reading), the test should still pass because the external behavior (search-and-replace) hasn't changed.

- **Don't test the TUI.** Visual rendering is tested manually. The TUI module is thin — it just calls agent methods and displays results.

- **Don't test the LLM's intelligence.** Mock LLM tests verify the orchestration loop, not whether the model makes good decisions.

## Current Test Coverage

| Package     | Tests | What's Covered |
|-------------|-------|----------------|
| `agent`     | 7     | Simple response, tool loop, multi-tool, permission denial, unknown tool, token tracking, status callbacks |
| `config`    | 5     | Defaults, YAML parsing, env overrides, env > config precedence, no-config fallback |
| `context`   | 7     | Constructor, default threshold, add/messages, summarize threshold, zero max, compact, message count |
| `permission`| 4     | Risk-tiered mode, yolo mode, always-ask mode, real tool tier verification |
| `provider`  | 7     | OpenAI message/tool conversion, Anthropic message/tool conversion, tool result merging |
| `session`   | 5     | Save/load round-trip, nonexistent load, listing (sort order), latest, preview truncation |
| `tools`     | 18    | ReadFile (happy, offset, not-found), WriteFile (happy, mkdir), EditFile (happy, not-found, no-match, multi-match), ListFiles (happy, pattern), Search (happy, no-match), Bash (happy, exit code, timeout), Registry (lookup, definitions) |

# Architecture Walkthrough

This document traces the complete lifecycle of a user interaction with `nbcode` — from launching the CLI to receiving a response. It is written for someone who wants to understand every layer of the system.

## High-Level Overview

```
┌─────────────────────────────────────────────────────┐
│                    cmd/nbcode/main.go                │
│  (entry point: loads config, creates components,     │
│   wires them together, starts TUI)                   │
└──────────────┬──────────────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────────────┐
│                   internal/tui/                       │
│  Bubble Tea app (Model-Update-View)                  │
│  - Renders chat pane, status bar, input              │
│  - Sends user messages to Agent                      │
│  - Displays responses and confirmation prompts       │
└──────────────┬──────────────────────────────────────┘
               │ calls agent.Run(userMessage)
               ▼
┌──────────────────────────────────────────────────────┐
│                  internal/agent/                      │
│  The orchestration loop:                             │
│  1. Add user message to context                      │
│  2. Call provider.Chat(messages, tools)               │
│  3. If tool calls → execute each → add results       │
│  4. Loop back to step 2                              │
│  5. If no tool calls → return text response           │
└───┬──────────┬───────────┬──────────────────────────┘
    │          │           │
    ▼          ▼           ▼
┌────────┐ ┌────────┐ ┌──────────────────┐
│provider│ │ tools  │ │   context        │
│        │ │        │ │                  │
│OpenAI  │ │ReadFile│ │Message history   │
│Anthropic│ │Bash   │ │Token tracking    │
│        │ │Edit...│ │Auto-summarize    │
└────────┘ └───┬────┘ └──────────────────┘
               │
               ▼
         ┌───────────┐
         │permission │
         │           │
         │Risk tiers │
         │Confirm/   │
         │Allow/Deny │
         └───────────┘
```

## Startup Sequence

When you run `nbcode`, here's what happens in order:

### 1. Config Loading (`internal/config/`)

```
main.go → config.Load()
```

The config system loads settings in two layers:

1. **Read `~/.nbcode/config.yaml`** — If the file exists, parse it into a `Config` struct. If it doesn't exist, start with `DefaultConfig()` (which sets provider to "openai", permissions to "risk-tiered", summarization threshold to 0.8).

2. **Apply environment variable overrides** — Check for `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`, `ANTHROPIC_MODEL`, `NBCODE_PROVIDER`, and `NBCODE_PERMISSIONS`. Any env var that is set overwrites the corresponding config value. This is why env vars always win over the config file.

### 2. Provider Creation

```
main.go → createProvider(cfg)
```

Based on `cfg.DefaultProvider`:
- If `"anthropic"` → creates an `AnthropicProvider` (native API, uses HTTP client directly)
- Anything else → creates an `OpenAIProvider` (uses `sashabaranov/go-openai` SDK)

Both implement the `Provider` interface, so the rest of the system doesn't care which one is active.

### 3. Component Assembly

```
main.go → creates Registry, ContextManager, PermissionChecker, builds system prompt
```

- **Tool Registry** (`tools.NewRegistry()`) — Instantiates all 7 built-in tools and stores them in a map by name. The sub-agent tool is registered later by the Agent itself, because it needs a reference back to the agent's `runSubAgent` method.

- **Context Manager** (`context.NewManager(maxTokens, threshold)`) — Initialized with the provider's context window size and the 80% summarization threshold.

- **Permission Checker** (`permission.NewChecker(mode)`) — Takes the configured permission mode (risk-tiered, always-ask, or yolo).

- **System Prompt** (`prompt.Build(workDir)`) — Assembles the dynamic system prompt:
  1. Base prompt (hardcoded in the binary — the agent's core instructions)
  2. Environment context (working directory, OS, shell, git branch/status)
  3. Project instructions from `nbcode.md` (if found at the git repo root)

### 4. Agent Creation

```
main.go → agent.New(cfg)
```

The Agent constructor:
1. Creates the system message and adds it to the context manager
2. Registers the sub-agent tool with its runner function
3. Stores references to all components

### 5. TUI Launch

```
main.go → tea.NewProgram(model, tea.WithAltScreen())
```

Bubble Tea takes over the terminal. The TUI model initializes:
- Textarea for input
- Viewport for chat history
- Spinner for loading states
- Status bar with model name, token count, git info

## The Request Lifecycle

Here's what happens when you type a message and press Enter:

### Step 1: User Input → TUI

```
tui.go Update() → tea.KeyEnter → sendMessage(input)
```

The TUI captures the Enter key, reads the textarea value, appends it to the chat display, sets `loading = true`, and dispatches a Bubble Tea command that calls `agent.RunStream(input)` in a goroutine.

### Step 2: Agent Receives Message

```
agent.go RunStream() → context.Add(userMessage)
```

The user's text is wrapped in a `Message{Role: RoleUser, Content: input}` and added to the context manager's message history.

### Step 3: LLM Call (Streaming)

```
agent.go → provider.ChatStream(context.Messages(), tools.Definitions())
```

The agent sends the entire conversation history plus tool definitions to the LLM provider via `ChatStream()`, which returns a `<-chan StreamEvent`. The agent ranges over this channel, processing events as they arrive:

- **`text_delta`** — Forwarded immediately to the TUI via `OnTextDelta` callback. The TUI accumulates deltas in a buffer and re-renders on a 50ms throttled tick (raw text during streaming, Glamour markdown on completion).
- **`tool_start`** — Notifies the TUI that a tool call is beginning.
- **`tool_delta`** — Tool call arguments accumulating (handled by provider internally).
- **`done`** — Stream complete. Contains final tool calls and usage stats.
- **`error`** — Stream error, propagated to caller.

The non-streaming `Chat()` method is still available and used by sub-agents and summarization where streaming adds no user-visible value.

**Note:** The `Run()` method (non-streaming) is preserved alongside `RunStream()` for sub-agents and internal use.

**If using OpenAI-compatible provider:**
- Messages are converted via `toOpenAIMessages()` — a straightforward mapping
- Tools are converted via `toOpenAITools()` — wraps each tool definition in OpenAI's function calling format
- The `go-openai` SDK handles the HTTP request, auth, and response parsing

**If using Anthropic provider:**
- `toAnthropicMessages()` does more work:
  - Extracts the system message (Anthropic puts it in a separate field, not in the messages array)
  - Converts assistant tool calls to `tool_use` content blocks
  - Converts tool result messages to `tool_result` content blocks inside user messages
  - Merges consecutive tool results into a single user message (Anthropic's requirement)
- The HTTP request is built manually (setting `x-api-key` and `anthropic-version` headers)

### Step 4: Response Processing

The provider returns a `Response` containing:
- `Content` — text response (may be empty if tool calls are present)
- `ToolCalls` — zero or more tool invocations
- `Usage` — token counts (prompt, completion, total)

Token usage is recorded: `context.UpdateUsage(resp.Usage)`

### Step 5: Tool Execution (if tool calls present)

```
for each toolCall in response.ToolCalls:
    tool = registry.Get(toolCall.Name)
    decision = permission.Check(tool)
    if decision == NeedsApproval:
        approved = onConfirm(toolName, args)  // asks user via TUI
        if !approved: result = "denied"
    result = tool.Execute(toolCall.Arguments)
    context.Add(Message{Role: RoleTool, Content: result, ToolCallID: toolCall.ID})
```

For each tool call:

1. **Look up the tool** in the registry by name. If not found, the error is added to context so the model can self-correct.

2. **Check permissions** via the permission checker:
   - `Allowed` (safe tools in risk-tiered mode, or yolo mode) → execute immediately
   - `NeedsApproval` → send a `ConfirmRequestMsg` to the TUI, which shows "Allow bash? (echo hello) [y/N]" and blocks until the user responds
   - `Denied` → return denial message

3. **Execute the tool** — Each tool's `Execute()` method receives the JSON arguments, validates them, does its work, and returns a string result. The result is added to the conversation as a `RoleTool` message.

4. **Status updates** — Before each tool execution, `onStatus("Running bash...")` is called, which sends a `StatusUpdateMsg` to the TUI to update the spinner text.

### Step 6: Loop or Return

After executing all tool calls from a response, the agent loops back to Step 3 — sending the updated conversation (now including tool results) back to the LLM.

If the LLM's response has no tool calls (just text), the loop ends and the text is returned to the TUI.

### Step 7: Context Window Check

```
if context.ShouldSummarize():
    summarize()
```

After each LLM response, the agent checks if total tokens have exceeded 80% of the provider's context window. If so:

1. A summarization request is sent to the LLM (using the same provider — ideally you'd configure a cheaper model)
2. The conversation is compacted: all messages are replaced with the system prompt + a summary message
3. Token count is reset

### Step 8: Response Display

```
tui.go → agentResponseMsg{content: result}
```

During streaming, text deltas arrive at the TUI as `TextDeltaMsg` messages. These are accumulated in a buffer and rendered as raw text on a 50ms tick. When the agent loop finishes, the final `agentResponseMsg` arrives:

1. Sets `loading = false`, clears streaming state
2. Appends the complete assistant message to the chat display
3. Renders the full response with Glamour markdown
4. Re-renders the viewport and scrolls to the bottom

This produces a smooth experience: raw text appears token-by-token during streaming, then "snaps" to formatted markdown on completion.

## Sub-Agent Execution

When the LLM calls the `sub_agent` tool:

1. The `SubAgentTool.Execute()` calls the runner function injected by the parent agent
2. `agent.runSubAgent(task)` creates a fresh context manager and message history
3. A child tool-use loop runs (max 20 iterations) using the parent's provider
4. The child has all tools except `sub_agent` (preventing recursive spawning)
5. The child uses the same permission checker (writes/executions still need approval)
6. When the child loop finishes, it returns a string summary to the parent
7. The parent agent treats this summary as the tool's result and continues

## Module Dependency Graph

```
main.go
  ├── config      (no internal deps)
  ├── provider    (no internal deps)
  ├── tools       (depends on: provider — for ToolDefinition type)
  ├── context     (depends on: provider — for Message/Usage types)
  ├── permission  (depends on: config, tools — for mode and risk tiers)
  ├── git         (no internal deps)
  ├── prompt      (depends on: git — for repo info)
  ├── session     (depends on: provider — for Message type)
  ├── agent       (depends on: provider, tools, context, permission)
  └── tui         (depends on: agent, git)
```

Note: there are no circular dependencies. Data types flow down from `provider` (which defines `Message`, `ToolCall`, etc.) and up through `agent` (which orchestrates everything).

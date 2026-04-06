# Future Roadmap

This document describes features that were intentionally deferred from v1 and how to think about prioritizing them.

## How to Prioritize

When deciding what to build next, ask:

1. **What's causing the most friction right now?** Use nbcode daily and notice where you get stuck. That's your highest-priority improvement.
2. **What's the smallest change that unlocks the most value?** A 50-line change that saves you time every session beats a 500-line feature you'll use once a month.
3. **Does this need to be in the core, or can it be a tool?** Many features can be implemented as new tools without touching the agent loop.

## Deferred Features

### High Value — Build These First

#### ~~Token-by-Token Streaming~~ ✓ Implemented
**Status:** Completed. Tokens stream to the TUI as they arrive.
**What was built:**
- `ChatStream(messages, tools) <-chan StreamEvent` method on the `Provider` interface
- `StreamEvent` supports `text_delta`, `tool_start`, `tool_delta`, `done`, and `error` event types
- OpenAI: native streaming via `CreateChatCompletionStream` from go-openai SDK
- Anthropic: stub implementation wrapping `Chat()` into single-shot channel events (native SSE streaming is a future enhancement)
- Agent: `RunStream()` method that forwards text deltas via `OnTextDelta` callback
- TUI: 50ms throttled render tick, raw text during streaming, Glamour markdown on completion
- `Run()` and `Chat()` preserved for sub-agents and summarization where streaming adds no value

#### Repo Map (Tree-Sitter)
**Current:** The agent uses `list_files` and `search` to explore codebases.
**Improvement:** Generate a condensed map of the repo's structure (files, functions, classes, signatures) that fits in context.
**Why deferred:** Requires tree-sitter bindings per language. Significant engineering effort.
**How to implement:**
- Use `github.com/smacker/go-tree-sitter` for Go tree-sitter bindings
- Parse source files to extract function/class/method signatures
- Build a condensed "repo map" string that lists files and their key symbols
- Include this in the system prompt or as a tool result
- Start with Go and Python, add languages incrementally

#### Session Resume UX
**Current:** Sessions are saved but there's no TUI for browsing/resuming them.
**Improvement:** `nbcode --resume` to continue the last session, `nbcode --sessions` to list and pick.
**How to implement:**
- Add CLI flags in `main.go` using `flag` package or `cobra`
- If `--resume`: load `sessionStore.Latest()`, restore messages to context manager
- If `--sessions`: display a list picker (Bubble Tea has a `list` component in `bubbles`)

### Medium Value — Build When You Feel the Pain

#### Command Allowlists
**Current:** All bash commands go through the same confirmation prompt.
**Improvement:** Auto-approve safe commands like `go test`, `git status`, `ls`.
**How to implement:**
- Add an `allowed_commands` list to config
- In permission checker, match bash commands against patterns before deciding
- Be careful with shell expansion — `ls` is safe but `ls; rm -rf /` is not

#### Configurable Summarization Model
**Current:** Summarization uses the same provider/model as coding.
**Improvement:** Use a cheaper, faster model for summarization.
**How to implement:**
- The `SummarizationConfig` already has `Provider` and `Model` fields
- Create a second provider instance in `main.go` for summarization
- Pass it to the agent separately
- Use it in `agent.summarize()` instead of `a.provider`

#### Git Worktree Sub-Agents
**Current:** Sub-agents share the filesystem with the parent.
**Improvement:** Each sub-agent gets a git worktree (isolated copy of the repo).
**How to implement:**
- `git worktree add /tmp/nbcode-subagent-xxx -b temp-branch`
- Run the sub-agent with that directory as its working directory
- After completion, merge or cherry-pick changes back
- Clean up: `git worktree remove /tmp/nbcode-subagent-xxx`
- This eliminates all file conflict concerns

#### Additional Providers
Add implementations for:
- **Google Gemini** — Different API format, needs its own provider implementation
- **Azure OpenAI** — Almost identical to OpenAI but with different auth (Azure AD tokens, deployment-based URLs)
- **Groq / Together / Fireworks** — OpenAI-compatible, just need base URL and model name config
- **Ollama** — Already works via OpenAI-compatible, but a native integration could use Ollama's pull/list APIs

### Lower Priority — Nice to Have

#### Sandboxed Execution
**Current:** Bash commands run directly on the host with user approval.
**Improvement:** Run commands in Docker containers or macOS sandbox.
**How to implement:**
- Wrap bash tool execution in `docker run -v $(pwd):/workspace -w /workspace`
- Or use macOS `sandbox-exec` with a restrictive profile
- Make it configurable — some users want raw access, others want safety

#### IDE Integration
**Current:** Terminal only.
**Improvement:** VSCode extension, Neovim plugin.
**How to implement:**
- Run nbcode as a language server or JSON-RPC server
- IDE extensions send messages and display responses
- This is a big project — consider whether the TUI is sufficient

#### Long-Term Memory
**Current:** Per-session persistence only.
**Improvement:** Cross-session memory (remember user preferences, project context).
**How to implement:**
- Memory file in `.nbcode/memory.md` (per-project)
- Agent can read/write to it via a `memory` tool
- Loaded into system prompt on startup
- Be careful about stale memory — add timestamps and expiry

#### Multi-File Diff View
**Current:** File edits show inline in the chat.
**Improvement:** Side-by-side diff view in the TUI when files are edited.
**How to implement:**
- Capture file state before and after edits
- Render a unified diff using `github.com/aymanbagabas/go-udiff`
- Add a TUI panel that shows diffs on demand

## Architecture Guidelines for New Features

### Adding a New Tool

1. Create `internal/tools/mytool.go`
2. Implement the `Tool` interface (Name, Description, Parameters, Execute, RiskTier)
3. Add it to the `builtins` slice in `tools.NewRegistry()`
4. Write tests in `internal/tools/tools_test.go`
5. The agent automatically picks it up — no changes to the agent loop needed

### Adding a New Provider

1. Create `internal/provider/myprovider.go`
2. Implement the `Provider` interface (Chat, Name, Model, MaxContextTokens)
3. Handle the API's message format conversion in private functions
4. Add a case in `main.go createProvider()` for the new provider name
5. Write message conversion tests in `internal/provider/provider_test.go`

### Modifying the Agent Loop

The agent loop in `agent.go Run()` is the most sensitive code. Changes here affect everything. Before modifying:
- Understand the full loop (see ARCHITECTURE.md)
- Add mock LLM tests that verify the new behavior
- Test with multiple providers (the loop should be provider-agnostic)

### Modifying the TUI

Bubble Tea uses the Elm architecture:
- `Model` — all state lives here
- `Update(msg) → (Model, Cmd)` — handle events, return new state + side effects
- `View() → string` — render current state to a string

To add a new UI element:
1. Add state to the `Model` struct
2. Handle relevant messages in `Update()`
3. Render it in `View()`
4. If it needs data from the agent, add a new message type

# NB Code Skill System — Implementation Plan

## Design Decisions (Locked)

| Decision | Choice |
|---|---|
| Scope | Full implementation |
| Discovery paths | Agent Skills standard (`.agents/skills/`) + NB Code paths (`~/.nbcode/skills/`, `.nbcode/skills/`) |
| Invocation | Both: user `/skill-name` in TUI + LLM `skill` tool (automatic) |
| Registry | Auto-populated from discovered skills on startup |
| Execution | Forked (isolated sub-agent) as default; inline as option |
| Forked agent tools | Same tool set as parent |
| Iteration cap | Removed for skill execution (skills can run as long as needed) |
| Standard compliance | Fully compliant with Agent Skills spec + own paths |
| Architecture | New `internal/skills/` package |

## Dependencies to Add

```
github.com/adrg/frontmatter    # YAML frontmatter parsing from markdown
github.com/google/shlex         # Shell-like argument tokenization
github.com/bmatcuk/doublestar/v4  # Glob pattern matching (for paths: field)
```

---

## Package: `internal/skills/`

### File: `skill.go` — Core Types

```go
package skills

// Frontmatter represents parsed YAML frontmatter from SKILL.md
type Frontmatter struct {
    Name                   string   `yaml:"name"`
    Description            string   `yaml:"description"`
    AllowedTools           []string `yaml:"allowed-tools"`
    ArgumentHint           string   `yaml:"argument-hint"`
    Model                  string   `yaml:"model"`
    Context                string   `yaml:"context"`                // "inline" or "fork" (default: "inline")
    Agent                  string   `yaml:"agent"`
    Paths                  []string `yaml:"paths"`                  // glob patterns for conditional activation
    UserInvocable          string   `yaml:"user-invocable"`         // "true"/"false" (default: "true")
    DisableModelInvocation string   `yaml:"disable-model-invocation"` // "true"/"false"
    Shell                  string   `yaml:"shell"`                  // "bash" (default) or "powershell"
    Effort                 string   `yaml:"effort"`
    Version                string   `yaml:"version"`
}

// Skill represents a discovered and parsed skill
type Skill struct {
    Frontmatter Frontmatter
    Body        string   // Markdown content after frontmatter
    Dir         string   // Absolute path to the skill directory
    FilePath    string   // Absolute path to SKILL.md
    Source      string   // "user", "project", "agents-standard"
    Priority    int      // Lower = higher priority (for dedup)
}

// Name returns the skill's canonical name (from frontmatter, or directory name)
func (s *Skill) Name() string

// IsUserInvocable returns whether the user can invoke this with /name
func (s *Skill) IsUserInvocable() bool

// IsModelInvocable returns whether the LLM can invoke this via the skill tool
func (s *Skill) IsModelInvocable() bool

// IsForked returns whether this skill runs in an isolated sub-agent
func (s *Skill) IsForked() bool

// IsConditional returns whether this skill has paths: conditions
func (s *Skill) IsConditional() bool
```

### File: `discovery.go` — Directory Scanning

```go
package skills

// DiscoveryConfig controls where skills are discovered
type DiscoveryConfig struct {
    UserDir    string   // ~/.nbcode/skills/
    ProjectDir string   // resolved from cwd
    AgentsDirs []string // .agents/skills/ (standard)
    ExtraDirs  []string // any additional directories
}

// DefaultDiscoveryConfig returns the standard discovery configuration
// scanning paths in priority order:
//   1. ~/.nbcode/skills/          (user personal)
//   2. .nbcode/skills/            (project, walk up to $HOME)
//   3. .agents/skills/            (Agent Skills standard, walk up to $HOME)
//   4. <project>/skills/          (project root — NB Code convention)
func DefaultDiscoveryConfig(cwd string) *DiscoveryConfig

// Discover scans all configured directories and returns deduplicated skills.
// Priority: user > project (.nbcode) > agents standard > project root.
// Dedup by canonical path (realpath). First-found wins on name collision.
func Discover(cfg *DiscoveryConfig) ([]*Skill, error)

// scanDir reads a single skills directory and returns found skills.
// Each subdirectory containing SKILL.md becomes a skill.
func scanDir(dir string, source string, priority int) ([]*Skill, error)

// walkUpToHome walks from cwd up to $HOME, checking for skillsSubdir at each level.
func walkUpToHome(cwd string, skillsSubdir string) []string
```

**Discovery priority order:**
1. `~/.nbcode/skills/` — user personal (always scanned)
2. Walk CWD → $HOME checking `.nbcode/skills/` — project-level NB Code
3. Walk CWD → $HOME checking `.agents/skills/` — Agent Skills standard
4. `<git-root>/skills/` — project root convention (where the Rails skill lives now)

### File: `parser.go` — Frontmatter + Body Processing

```go
package skills

// Parse reads a SKILL.md file and returns a parsed Skill.
// Uses github.com/adrg/frontmatter for YAML extraction.
func Parse(filePath string) (*Skill, error)

// PrepareBody processes the skill body for injection into context:
//   1. Substitutes $ARGUMENTS, $0, $1, etc.
//   2. Substitutes ${CLAUDE_SKILL_DIR} with the skill's directory path
//   3. Executes !`command` and ```! blocks, replaces with output
// Returns the processed markdown body ready for injection.
func PrepareBody(skill *Skill, args string) (string, error)

// substituteArgs replaces $ARGUMENTS, $ARGUMENTS[N], $N placeholders.
// Uses github.com/google/shlex for tokenization.
func substituteArgs(body string, rawArgs string) string

// substituteVars replaces ${CLAUDE_SKILL_DIR} and similar variables.
func substituteVars(body string, skill *Skill) string

// executeShellBlocks finds !`command` and ```! blocks,
// executes them, and replaces with output.
// Respects skill.Frontmatter.Shell (default: "bash").
func executeShellBlocks(body string, shell string) (string, error)
```

### File: `catalog.go` — System Prompt Injection

```go
package skills

// CatalogConfig controls how the skill catalog is built
type CatalogConfig struct {
    MaxChars int // Budget for catalog text (default: 1% of context window)
}

// BuildCatalog generates the skill catalog text for system prompt injection.
// Format:
//   The following skills are available for use with the skill tool:
//   - skill-name: description (truncated to fit budget)
//   - another-skill: description
//
// If budget is exceeded, descriptions are progressively truncated.
// Skills with disable-model-invocation: true are excluded from the catalog
// but still available via /name.
func BuildCatalog(skills []*Skill, cfg CatalogConfig) string

// BuildUserCommandList generates the list of /commands available in the TUI.
// Includes all skills where user-invocable is true (default).
// Format suitable for display in help text.
func BuildUserCommandList(skills []*Skill) string
```

### File: `executor.go` — Skill Execution

```go
package skills

import (
    "github.com/ajistrying/nbcode2/internal/provider"
    "github.com/ajistrying/nbcode2/internal/tools"
    agentctx "github.com/ajistrying/nbcode2/internal/context"
    "github.com/ajistrying/nbcode2/internal/permission"
)

// ExecutorConfig holds dependencies needed for skill execution
type ExecutorConfig struct {
    Provider   provider.Provider
    Tools      *tools.Registry
    Permission *permission.Checker
    OnConfirm  func(toolName string, args json.RawMessage) bool
    OnStatus   func(status string)
    OnToolCall func(toolName string, args json.RawMessage)
    SystemMsg  provider.Message
}

// Executor handles running skills (inline or forked)
type Executor struct {
    config ExecutorConfig
}

// NewExecutor creates a skill executor with the given dependencies
func NewExecutor(cfg ExecutorConfig) *Executor

// Execute runs a skill with the given arguments.
// If skill.IsForked(), spawns an isolated sub-agent.
// Otherwise, returns the prepared body for inline injection.
//
// For forked execution:
//   - Creates fresh context.Manager (no iteration cap)
//   - Uses same tool registry as parent
//   - Runs full agent loop until completion
//   - Returns the final assistant response text
//
// For inline execution:
//   - Returns the prepared body text (caller injects into conversation)
func (e *Executor) Execute(skill *Skill, args string) (string, error)

// executeForked spawns an isolated sub-agent for the skill.
// No iteration cap — skills run until the LLM stops making tool calls.
func (e *Executor) executeForked(skill *Skill, body string) (string, error)

// executeInline returns the processed body for injection into the conversation.
func (e *Executor) executeInline(skill *Skill, body string) (string, error)
```

### File: `registry.go` — Skill Registry

```go
package skills

// Registry manages discovered skills and provides lookup
type Registry struct {
    skills       map[string]*Skill // name -> skill
    conditional  map[string]*Skill // name -> skill (with paths: field)
    discoveryConfig *DiscoveryConfig
}

// NewRegistry discovers and loads all skills, returns populated registry
func NewRegistry(cwd string) (*Registry, error)

// Get returns a skill by name, or error if not found
func (r *Registry) Get(name string) (*Skill, error)

// All returns all non-conditional skills
func (r *Registry) All() []*Skill

// UserInvocable returns skills the user can invoke with /name
func (r *Registry) UserInvocable() []*Skill

// ModelInvocable returns skills the LLM can invoke via the skill tool
func (r *Registry) ModelInvocable() []*Skill

// MatchConditional checks if any conditional skills match the given file path
// and returns them (used when files are touched during the session)
func (r *Registry) MatchConditional(filePath string) []*Skill

// Reload re-scans discovery paths and updates the registry
// (for hot-reload if we want it later)
func (r *Registry) Reload() error
```

### File: `tool.go` — Skill Tool for LLM Invocation

```go
package skills

import (
    "github.com/ajistrying/nbcode2/internal/tools"
)

// SkillTool implements tools.Tool, allowing the LLM to invoke skills.
// Registered in the tool registry so the model sees it as an available tool.
type SkillTool struct {
    registry *Registry
    executor *Executor
}

// NewSkillTool creates the tool that the LLM uses to invoke skills
func NewSkillTool(registry *Registry, executor *Executor) *SkillTool

// Name returns "skill"
func (t *SkillTool) Name() string { return "skill" }

// Description returns guidance for the LLM on when/how to use skills
func (t *SkillTool) Description() string {
    // "Execute a skill by name. Use this when a user's request matches
    //  an available skill from the skill catalog. Pass the skill name
    //  and any arguments."
}

// Parameters returns JSON schema:
// {
//   "type": "object",
//   "properties": {
//     "skill_name": { "type": "string", "description": "Name of the skill to invoke" },
//     "args": { "type": "string", "description": "Arguments to pass to the skill" }
//   },
//   "required": ["skill_name"]
// }
func (t *SkillTool) Parameters() json.RawMessage

// Execute looks up the skill, validates it's model-invocable,
// prepares the body with args, and runs it via the executor.
func (t *SkillTool) Execute(args json.RawMessage) (string, error)

// RiskTier returns RiskMedium (skills execute code, need approval by default)
func (t *SkillTool) RiskTier() tools.RiskTier { return tools.RiskMedium }
```

---

## Integration Points

### 1. `cmd/nbcode/main.go` — Initialization

```go
// After existing setup...

// Discover and load skills
skillRegistry, err := skills.NewRegistry(workDir)
if err != nil {
    log.Printf("Warning: skill discovery failed: %v", err)
    // Non-fatal — agent works without skills
}

// Build skill catalog for system prompt
catalog := skills.BuildCatalog(skillRegistry.All(), skills.CatalogConfig{
    MaxChars: prov.MaxContextTokens() / 100, // 1% of context
})

// Append catalog to system prompt
systemPrompt := prompt.Build(workDir)
if catalog != "" {
    systemPrompt += "\n\n" + catalog
}

// Create skill executor
skillExecutor := skills.NewExecutor(skills.ExecutorConfig{
    Provider:   prov,
    Tools:      registry,
    Permission: permChecker,
    OnConfirm:  confirmFunc,
    OnStatus:   statusFunc,
    OnToolCall: toolCallFunc,
    SystemMsg:  provider.Message{Role: provider.RoleSystem, Content: systemPrompt},
})

// Register the skill tool so the LLM can invoke skills
skillTool := skills.NewSkillTool(skillRegistry, skillExecutor)
registry.Register(skillTool)

// Pass skill registry + executor to TUI for /command handling
tuiModel := tui.New(a, prov, workDir, skillRegistry, skillExecutor)
```

### 2. `internal/tui/tui.go` — Slash Command Routing

```go
// In the Model struct, add:
type Model struct {
    // ...existing fields...
    skillRegistry *skills.Registry
    skillExecutor *skills.Executor
}

// In Update(), replace the /quit check with a command router:
case tea.KeyEnter:
    input := strings.TrimSpace(m.textarea.Value())
    if input == "" || m.loading || m.confirming {
        break
    }

    if strings.HasPrefix(input, "/") {
        return m.handleSlashCommand(input)
    }

    // ...existing sendMessage logic...

// New method:
func (m *Model) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
    // Parse: "/skill-name arg1 arg2" → name="skill-name", args="arg1 arg2"
    parts := strings.SplitN(input[1:], " ", 2)
    name := parts[0]
    args := ""
    if len(parts) > 1 {
        args = parts[1]
    }

    // Built-in commands
    switch name {
    case "quit", "exit":
        return m, tea.Quit
    case "help":
        return m.showHelp()
    case "skills":
        return m.listSkills()
    }

    // Skill lookup
    skill, err := m.skillRegistry.Get(name)
    if err != nil {
        m.appendMessage("system", fmt.Sprintf("Unknown command: /%s", name))
        return m, nil
    }

    if !skill.IsUserInvocable() {
        m.appendMessage("system", fmt.Sprintf("Skill '%s' is not user-invocable", name))
        return m, nil
    }

    // Execute skill (async, same pattern as sendMessage)
    m.loading = true
    m.textarea.Reset()
    return m, m.executeSkill(skill, args)
}

func (m *Model) executeSkill(skill *Skill, args string) tea.Cmd {
    return func() tea.Msg {
        result, err := m.skillExecutor.Execute(skill, args)
        if err != nil {
            return agentResponseMsg(fmt.Sprintf("Skill error: %v", err))
        }

        if skill.IsForked() {
            // Forked: result is the complete output
            return agentResponseMsg(result)
        }

        // Inline: inject body into conversation, let agent process it
        response, err := m.agent.Run(result)
        if err != nil {
            return agentResponseMsg(fmt.Sprintf("Error: %v", err))
        }
        return agentResponseMsg(response)
    }
}
```

### 3. `internal/agent/agent.go` — Forked Execution Support

The `Executor.executeForked()` method needs to create a child agent. Two options:

**Option A: Export a factory function from agent package**

```go
// In agent.go, add:

// RunIsolated creates a temporary agent with its own context and runs
// a single task to completion. No iteration cap.
// Used by the skill executor for forked skill execution.
func RunIsolated(cfg Config, task string) (string, error) {
    child := New(cfg)
    // Override: no sub-agent tool in isolated runs
    // Override: no iteration cap

    // Run the full agent loop
    return child.Run(task)
}
```

**Option B: Refactor runSubAgent to accept options**

```go
type SubAgentOptions struct {
    MaxIterations int  // 0 = unlimited
    Task          string
    SystemPrompt  string // override system prompt
    ToolFilter    func(tools.Tool) bool // optional tool filtering
}

func (a *Agent) RunSubAgent(opts SubAgentOptions) (string, error)
```

**Recommendation: Option B** — it extends the existing pattern without duplicating agent creation logic. The skill executor calls `agent.RunSubAgent()` with `MaxIterations: 0`.

This requires making `RunSubAgent` exported (capital R) and changing the existing `runSubAgent` to call it with `MaxIterations: 20`.

### 4. `internal/prompt/prompt.go` — Catalog Injection

Two approaches:

**Approach A: Caller appends catalog to prompt** (shown in main.go above)
Simple, no changes to prompt package.

**Approach B: prompt.Build accepts optional sections**

```go
func Build(workDir string, extraSections ...string) string {
    parts := []string{basePrompt, buildEnvContext(workDir), loadProjectConfig(workDir)}
    parts = append(parts, extraSections...)
    return strings.Join(parts, "\n\n")
}
```

**Recommendation: Approach A** for now. Keep it simple.

---

## Forked Execution Flow (Detailed)

```
User types: /rails-new myapp
        │
        ▼
TUI.handleSlashCommand()
  ├─ Parse name="rails-new", args="myapp"
  ├─ skillRegistry.Get("rails-new") → Skill
  ├─ skill.IsForked() → true
  │
  ▼
skillExecutor.Execute(skill, "myapp")
  ├─ PrepareBody(skill, "myapp")
  │   ├─ substituteArgs: $0 → "myapp", $ARGUMENTS → "myapp"
  │   ├─ substituteVars: ${CLAUDE_SKILL_DIR} → "/path/to/skills/rails-new"
  │   └─ executeShellBlocks: run any !`cmd` blocks
  │
  ├─ skill.IsForked() → executeForked()
  │   ├─ Create fresh context.Manager (provider.MaxContextTokens(), 0.8)
  │   ├─ Build system message (base prompt + "You are executing a skill...")
  │   ├─ Add prepared body as user message
  │   ├─ Create agent via agent.RunIsolated() or agent.RunSubAgent()
  │   │   ├─ Uses SAME tool registry (all tools available)
  │   │   ├─ Uses SAME permission checker
  │   │   ├─ NO iteration cap
  │   │   ├─ Runs full agentic loop:
  │   │   │   ├─ LLM reads skill instructions
  │   │   │   ├─ Calls bash tool: rails new myapp ...
  │   │   │   ├─ Calls write_file: create config files
  │   │   │   ├─ Calls bash tool: bundle install
  │   │   │   ├─ Calls edit_file: modify configs
  │   │   │   ├─ ... (as many iterations as needed)
  │   │   │   ├─ Calls bash tool: bundle exec rspec
  │   │   │   └─ Returns final summary
  │   │   │
  │   │   └─ Returns final assistant text
  │   │
  │   └─ Returns result string
  │
  └─ Return to TUI as agentResponseMsg
```

---

## Implementation Order

### Phase 1: Core Package (no integration yet)
1. `skill.go` — types, Skill methods
2. `parser.go` — frontmatter parsing, body preparation
3. `discovery.go` — directory scanning, dedup
4. `registry.go` — skill registry
5. `catalog.go` — catalog text generation
6. **Tests for all of the above**

### Phase 2: Execution
7. `executor.go` — inline + forked execution
8. Modify `agent.go` — export RunSubAgent with options, remove iteration cap for skills
9. `tool.go` — SkillTool for LLM invocation
10. **Tests for execution**

### Phase 3: Integration
11. Modify `main.go` — wire up discovery, registry, executor, skill tool
12. Modify `tui.go` — slash command routing, /help, /skills
13. Modify `prompt.go` — catalog injection
14. **Integration tests**

### Phase 4: Polish
15. Error handling and user-friendly messages
16. `/skills` command to list available skills
17. `/help` updated with skill info
18. Conditional skills (paths: matching) — activate when files are touched
19. Hot-reload (optional — `Reload()` on file change)

---

## Testing Strategy

```
internal/skills/
  skill_test.go       — Skill methods, IsForked, IsUserInvocable, etc.
  parser_test.go      — Frontmatter parsing, arg substitution, shell blocks
  discovery_test.go   — Directory scanning with temp dirs, dedup, priority
  registry_test.go    — Lookup, filtering, conditional matching
  catalog_test.go     — Budget enforcement, progressive truncation
  executor_test.go    — Mock provider, verify forked vs inline behavior
  tool_test.go        — SkillTool.Execute with mock registry/executor
```

Test fixtures: create a `testdata/` directory with sample SKILL.md files covering edge cases (missing frontmatter, invalid YAML, various field combinations).

---

## File Inventory

**New files (9):**
- `internal/skills/skill.go`
- `internal/skills/discovery.go`
- `internal/skills/parser.go`
- `internal/skills/catalog.go`
- `internal/skills/executor.go`
- `internal/skills/registry.go`
- `internal/skills/tool.go`
- `internal/skills/testdata/` (test fixtures)
- `internal/skills/*_test.go` (7 test files)

**Modified files (3):**
- `cmd/nbcode/main.go` — skill initialization + wiring
- `internal/tui/tui.go` — slash command router + skill execution
- `internal/agent/agent.go` — exported RunSubAgent with configurable iteration cap

**New dependencies (3):**
- `github.com/adrg/frontmatter`
- `github.com/google/shlex`
- `github.com/bmatcuk/doublestar/v4`

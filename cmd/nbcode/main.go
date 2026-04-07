package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ajistrying/nbcode2/internal/agent"
	agentctx "github.com/ajistrying/nbcode2/internal/context"
	"github.com/ajistrying/nbcode2/internal/config"
	"github.com/ajistrying/nbcode2/internal/permission"
	"github.com/ajistrying/nbcode2/internal/prompt"
	"github.com/ajistrying/nbcode2/internal/provider"
	"github.com/ajistrying/nbcode2/internal/skills"
	"github.com/ajistrying/nbcode2/internal/tools"
	"github.com/ajistrying/nbcode2/internal/tui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("nbcode %s\n", version)
		os.Exit(0)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	llmProvider, err := createProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating provider: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nSet up a provider via environment variables:\n")
		fmt.Fprintf(os.Stderr, "  ANTHROPIC_API_KEY=sk-ant-...  (for Claude)\n")
		fmt.Fprintf(os.Stderr, "  OPENAI_API_KEY=sk-...         (for OpenAI)\n")
		fmt.Fprintf(os.Stderr, "  OPENAI_BASE_URL=http://...    (for vLLM/HF)\n")
		fmt.Fprintf(os.Stderr, "\nOr create ~/.nbcode/config.yaml\n")
		os.Exit(1)
	}

	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Build components
	registry := tools.NewRegistry()
	ctxManager := agentctx.NewManager(llmProvider.MaxContextTokens(), cfg.Summarization.Threshold)
	permChecker := permission.NewChecker(cfg.Permissions)
	systemPrompt := prompt.Build(workDir)

	// Discover and load skills
	skillRegistry, err := skills.NewRegistry(workDir)
	if err != nil {
		log.Printf("Warning: skill discovery failed: %v", err)
	}

	// Append skill catalog to system prompt
	if skillRegistry != nil && skillRegistry.Count() > 0 {
		catalog := skills.BuildCatalog(skillRegistry.All(), skills.CatalogConfig{
			MaxChars: llmProvider.MaxContextTokens() / 100,
		})
		if catalog != "" {
			systemPrompt += "\n\n" + catalog
		}
	}

	// Confirmation channel for TUI ↔ agent communication
	confirmChan := make(chan bool, 1)

	var tuiProgram *tea.Program

	onConfirm := func(toolName string, args json.RawMessage) bool {
		tuiProgram.Send(tui.ConfirmRequestMsg{
			ToolName: toolName,
			Args:     args,
			Respond:  confirmChan,
		})
		return <-confirmChan
	}

	onStatus := func(status string) {
		if tuiProgram != nil {
			tuiProgram.Send(tui.StatusUpdateMsg(status))
		}
	}

	onToolCall := func(toolName string, args json.RawMessage) {
		if tuiProgram != nil {
			tuiProgram.Send(tui.ToolCallMsg{
				ToolName: toolName,
				Args:     args,
			})
		}
	}

	agentCfg := agent.Config{
		Provider:     llmProvider,
		Tools:        registry,
		Context:      ctxManager,
		Permission:   permChecker,
		SystemPrompt: systemPrompt,
		OnConfirm:  onConfirm,
		OnStatus:   onStatus,
		OnToolCall: onToolCall,
		OnTextDelta: func(delta string) {
			if tuiProgram != nil {
				tuiProgram.Send(tui.TextDeltaMsg(delta))
			}
		},
	}

	a := agent.New(agentCfg)

	// Create skill executor (shares tools, permissions, and callbacks with agent)
	systemMsg := provider.Message{Role: provider.RoleSystem, Content: systemPrompt}
	var skillExecutor *skills.Executor
	if skillRegistry != nil {
		skillExecutor = skills.NewExecutor(skills.ExecutorConfig{
			Provider:   llmProvider,
			Tools:      registry,
			Permission: permChecker,
			OnConfirm:  onConfirm,
			OnStatus:   onStatus,
			OnToolCall: onToolCall,
			SystemMsg:  systemMsg,
		})

		// Register the skill tool so the LLM can invoke skills
		skillTool := skills.NewSkillTool(skillRegistry, skillExecutor)
		registry.Register(skillTool)
	}

	model := tui.New(a, llmProvider, workDir, skillRegistry, skillExecutor)
	tuiProgram = tea.NewProgram(model, tea.WithAltScreen())

	if _, err := tuiProgram.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createProvider(cfg *config.Config) (provider.Provider, error) {
	providerName := cfg.DefaultProvider

	provCfg, exists := cfg.Providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %q not configured. Set API key via environment variable or config file", providerName)
	}

	if provCfg.APIKey == "" {
		return nil, fmt.Errorf("no API key for provider %q", providerName)
	}

	switch providerName {
	case "anthropic":
		model := provCfg.DefaultModel
		if model == "" {
			model = "claude-sonnet-4-20250514"
		}
		return provider.NewAnthropicProvider(provider.AnthropicConfig{
			APIKey:  provCfg.APIKey,
			BaseURL: provCfg.BaseURL,
			Model:   model,
		}), nil

	default:
		// Treat everything else as OpenAI-compatible
		model := provCfg.DefaultModel
		if model == "" {
			model = "gpt-5.4-mini-2026-03-17"
		}
		return provider.NewOpenAIProvider(provider.OpenAIConfig{
			APIKey:  provCfg.APIKey,
			BaseURL: provCfg.BaseURL,
			Model:   model,
			Name:    providerName,
		}), nil
	}
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ajistrying/nbcode2/internal/agent"
	agentctx "github.com/ajistrying/nbcode2/internal/context"
	"github.com/ajistrying/nbcode2/internal/config"
	"github.com/ajistrying/nbcode2/internal/permission"
	"github.com/ajistrying/nbcode2/internal/prompt"
	"github.com/ajistrying/nbcode2/internal/provider"
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

	// Confirmation channel for TUI ↔ agent communication
	confirmChan := make(chan bool, 1)

	var tuiProgram *tea.Program

	agentCfg := agent.Config{
		Provider:     llmProvider,
		Tools:        registry,
		Context:      ctxManager,
		Permission:   permChecker,
		SystemPrompt: systemPrompt,
		OnConfirm: func(toolName string, args json.RawMessage) bool {
			// Send confirm request to TUI
			tuiProgram.Send(tui.ConfirmRequestMsg{
				ToolName: toolName,
				Args:     args,
				Respond:  confirmChan,
			})
			// Block until user responds
			return <-confirmChan
		},
		OnStatus: func(status string) {
			if tuiProgram != nil {
				tuiProgram.Send(tui.StatusUpdateMsg(status))
			}
		},
	}

	a := agent.New(agentCfg)
	model := tui.New(a, llmProvider.Model(), workDir)
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
			model = "gpt-4o"
		}
		return provider.NewOpenAIProvider(provider.OpenAIConfig{
			APIKey:  provCfg.APIKey,
			BaseURL: provCfg.BaseURL,
			Model:   model,
			Name:    providerName,
		}), nil
	}
}

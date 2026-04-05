package provider

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider for any OpenAI-compatible endpoint
// (OpenAI, vLLM, HuggingFace Inference Endpoints, Ollama).
type OpenAIProvider struct {
	client   *openai.Client
	model    string
	name     string
	maxTokens int
}

// OpenAIConfig holds configuration for creating an OpenAI-compatible provider.
type OpenAIConfig struct {
	APIKey    string
	BaseURL   string
	Model     string
	Name      string // display name, e.g. "openai", "vllm", "huggingface"
	MaxTokens int    // context window size
}

// NewOpenAIProvider creates a provider for any OpenAI-compatible API.
func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	clientCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientCfg.BaseURL = cfg.BaseURL
	}

	name := cfg.Name
	if name == "" {
		name = "openai"
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 128000 // sensible default
	}

	return &OpenAIProvider{
		client:    openai.NewClientWithConfig(clientCfg),
		model:     cfg.Model,
		name:      name,
		maxTokens: maxTokens,
	}
}

func (p *OpenAIProvider) Name() string           { return p.name }
func (p *OpenAIProvider) Model() string           { return p.model }
func (p *OpenAIProvider) MaxContextTokens() int   { return p.maxTokens }

func (p *OpenAIProvider) Chat(messages []Message, tools []ToolDefinition) (*Response, error) {
	req := openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: toOpenAIMessages(messages),
	}

	if len(tools) > 0 {
		req.Tools = toOpenAITools(tools)
	}

	resp, err := p.client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("openai chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai: no choices returned")
	}

	choice := resp.Choices[0]
	result := &Response{
		Content: choice.Message.Content,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	for _, tc := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}

	return result, nil
}

func toOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	out := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, m := range messages {
		msg := openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}

		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}

		if m.Name != "" {
			msg.Name = m.Name
		}

		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tc.Name,
					Arguments: string(tc.Arguments),
				},
			})
		}

		out = append(out, msg)
	}
	return out
}

func toOpenAITools(tools []ToolDefinition) []openai.Tool {
	out := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		params := make(map[string]any)
		if len(t.Parameters) > 0 {
			json.Unmarshal(t.Parameters, &params)
		}
		out = append(out, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return out
}

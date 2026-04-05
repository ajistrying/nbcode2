package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AnthropicProvider implements Provider for Anthropic's native API.
type AnthropicProvider struct {
	apiKey    string
	baseURL   string
	model     string
	maxTokens int
	httpClient *http.Client
}

// AnthropicConfig holds configuration for creating an Anthropic provider.
type AnthropicConfig struct {
	APIKey    string
	BaseURL   string
	Model     string
	MaxTokens int // context window size
}

// NewAnthropicProvider creates a provider for Anthropic's native API.
func NewAnthropicProvider(cfg AnthropicConfig) *AnthropicProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 200000 // Claude default
	}

	return &AnthropicProvider{
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		model:      cfg.Model,
		maxTokens:  maxTokens,
		httpClient: &http.Client{},
	}
}

func (p *AnthropicProvider) Name() string           { return "anthropic" }
func (p *AnthropicProvider) Model() string           { return p.model }
func (p *AnthropicProvider) MaxContextTokens() int   { return p.maxTokens }

// Anthropic API request/response types

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	StopReason string `json:"stop_reason"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Error   struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *AnthropicProvider) Chat(messages []Message, tools []ToolDefinition) (*Response, error) {
	systemPrompt, anthropicMsgs := toAnthropicMessages(messages)

	req := anthropicRequest{
		Model:     p.model,
		MaxTokens: 8192,
		System:    systemPrompt,
		Messages:  anthropicMsgs,
	}

	if len(tools) > 0 {
		req.Tools = toAnthropicTools(tools)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var apiErr anthropicError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("anthropic API error (%d): %s", httpResp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("anthropic API error (%d): %s", httpResp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &Response{
		Usage: Usage{
			PromptTokens:     apiResp.Usage.InputTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			TotalTokens:      apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		},
	}

	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}

	return result, nil
}

// toAnthropicMessages converts our messages to Anthropic format.
// Extracts the system message and converts tool call/result messages.
func toAnthropicMessages(messages []Message) (string, []anthropicMessage) {
	var system string
	var out []anthropicMessage

	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			system = m.Content

		case RoleUser:
			out = append(out, anthropicMessage{
				Role:    "user",
				Content: m.Content,
			})

		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				blocks := make([]anthropicContentBlock, 0)
				if m.Content != "" {
					blocks = append(blocks, anthropicContentBlock{
						Type: "text",
						Text: m.Content,
					})
				}
				for _, tc := range m.ToolCalls {
					blocks = append(blocks, anthropicContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Name,
						Input: tc.Arguments,
					})
				}
				out = append(out, anthropicMessage{
					Role:    "assistant",
					Content: blocks,
				})
			} else {
				out = append(out, anthropicMessage{
					Role:    "assistant",
					Content: m.Content,
				})
			}

		case RoleTool:
			// Anthropic expects tool results as user messages with tool_result content blocks
			block := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}
			// Check if last message is already a user message with tool results — merge
			if len(out) > 0 && out[len(out)-1].Role == "user" {
				if blocks, ok := out[len(out)-1].Content.([]anthropicContentBlock); ok {
					out[len(out)-1].Content = append(blocks, block)
					continue
				}
			}
			out = append(out, anthropicMessage{
				Role:    "user",
				Content: []anthropicContentBlock{block},
			})
		}
	}

	return system, out
}

func toAnthropicTools(tools []ToolDefinition) []anthropicTool {
	out := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	return out
}

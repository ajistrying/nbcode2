package context

import (
	"github.com/ajistrying/nbcode2/internal/provider"
)

// Manager tracks conversation messages and monitors context window usage.
type Manager struct {
	messages      []provider.Message
	totalTokens   int
	maxTokens     int
	threshold     float64 // 0.0-1.0, triggers summarization
}

// NewManager creates a context manager for the given provider's window size.
func NewManager(maxTokens int, threshold float64) *Manager {
	if threshold <= 0 || threshold > 1.0 {
		threshold = 0.8
	}
	return &Manager{
		messages:  make([]provider.Message, 0),
		maxTokens: maxTokens,
		threshold: threshold,
	}
}

// Add appends a message to the conversation.
func (m *Manager) Add(msg provider.Message) {
	m.messages = append(m.messages, msg)
}

// Messages returns all messages in the conversation.
func (m *Manager) Messages() []provider.Message {
	return m.messages
}

// UpdateUsage records token usage from the latest provider response.
func (m *Manager) UpdateUsage(usage provider.Usage) {
	m.totalTokens = usage.TotalTokens
}

// ShouldSummarize returns true if we've hit the summarization threshold.
func (m *Manager) ShouldSummarize() bool {
	if m.maxTokens == 0 {
		return false
	}
	return float64(m.totalTokens) >= float64(m.maxTokens)*m.threshold
}

// TotalTokens returns the current token count.
func (m *Manager) TotalTokens() int {
	return m.totalTokens
}

// Compact replaces all messages with a summary message.
// Called after summarization completes.
func (m *Manager) Compact(summary string, systemMsg provider.Message) {
	m.messages = []provider.Message{
		systemMsg,
		{
			Role:    provider.RoleUser,
			Content: "Here is a summary of our conversation so far:\n\n" + summary,
		},
	}
	// Reset token count — will be updated on next provider call
	m.totalTokens = 0
}

// MessageCount returns the number of messages.
func (m *Manager) MessageCount() int {
	return len(m.messages)
}

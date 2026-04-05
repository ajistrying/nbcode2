package context

import (
	"testing"

	"github.com/ajistrying/nbcode2/internal/provider"
)

func TestNewManager(t *testing.T) {
	m := NewManager(100000, 0.8)

	if m.maxTokens != 100000 {
		t.Errorf("expected maxTokens 100000, got %d", m.maxTokens)
	}
	if m.threshold != 0.8 {
		t.Errorf("expected threshold 0.8, got %f", m.threshold)
	}
	if len(m.Messages()) != 0 {
		t.Errorf("expected empty messages, got %d", len(m.Messages()))
	}
}

func TestNewManagerDefaultThreshold(t *testing.T) {
	m := NewManager(100000, 0) // 0 should default to 0.8
	if m.threshold != 0.8 {
		t.Errorf("expected default threshold 0.8, got %f", m.threshold)
	}

	m = NewManager(100000, -1) // negative should default to 0.8
	if m.threshold != 0.8 {
		t.Errorf("expected default threshold 0.8, got %f", m.threshold)
	}
}

func TestAddAndMessages(t *testing.T) {
	m := NewManager(100000, 0.8)

	m.Add(provider.Message{Role: provider.RoleSystem, Content: "You are helpful."})
	m.Add(provider.Message{Role: provider.RoleUser, Content: "Hello"})
	m.Add(provider.Message{Role: provider.RoleAssistant, Content: "Hi!"})

	msgs := m.Messages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Role != provider.RoleSystem {
		t.Errorf("expected system role, got %s", msgs[0].Role)
	}
	if msgs[1].Content != "Hello" {
		t.Errorf("expected 'Hello', got %q", msgs[1].Content)
	}
}

func TestShouldSummarize(t *testing.T) {
	m := NewManager(100000, 0.8)

	// Below threshold
	m.UpdateUsage(provider.Usage{TotalTokens: 50000})
	if m.ShouldSummarize() {
		t.Error("should not summarize at 50% usage")
	}

	// At threshold
	m.UpdateUsage(provider.Usage{TotalTokens: 80000})
	if !m.ShouldSummarize() {
		t.Error("should summarize at 80% usage")
	}

	// Above threshold
	m.UpdateUsage(provider.Usage{TotalTokens: 95000})
	if !m.ShouldSummarize() {
		t.Error("should summarize at 95% usage")
	}
}

func TestShouldSummarizeZeroMax(t *testing.T) {
	m := NewManager(0, 0.8)
	m.UpdateUsage(provider.Usage{TotalTokens: 50000})

	if m.ShouldSummarize() {
		t.Error("should not summarize with zero max tokens")
	}
}

func TestCompact(t *testing.T) {
	m := NewManager(100000, 0.8)

	systemMsg := provider.Message{Role: provider.RoleSystem, Content: "system prompt"}
	m.Add(systemMsg)
	m.Add(provider.Message{Role: provider.RoleUser, Content: "message 1"})
	m.Add(provider.Message{Role: provider.RoleAssistant, Content: "response 1"})
	m.Add(provider.Message{Role: provider.RoleUser, Content: "message 2"})
	m.Add(provider.Message{Role: provider.RoleAssistant, Content: "response 2"})

	if len(m.Messages()) != 5 {
		t.Fatalf("expected 5 messages before compact, got %d", len(m.Messages()))
	}

	m.Compact("This is a summary of our conversation.", systemMsg)

	msgs := m.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after compact, got %d", len(msgs))
	}
	if msgs[0].Role != provider.RoleSystem {
		t.Errorf("expected system message first, got %s", msgs[0].Role)
	}
	if msgs[1].Role != provider.RoleUser {
		t.Errorf("expected user message (summary) second, got %s", msgs[1].Role)
	}
	if m.TotalTokens() != 0 {
		t.Errorf("expected zero tokens after compact, got %d", m.TotalTokens())
	}
}

func TestMessageCount(t *testing.T) {
	m := NewManager(100000, 0.8)
	if m.MessageCount() != 0 {
		t.Errorf("expected 0 messages, got %d", m.MessageCount())
	}

	m.Add(provider.Message{Role: provider.RoleUser, Content: "hi"})
	if m.MessageCount() != 1 {
		t.Errorf("expected 1 message, got %d", m.MessageCount())
	}
}

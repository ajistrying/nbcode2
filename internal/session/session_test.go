package session

import (
	"testing"
	"time"

	"github.com/ajistrying/nbcode2/internal/provider"
)

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	session := &Session{
		ID:        "test-session-1",
		CreatedAt: time.Now(),
		Model:     "claude-sonnet-4-20250514",
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "You are helpful."},
			{Role: provider.RoleUser, Content: "Hello"},
			{Role: provider.RoleAssistant, Content: "Hi there!"},
		},
	}

	if err := store.Save(session); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	loaded, err := store.Load("test-session-1")
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}

	if loaded.ID != "test-session-1" {
		t.Errorf("expected ID 'test-session-1', got %q", loaded.ID)
	}
	if loaded.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model 'claude-sonnet-4-20250514', got %q", loaded.Model)
	}
	if len(loaded.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(loaded.Messages))
	}
	if loaded.Messages[1].Content != "Hello" {
		t.Errorf("expected user message 'Hello', got %q", loaded.Messages[1].Content)
	}
}

func TestLoadNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	_, err := store.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	// Save two sessions
	s1 := &Session{
		ID:        "session-old",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		Model:     "gpt-4o",
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "First conversation"},
		},
	}
	s2 := &Session{
		ID:        "session-new",
		CreatedAt: time.Now(),
		Model:     "claude-sonnet-4-20250514",
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Second conversation"},
		},
	}

	store.Save(s1)
	time.Sleep(10 * time.Millisecond) // ensure different UpdatedAt
	store.Save(s2)

	metas, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error listing: %v", err)
	}

	if len(metas) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(metas))
	}

	// Newest first
	if metas[0].ID != "session-new" {
		t.Errorf("expected newest first, got %q", metas[0].ID)
	}

	// Preview should contain first user message
	if metas[0].Preview != "Second conversation" {
		t.Errorf("expected preview 'Second conversation', got %q", metas[0].Preview)
	}
}

func TestLatest(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	// No sessions
	if latest := store.Latest(); latest != "" {
		t.Errorf("expected empty latest, got %q", latest)
	}

	store.Save(&Session{
		ID:        "only-session",
		CreatedAt: time.Now(),
		Messages:  []provider.Message{},
	})

	if latest := store.Latest(); latest != "only-session" {
		t.Errorf("expected 'only-session', got %q", latest)
	}
}

func TestPreviewTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	longMessage := ""
	for i := 0; i < 200; i++ {
		longMessage += "x"
	}

	store.Save(&Session{
		ID:        "long-preview",
		CreatedAt: time.Now(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: longMessage},
		},
	})

	metas, _ := store.List()
	if len(metas[0].Preview) > 84 { // 80 + "..."
		t.Errorf("expected truncated preview, got length %d", len(metas[0].Preview))
	}
}

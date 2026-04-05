package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ajistrying/nbcode2/internal/provider"
)

// Session represents a saved conversation.
type Session struct {
	ID        string             `json:"id"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	Model     string             `json:"model"`
	Messages  []provider.Message `json:"messages"`
}

// Meta is a lightweight session summary for listing.
type Meta struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Model     string    `json:"model"`
	MsgCount  int       `json:"msg_count"`
	Preview   string    `json:"preview"` // first user message
}

// Store handles session persistence in .nbcode/sessions/.
type Store struct {
	dir string
}

// NewStore creates a session store for the given project directory.
func NewStore(projectDir string) (*Store, error) {
	dir := filepath.Join(projectDir, ".nbcode", "sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating sessions directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Save persists a session to disk.
func (s *Store) Save(session *Session) error {
	session.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	path := filepath.Join(s.dir, session.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// Load reads a session from disk.
func (s *Store) Load(id string) (*Session, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parsing session: %w", err)
	}

	return &session, nil
}

// List returns metadata for all saved sessions, newest first.
func (s *Store) List() ([]Meta, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	var metas []Meta
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}

		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}

		preview := ""
		for _, msg := range sess.Messages {
			if msg.Role == provider.RoleUser && msg.Content != "" {
				preview = msg.Content
				if len(preview) > 80 {
					preview = preview[:80] + "..."
				}
				break
			}
		}

		metas = append(metas, Meta{
			ID:        sess.ID,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
			Model:     sess.Model,
			MsgCount:  len(sess.Messages),
			Preview:   preview,
		})
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})

	return metas, nil
}

// Latest returns the most recently updated session ID, or empty string if none.
func (s *Store) Latest() string {
	metas, err := s.List()
	if err != nil || len(metas) == 0 {
		return ""
	}
	return metas[0].ID
}

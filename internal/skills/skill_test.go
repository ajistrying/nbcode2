package skills

import "testing"

func TestSkillName(t *testing.T) {
	tests := []struct {
		name     string
		skill    Skill
		expected string
	}{
		{
			name: "uses frontmatter name",
			skill: Skill{
				Frontmatter: Frontmatter{Name: "my-skill"},
				Dir:         "/path/to/some-dir",
			},
			expected: "my-skill",
		},
		{
			name: "derives from directory",
			skill: Skill{
				Dir: "/path/to/rails-new",
			},
			expected: "rails-new",
		},
		{
			name: "handles trailing slash",
			skill: Skill{
				Dir: "/path/to/rails-new/",
			},
			expected: "rails-new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.skill.Name()
			if got != tt.expected {
				t.Errorf("Name() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSkillFlags(t *testing.T) {
	t.Run("IsUserInvocable defaults to true", func(t *testing.T) {
		s := &Skill{}
		if !s.IsUserInvocable() {
			t.Error("expected IsUserInvocable to default to true")
		}
	})

	t.Run("IsUserInvocable false when set", func(t *testing.T) {
		s := &Skill{Frontmatter: Frontmatter{UserInvocable: "false"}}
		if s.IsUserInvocable() {
			t.Error("expected IsUserInvocable to be false")
		}
	})

	t.Run("IsModelInvocable defaults to true", func(t *testing.T) {
		s := &Skill{}
		if !s.IsModelInvocable() {
			t.Error("expected IsModelInvocable to default to true")
		}
	})

	t.Run("IsModelInvocable false when disabled", func(t *testing.T) {
		s := &Skill{Frontmatter: Frontmatter{DisableModelInvocation: "true"}}
		if s.IsModelInvocable() {
			t.Error("expected IsModelInvocable to be false")
		}
	})

	t.Run("IsForked", func(t *testing.T) {
		s := &Skill{Frontmatter: Frontmatter{Context: "fork"}}
		if !s.IsForked() {
			t.Error("expected IsForked to be true")
		}
	})

	t.Run("IsForked defaults to false", func(t *testing.T) {
		s := &Skill{}
		if s.IsForked() {
			t.Error("expected IsForked to default to false")
		}
	})

	t.Run("IsConditional with paths", func(t *testing.T) {
		s := &Skill{Frontmatter: Frontmatter{Paths: []string{"src/**/*.ts"}}}
		if !s.IsConditional() {
			t.Error("expected IsConditional to be true")
		}
	})

	t.Run("IsConditional without paths", func(t *testing.T) {
		s := &Skill{}
		if s.IsConditional() {
			t.Error("expected IsConditional to default to false")
		}
	})
}

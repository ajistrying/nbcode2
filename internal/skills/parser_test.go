package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test-skill")
	_ = os.MkdirAll(skillDir, 0o755)

	content := `---
name: test-skill
description: A test skill
context: fork
allowed-tools:
  - bash
  - read_file
argument-hint: <app-name>
disable-model-invocation: "true"
---

# Test Skill

Do something with $0.
`
	skillPath := filepath.Join(skillDir, "SKILL.md")
	_ = os.WriteFile(skillPath, []byte(content), 0o644)

	skill, err := Parse(skillPath)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if skill.Name() != "test-skill" {
		t.Errorf("Name() = %q, want %q", skill.Name(), "test-skill")
	}

	if skill.Description() != "A test skill" {
		t.Errorf("Description() = %q, want %q", skill.Description(), "A test skill")
	}

	if !skill.IsForked() {
		t.Error("expected IsForked to be true")
	}

	if skill.IsModelInvocable() {
		t.Error("expected IsModelInvocable to be false")
	}

	if len(skill.Frontmatter.AllowedTools) != 2 {
		t.Errorf("AllowedTools len = %d, want 2", len(skill.Frontmatter.AllowedTools))
	}

	if !strings.Contains(skill.Body, "Do something with $0") {
		t.Errorf("Body should contain skill instructions, got: %s", skill.Body)
	}

	if skill.Dir != skillDir {
		t.Errorf("Dir = %q, want %q", skill.Dir, skillDir)
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := "# Just markdown\n\nNo frontmatter here."
	skillPath := filepath.Join(dir, "SKILL.md")
	_ = os.WriteFile(skillPath, []byte(content), 0o644)

	skill, err := Parse(skillPath)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if skill.Frontmatter.Name != "" {
		t.Errorf("expected empty name, got %q", skill.Frontmatter.Name)
	}

	if !strings.Contains(skill.Body, "Just markdown") {
		t.Errorf("Body should contain full content")
	}
}

func TestSubstituteArgs(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		args     string
		expected string
	}{
		{
			name:     "replaces $0",
			body:     "Create app $0",
			args:     "myapp",
			expected: "Create app myapp",
		},
		{
			name:     "replaces multiple indexed args",
			body:     "Move $0 to $1",
			args:     "foo bar",
			expected: "Move foo to bar",
		},
		{
			name:     "replaces $ARGUMENTS",
			body:     "Run with: $ARGUMENTS",
			args:     "foo bar baz",
			expected: "Run with: foo bar baz",
		},
		{
			name:     "replaces $ARGUMENTS[N]",
			body:     "First: $ARGUMENTS[0], Second: $ARGUMENTS[1]",
			args:     "alpha beta",
			expected: "First: alpha, Second: beta",
		},
		{
			name:     "handles quoted args",
			body:     "Name: $0, Desc: $1",
			args:     `myapp "hello world"`,
			expected: "Name: myapp, Desc: hello world",
		},
		{
			name:     "empty args leaves placeholders",
			body:     "Create app $0",
			args:     "",
			expected: "Create app $0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteArgs(tt.body, tt.args)
			if got != tt.expected {
				t.Errorf("substituteArgs() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSubstituteVars(t *testing.T) {
	skill := &Skill{Dir: "/home/user/.nbcode/skills/rails-new"}
	body := "Run: bash ${CLAUDE_SKILL_DIR}/scripts/validate.sh"
	expected := "Run: bash /home/user/.nbcode/skills/rails-new/scripts/validate.sh"

	got := substituteVars(body, skill)
	if got != expected {
		t.Errorf("substituteVars() = %q, want %q", got, expected)
	}
}

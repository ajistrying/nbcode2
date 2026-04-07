package skills

import "strings"

// Frontmatter represents parsed YAML frontmatter from SKILL.md.
type Frontmatter struct {
	Name                   string   `yaml:"name"`
	Description            string   `yaml:"description"`
	AllowedTools           []string `yaml:"allowed-tools"`
	ArgumentHint           string   `yaml:"argument-hint"`
	Model                  string   `yaml:"model"`
	Context                string   `yaml:"context"`                  // "inline" or "fork" (default: "inline")
	Agent                  string   `yaml:"agent"`
	Paths                  []string `yaml:"paths"`                    // glob patterns for conditional activation
	UserInvocable          string   `yaml:"user-invocable"`           // "true"/"false" (default: "true")
	DisableModelInvocation string   `yaml:"disable-model-invocation"` // "true"/"false"
	Shell                  string   `yaml:"shell"`                    // "bash" (default)
	Effort                 string   `yaml:"effort"`
	Version                string   `yaml:"version"`
}

// Skill represents a discovered and parsed skill.
type Skill struct {
	Frontmatter Frontmatter
	Body        string // Markdown content after frontmatter
	Dir         string // Absolute path to the skill directory
	FilePath    string // Absolute path to SKILL.md
	Source      string // "user", "project", "agents-standard", "project-root"
	Priority    int    // Lower = higher priority (for dedup)
}

// Name returns the skill's canonical name.
// Uses frontmatter name if set, otherwise derives from directory name.
func (s *Skill) Name() string {
	if s.Frontmatter.Name != "" {
		return s.Frontmatter.Name
	}
	// Derive from directory: /path/to/my-skill/ → "my-skill"
	dir := strings.TrimRight(s.Dir, "/")
	idx := strings.LastIndex(dir, "/")
	if idx >= 0 {
		return dir[idx+1:]
	}
	return dir
}

// Description returns the skill's description for the catalog.
func (s *Skill) Description() string {
	return s.Frontmatter.Description
}

// IsUserInvocable returns whether the user can invoke this with /name.
func (s *Skill) IsUserInvocable() bool {
	v := strings.ToLower(s.Frontmatter.UserInvocable)
	// Default is true — only false if explicitly set
	return v != "false"
}

// IsModelInvocable returns whether the LLM can invoke this via the skill tool.
func (s *Skill) IsModelInvocable() bool {
	v := strings.ToLower(s.Frontmatter.DisableModelInvocation)
	// Default is invocable — only disabled if explicitly set
	return v != "true"
}

// IsForked returns whether this skill runs in an isolated sub-agent.
func (s *Skill) IsForked() bool {
	return strings.ToLower(s.Frontmatter.Context) == "fork"
}

// IsConditional returns whether this skill has paths: conditions.
func (s *Skill) IsConditional() bool {
	return len(s.Frontmatter.Paths) > 0
}

package skills

import (
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
)

// Registry manages discovered skills and provides lookup.
type Registry struct {
	skills      map[string]*Skill // name → skill
	conditional map[string]*Skill // name → skill (with paths: field)
	all         []*Skill          // ordered list
}

// NewRegistry discovers and loads all skills for the given working directory.
func NewRegistry(cwd string) (*Registry, error) {
	cfg := DefaultDiscoveryConfig(cwd)
	discovered, err := Discover(cfg)
	if err != nil {
		return nil, fmt.Errorf("skill discovery: %w", err)
	}

	r := &Registry{
		skills:      make(map[string]*Skill),
		conditional: make(map[string]*Skill),
	}

	for _, s := range discovered {
		name := s.Name()
		if s.IsConditional() {
			r.conditional[name] = s
		} else {
			r.skills[name] = s
		}
		r.all = append(r.all, s)
	}

	return r, nil
}

// Get returns a skill by name, checking both regular and conditional skills.
func (r *Registry) Get(name string) (*Skill, error) {
	if s, ok := r.skills[name]; ok {
		return s, nil
	}
	if s, ok := r.conditional[name]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("unknown skill: %s", name)
}

// All returns all discovered skills.
func (r *Registry) All() []*Skill {
	return r.all
}

// UserInvocable returns skills the user can invoke with /name.
func (r *Registry) UserInvocable() []*Skill {
	var result []*Skill
	for _, s := range r.all {
		if s.IsUserInvocable() {
			result = append(result, s)
		}
	}
	return result
}

// ModelInvocable returns skills the LLM can invoke via the skill tool.
func (r *Registry) ModelInvocable() []*Skill {
	var result []*Skill
	for _, s := range r.all {
		if s.IsModelInvocable() {
			result = append(result, s)
		}
	}
	return result
}

// MatchConditional returns conditional skills whose paths: patterns
// match the given file path.
func (r *Registry) MatchConditional(filePath string) []*Skill {
	var result []*Skill
	for _, s := range r.conditional {
		for _, pattern := range s.Frontmatter.Paths {
			matched, err := doublestar.Match(pattern, filePath)
			if err == nil && matched {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

// Count returns the total number of discovered skills.
func (r *Registry) Count() int {
	return len(r.all)
}

// Names returns all skill names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.all))
	for _, s := range r.all {
		names = append(names, s.Name())
	}
	return names
}

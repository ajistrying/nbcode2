package skills

import (
	"fmt"
	"strings"
)

// CatalogConfig controls how the skill catalog is built.
type CatalogConfig struct {
	MaxChars int // Budget for catalog text (default: 1% of context tokens)
}

// BuildCatalog generates the skill catalog text for system prompt injection.
// Skills with disable-model-invocation: true are excluded.
// Descriptions are progressively truncated if the budget is exceeded.
func BuildCatalog(skills []*Skill, cfg CatalogConfig) string {
	// Filter to model-invocable skills only
	var invocable []*Skill
	for _, s := range skills {
		if s.IsModelInvocable() {
			invocable = append(invocable, s)
		}
	}

	if len(invocable) == 0 {
		return ""
	}

	maxDescLen := 250 // start with full descriptions

	for {
		catalog := buildCatalogText(invocable, maxDescLen)
		if cfg.MaxChars <= 0 || len(catalog) <= cfg.MaxChars || maxDescLen <= 0 {
			return catalog
		}
		// Progressively truncate descriptions
		maxDescLen -= 50
		if maxDescLen < 0 {
			maxDescLen = 0
		}
	}
}

func buildCatalogText(skills []*Skill, maxDescLen int) string {
	var sb strings.Builder
	sb.WriteString("# Available Skills\n\n")
	sb.WriteString("The following skills are available. Use the `skill` tool to invoke them:\n\n")

	for _, s := range skills {
		name := s.Name()
		desc := s.Description()

		if maxDescLen > 0 && len(desc) > maxDescLen {
			desc = desc[:maxDescLen] + "..."
		}

		if desc != "" {
			sb.WriteString(fmt.Sprintf("- **%s**: %s", name, desc))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s**", name))
		}

		if s.Frontmatter.ArgumentHint != "" {
			sb.WriteString(fmt.Sprintf(" — Usage: `/%s %s`", name, s.Frontmatter.ArgumentHint))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// BuildUserCommandList generates the list of /commands for TUI display.
func BuildUserCommandList(skills []*Skill) string {
	var sb strings.Builder
	sb.WriteString("Available skill commands:\n\n")

	for _, s := range skills {
		if !s.IsUserInvocable() {
			continue
		}
		name := s.Name()
		desc := s.Description()
		if len(desc) > 80 {
			desc = desc[:80] + "..."
		}

		if desc != "" {
			sb.WriteString(fmt.Sprintf("  /%s — %s\n", name, desc))
		} else {
			sb.WriteString(fmt.Sprintf("  /%s\n", name))
		}
	}

	return sb.String()
}

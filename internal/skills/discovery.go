package skills

import (
	"os"
	"path/filepath"
)

// DiscoveryConfig controls where skills are discovered.
type DiscoveryConfig struct {
	// Directories to scan, in priority order (lower index = higher priority).
	// Each entry: {path, source label, priority}.
	dirs []discoveryDir
}

type discoveryDir struct {
	path     string
	source   string
	priority int
}

// DefaultDiscoveryConfig returns the standard discovery paths for NB Code.
// Priority order:
//  1. ~/.nbcode/skills/          (user personal)
//  2. .nbcode/skills/            (project, walk up to $HOME)
//  3. .agents/skills/            (Agent Skills standard, walk up to $HOME)
//  4. <git-root>/skills/         (project root convention)
func DefaultDiscoveryConfig(cwd string) *DiscoveryConfig {
	cfg := &DiscoveryConfig{}
	priority := 0

	// 1. User personal: ~/.nbcode/skills/
	home, err := os.UserHomeDir()
	if err == nil {
		cfg.dirs = append(cfg.dirs, discoveryDir{
			path:     filepath.Join(home, ".nbcode", "skills"),
			source:   "user",
			priority: priority,
		})
		priority++
	}

	// 2. Project: walk up from CWD checking .nbcode/skills/
	for _, dir := range walkUpToHome(cwd, filepath.Join(".nbcode", "skills")) {
		cfg.dirs = append(cfg.dirs, discoveryDir{
			path:     dir,
			source:   "project",
			priority: priority,
		})
		priority++
	}

	// 3. Agent Skills standard: walk up from CWD checking .agents/skills/
	for _, dir := range walkUpToHome(cwd, filepath.Join(".agents", "skills")) {
		cfg.dirs = append(cfg.dirs, discoveryDir{
			path:     dir,
			source:   "agents-standard",
			priority: priority,
		})
		priority++
	}

	// 4. Project root: <cwd>/skills/
	cfg.dirs = append(cfg.dirs, discoveryDir{
		path:     filepath.Join(cwd, "skills"),
		source:   "project-root",
		priority: priority,
	})

	return cfg
}

// Discover scans all configured directories and returns deduplicated skills.
// First-found wins on name collision (lower priority number = higher priority).
func Discover(cfg *DiscoveryConfig) ([]*Skill, error) {
	seen := make(map[string]bool)     // canonical path → already loaded
	byName := make(map[string]bool)   // skill name → already loaded
	var result []*Skill

	for _, dd := range cfg.dirs {
		skills, err := scanDir(dd.path, dd.source, dd.priority)
		if err != nil {
			// Skip directories that don't exist or can't be read
			continue
		}

		for _, s := range skills {
			// Dedup by canonical file path
			canonical, err := filepath.EvalSymlinks(s.FilePath)
			if err != nil {
				canonical = s.FilePath
			}
			if seen[canonical] {
				continue
			}

			// Dedup by name (first-found wins)
			name := s.Name()
			if byName[name] {
				continue
			}

			seen[canonical] = true
			byName[name] = true
			result = append(result, s)
		}
	}

	return result, nil
}

// scanDir reads a single skills directory and returns found skills.
func scanDir(dir string, source string, priority int) ([]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []*Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue
		}

		skill, err := Parse(skillPath)
		if err != nil {
			continue
		}

		skill.Source = source
		skill.Priority = priority
		skills = append(skills, skill)
	}

	return skills, nil
}

// walkUpToHome walks from cwd to $HOME, returning directories that exist.
func walkUpToHome(cwd string, subdir string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var dirs []string
	dir := cwd
	for {
		candidate := filepath.Join(dir, subdir)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			dirs = append(dirs, candidate)
		}

		parent := filepath.Dir(dir)
		if parent == dir || dir == home {
			break
		}
		dir = parent
	}

	return dirs
}

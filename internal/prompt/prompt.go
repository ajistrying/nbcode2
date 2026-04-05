package prompt

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/ajistrying/nbcode2/internal/git"
)

const basePrompt = `You are nbcode, a coding agent that helps developers with software engineering tasks.

You have access to tools for reading, writing, and editing files, searching codebases, executing shell commands, and searching the web. Use these tools to help the user accomplish their goals.

# Guidelines
- Read files before modifying them to understand existing code.
- Use edit_file for targeted changes instead of rewriting entire files with write_file.
- When executing shell commands, be mindful of destructive operations.
- Prefer search and list_files to explore unfamiliar codebases before making changes.
- Provide clear, concise explanations of what you're doing and why.
- If you're unsure about something, ask the user rather than guessing.
- Do not make changes beyond what was requested.

# Tool Output Limits
- File reads are limited to 2000 lines. Use offset/limit for larger files.
- Bash output is capped at 30,000 characters.
- Search results are limited to 100 matches.
`

// Build assembles the full system prompt from base instructions,
// project config (nbcode.md), and environment context.
func Build(workDir string) string {
	var parts []string

	parts = append(parts, basePrompt)

	// Environment context
	parts = append(parts, buildEnvContext(workDir))

	// Project config (nbcode.md)
	projectConfig := loadProjectConfig(workDir)
	if projectConfig != "" {
		parts = append(parts, fmt.Sprintf("# Project Instructions\n\n%s", projectConfig))
	}

	return strings.Join(parts, "\n\n")
}

func buildEnvContext(workDir string) string {
	var lines []string
	lines = append(lines, "# Environment")
	lines = append(lines, fmt.Sprintf("- Working directory: %s", workDir))
	lines = append(lines, fmt.Sprintf("- Platform: %s/%s", runtime.GOOS, runtime.GOARCH))

	if shell := os.Getenv("SHELL"); shell != "" {
		lines = append(lines, fmt.Sprintf("- Shell: %s", shell))
	}

	gitInfo := git.GetInfo(workDir)
	if gitInfo.IsRepo {
		lines = append(lines, fmt.Sprintf("- Git repo root: %s", gitInfo.Root))
		lines = append(lines, fmt.Sprintf("- Git branch: %s", gitInfo.Branch))
		lines = append(lines, fmt.Sprintf("- Git status: %s", gitInfo.Status))
	} else {
		lines = append(lines, "- Not a git repository")
	}

	return strings.Join(lines, "\n")
}

func loadProjectConfig(workDir string) string {
	configPath := git.FindProjectConfig(workDir)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

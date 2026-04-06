package skills

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/google/shlex"
)

// Parse reads a SKILL.md file and returns a parsed Skill.
func Parse(filePath string) (*Skill, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading skill file: %w", err)
	}

	var fm Frontmatter
	body, err := frontmatter.Parse(bytes.NewReader(data), &fm)
	if err != nil {
		// If frontmatter parsing fails, treat the whole file as body
		fm = Frontmatter{}
		body = data
	}

	dir := filePath
	if idx := strings.LastIndex(dir, "/"); idx >= 0 {
		dir = dir[:idx]
	}

	return &Skill{
		Frontmatter: fm,
		Body:        string(body),
		Dir:         dir,
		FilePath:    filePath,
	}, nil
}

// PrepareBody processes the skill body for injection into context:
//  1. Substitutes $ARGUMENTS, $0, $1, etc.
//  2. Substitutes ${CLAUDE_SKILL_DIR} with the skill's directory path
//  3. Executes !`command` and ```! blocks, replaces with output
func PrepareBody(skill *Skill, args string) (string, error) {
	body := skill.Body

	body = substituteArgs(body, args)
	body = substituteVars(body, skill)

	var err error
	body, err = executeShellBlocks(body, skill.Frontmatter.Shell)
	if err != nil {
		return "", fmt.Errorf("executing shell blocks: %w", err)
	}

	return body, nil
}

// substituteArgs replaces $ARGUMENTS, $ARGUMENTS[N], $N placeholders.
func substituteArgs(body string, rawArgs string) string {
	if rawArgs == "" {
		return body
	}

	tokens, err := shlex.Split(rawArgs)
	if err != nil {
		// Fallback: split on whitespace
		tokens = strings.Fields(rawArgs)
	}

	// Replace $ARGUMENTS[N] and $N (indexed)
	for i, token := range tokens {
		body = strings.ReplaceAll(body, fmt.Sprintf("$ARGUMENTS[%d]", i), token)
		body = strings.ReplaceAll(body, fmt.Sprintf("$%d", i), token)
	}

	// Replace $ARGUMENTS (full string)
	body = strings.ReplaceAll(body, "$ARGUMENTS", rawArgs)

	return body
}

// substituteVars replaces ${CLAUDE_SKILL_DIR} and similar variables.
func substituteVars(body string, skill *Skill) string {
	body = strings.ReplaceAll(body, "${CLAUDE_SKILL_DIR}", skill.Dir)
	body = strings.ReplaceAll(body, "${SKILL_DIR}", skill.Dir)
	return body
}

// Patterns for shell execution in skill markdown.
var (
	// Matches !`command` (inline shell)
	inlineShellPattern = regexp.MustCompile("!`([^`]+)`")

	// Matches ```! ... ``` (block shell)
	blockShellPattern = regexp.MustCompile("(?s)```!\n(.*?)```")
)

// executeShellBlocks finds shell execution patterns and replaces with output.
func executeShellBlocks(body string, shell string) (string, error) {
	if shell == "" {
		shell = "bash"
	}

	var lastErr error

	// Process block patterns first (```! ... ```)
	body = blockShellPattern.ReplaceAllStringFunc(body, func(match string) string {
		submatch := blockShellPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		cmd := strings.TrimSpace(submatch[1])
		output, err := runShellCommand(shell, cmd)
		if err != nil {
			lastErr = err
			return fmt.Sprintf("```\nError executing command: %s\n```", err)
		}
		return output
	})

	// Process inline patterns (!`command`)
	body = inlineShellPattern.ReplaceAllStringFunc(body, func(match string) string {
		submatch := inlineShellPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		cmd := strings.TrimSpace(submatch[1])
		output, err := runShellCommand(shell, cmd)
		if err != nil {
			lastErr = err
			return fmt.Sprintf("[Error: %s]", err)
		}
		return strings.TrimSpace(output)
	})

	return body, lastErr
}

// runShellCommand executes a command and returns its output.
func runShellCommand(shell string, command string) (string, error) {
	cmd := exec.Command(shell, "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command %q failed: %w", command, err)
	}
	return string(output), nil
}

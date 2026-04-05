package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Info holds git repository state for display and prompt assembly.
type Info struct {
	IsRepo  bool
	Root    string
	Branch  string
	Status  string // short status summary
}

// GetInfo returns git information for the given directory.
func GetInfo(dir string) Info {
	info := Info{}

	// Check if we're in a git repo
	root, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return info
	}
	info.IsRepo = true
	info.Root = strings.TrimSpace(root)

	// Get current branch
	branch, err := runGit(dir, "branch", "--show-current")
	if err == nil {
		info.Branch = strings.TrimSpace(branch)
	}
	if info.Branch == "" {
		// Detached HEAD — get short SHA
		sha, err := runGit(dir, "rev-parse", "--short", "HEAD")
		if err == nil {
			info.Branch = "detached:" + strings.TrimSpace(sha)
		}
	}

	// Get short status
	status, err := runGit(dir, "status", "--short")
	if err == nil {
		lines := strings.Split(strings.TrimSpace(status), "\n")
		if len(lines) == 1 && lines[0] == "" {
			info.Status = "clean"
		} else {
			info.Status = pluralize(len(lines), "file changed", "files changed")
		}
	}

	return info
}

// RepoRoot finds the git repo root for nbcode.md lookup.
// Returns empty string if not in a git repo.
func RepoRoot(dir string) string {
	root, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(root)
}

// FindProjectConfig looks for nbcode.md in the repo root.
func FindProjectConfig(dir string) string {
	root := RepoRoot(dir)
	if root == "" {
		// Not a git repo — check current dir
		return filepath.Join(dir, "nbcode.md")
	}
	return filepath.Join(root, "nbcode.md")
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

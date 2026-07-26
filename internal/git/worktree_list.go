package git

import (
	"path/filepath"
	"strings"
)

// Worktree is one entry from `git worktree list --porcelain`.
type Worktree struct {
	Path     string
	Branch   string // short branch name (refs/heads/ stripped); empty if detached
	Prunable bool   // the worktree's directory is missing on disk
}

// ListWorktrees parses `git worktree list --porcelain` for the repo at dir.
func ListWorktrees(dir string) ([]Worktree, error) {
	out, err := Run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(out), nil
}

// parseWorktreeList parses --porcelain output: entries are separated by
// blank lines, each starting with "worktree <path>", then a "HEAD <sha>"
// line, then either "branch refs/heads/<name>" or "detached", and
// optionally a "prunable <reason>" line if the directory is missing.
func parseWorktreeList(out string) []Worktree {
	var result []Worktree
	var cur Worktree

	flush := func() {
		if cur.Path != "" {
			result = append(result, cur)
		}
		cur = Worktree{}
	}

	for _, line := range Lines(out) {
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "worktree "):
			// git always reports this path with forward slashes, even on
			// Windows; normalize to the native separator so every
			// WorktreePath a caller sees is consistent regardless of
			// whether it came from here or from ComputeWorktreePath.
			cur.Path = filepath.FromSlash(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case strings.HasPrefix(line, "prunable"):
			cur.Prunable = true
		}
	}
	flush()
	return result
}

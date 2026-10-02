package git

import (
	"path/filepath"
	"strings"
)

// ComputeWorktreePath returns the sibling worktree path for an issue
// branch, per doc/architecture.md's worktree sibling convention:
//
//	worktreesRoot       = <mainRepoRoot's parent>/<basename>.worktrees
//	issue worktree path = worktreesRoot/<typeName>/<title>
//
// <basename> is mainRepoRoot's own basename, with one exception: a bare
// repo conventionally named "<name>.git" gets a sibling named
// "<name>.worktrees", not "<name>.git.worktrees" — purely a naming
// nicety for the common bare-repo naming convention (e.g. a GitHub
// mirror), not a correctness requirement.
//
// If worktreesDirOverride is non-empty (from a repo-local .pmt.yaml's
// worktrees_dir), it's resolved relative to mainRepoRoot instead of the
// default sibling convention.
func ComputeWorktreePath(mainRepoRoot, worktreesDirOverride, typeName, title string) string {
	return filepath.Join(WorktreesRoot(mainRepoRoot, worktreesDirOverride), typeName, title)
}

// WorktreesRoot is the directory that holds every issue worktree, per the
// convention documented on ComputeWorktreePath.
func WorktreesRoot(mainRepoRoot, worktreesDirOverride string) string {
	if worktreesDirOverride != "" {
		return filepath.Clean(filepath.Join(mainRepoRoot, worktreesDirOverride))
	}
	base := strings.TrimSuffix(filepath.Base(mainRepoRoot), ".git")
	return filepath.Join(filepath.Dir(mainRepoRoot), base+".worktrees")
}

// WorktreeIgnoredFiles lists files in the worktree that git ignores (e.g.
// local settings). They are invisible to `git status`, but `git worktree
// remove` deletes them, so removal code checks this first.
func WorktreeIgnoredFiles(worktreePath string) ([]string, error) {
	out, err := Run(worktreePath, "status", "--porcelain", "--ignored")
	if err != nil {
		return nil, err
	}
	var ignored []string
	for _, line := range Lines(out) {
		if strings.HasPrefix(line, "!! ") {
			ignored = append(ignored, strings.TrimPrefix(line, "!! "))
		}
	}
	return ignored, nil
}

// WorktreeAdd creates a new linked worktree at path, checking out branch
// (which must already exist). Run in the main repo at dir.
func WorktreeAdd(dir, path, branch string) error {
	_, err := Run(dir, "worktree", "add", path, branch)
	return err
}

// IsWorktreeDirty reports whether worktreePath has uncommitted changes
// (`git status --porcelain` is non-empty). Run with dir = worktreePath.
func IsWorktreeDirty(worktreePath string) (bool, error) {
	out, err := Run(worktreePath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

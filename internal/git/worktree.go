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
	var worktreesRoot string
	if worktreesDirOverride != "" {
		worktreesRoot = filepath.Clean(filepath.Join(mainRepoRoot, worktreesDirOverride))
	} else {
		base := strings.TrimSuffix(filepath.Base(mainRepoRoot), ".git")
		worktreesRoot = filepath.Join(filepath.Dir(mainRepoRoot), base+".worktrees")
	}
	return filepath.Join(worktreesRoot, typeName, title)
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

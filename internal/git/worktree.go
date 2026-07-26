package git

import "path/filepath"

// ComputeWorktreePath returns the sibling worktree path for an issue
// branch, per doc/architecture.md's worktree sibling convention:
//
//	worktreesRoot       = <mainRepoRoot's parent>/<mainRepoRoot's basename>.worktrees
//	issue worktree path = worktreesRoot/<typeName>/<title>
//
// If worktreesDirOverride is non-empty (from a repo-local .pmt.yaml's
// worktrees_dir), it's resolved relative to mainRepoRoot instead of the
// default sibling convention.
func ComputeWorktreePath(mainRepoRoot, worktreesDirOverride, typeName, title string) string {
	var worktreesRoot string
	if worktreesDirOverride != "" {
		worktreesRoot = filepath.Clean(filepath.Join(mainRepoRoot, worktreesDirOverride))
	} else {
		worktreesRoot = filepath.Join(filepath.Dir(mainRepoRoot), filepath.Base(mainRepoRoot)+".worktrees")
	}
	return filepath.Join(worktreesRoot, typeName, title)
}

// WorktreeAdd creates a new linked worktree at path, checking out branch
// (which must already exist). Run in the main repo at dir.
func WorktreeAdd(dir, path, branch string) error {
	_, err := Run(dir, "worktree", "add", path, branch)
	return err
}

package git

import (
	"errors"
	"fmt"
	"path/filepath"
)

// ErrNotARepo is returned by Discover when cwd is not inside a git repository.
var ErrNotARepo = errors.New("not inside a git repository")

// RepoInfo is the result of resolving a git repository from a working
// directory, per doc/architecture.md's repo-resolution section.
type RepoInfo struct {
	GitDir       string // the .git dir (or file, if a linked worktree) in use
	GitCommonDir string // the main repo's real .git dir, shared by all worktrees
	IsBare       bool
}

// MainRoot is the canonical main-repo root, derived from GitCommonDir so
// pmt always operates relative to the main checkout regardless of which
// worktree it was invoked from — including one of pmt's own issue worktrees.
func (r RepoInfo) MainRoot() string {
	return filepath.Dir(r.GitCommonDir)
}

// InLinkedWorktree reports whether cwd resolved inside a linked worktree
// rather than the main checkout (GitDir and GitCommonDir diverge).
func (r RepoInfo) InLinkedWorktree() bool {
	return r.GitDir != r.GitCommonDir
}

// Discover resolves the git repository containing cwd via a single
// `rev-parse` call. A non-zero exit (not inside a repo) is reported as
// ErrNotARepo; any other execution failure is returned as-is.
//
// --show-toplevel is deliberately not requested: it hard-fails with
// "fatal: this operation must be run in a work tree" inside a bare repo,
// which would break bare-repo detection in the very call meant to catch it.
func Discover(cwd string) (RepoInfo, error) {
	out, code, err := RunRaw(cwd, "rev-parse",
		"--path-format=absolute",
		"--git-dir",
		"--git-common-dir",
		"--is-bare-repository",
	)
	if err != nil {
		return RepoInfo{}, err
	}
	if code != 0 {
		return RepoInfo{}, ErrNotARepo
	}

	lines := Lines(out)
	if len(lines) != 3 {
		return RepoInfo{}, fmt.Errorf("unexpected `git rev-parse` output: %q", out)
	}
	return RepoInfo{
		GitDir:       lines[0],
		GitCommonDir: lines[1],
		IsBare:       lines[2] == "true",
	}, nil
}

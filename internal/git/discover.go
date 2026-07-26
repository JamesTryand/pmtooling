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
	IsBare       bool   // whether the MAIN repo (GitCommonDir) is bare — see Discover's doc
}

// MainRoot is the canonical main-repo root, derived from GitCommonDir so
// pmt always operates relative to the main checkout regardless of which
// worktree it was invoked from — including one of pmt's own issue
// worktrees. For a bare repo, GitCommonDir already *is* the repo root
// (verified empirically: git-dir == git-common-dir == the bare path
// itself), so filepath.Dir would incorrectly return its parent.
func (r RepoInfo) MainRoot() string {
	if r.IsBare {
		return r.GitCommonDir
	}
	return filepath.Dir(r.GitCommonDir)
}

// InLinkedWorktree reports whether cwd resolved inside a linked worktree
// rather than the main checkout (GitDir and GitCommonDir diverge).
func (r RepoInfo) InLinkedWorktree() bool {
	return r.GitDir != r.GitCommonDir
}

// Discover resolves the git repository containing cwd via a single
// `rev-parse` call (plus, only when invoked from inside a linked
// worktree, one small follow-up — see below). A non-zero exit (not
// inside a repo) is reported as ErrNotARepo; any other execution failure
// is returned as-is.
//
// --show-toplevel is deliberately not requested: it hard-fails with
// "fatal: this operation must be run in a work tree" inside a bare repo,
// which would break bare-repo detection in the very call meant to catch it.
//
// IsBare deliberately reports whether the MAIN repo (GitCommonDir) is
// bare, not whether cwd's own immediate location is bare. Those differ
// when invoked from inside a linked worktree of a bare repo: a linked
// worktree always has a real working tree, so `--is-bare-repository`
// reports false there even though the repo it belongs to is bare —
// verified empirically. In that one case, bareness is re-checked by
// running `rev-parse --is-bare-repository` with dir = GitCommonDir
// itself, which correctly reports true for a bare repo's own path and
// false for a normal repo's .git directory (git recognizes the latter
// belongs to a parent working tree even when invoked with cwd pointed
// directly at it — also verified empirically, not assumed).
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
	gitDir, gitCommonDir := lines[0], lines[1]
	isBare := lines[2] == "true"

	if gitDir != gitCommonDir {
		isBare, err = isBareAt(gitCommonDir)
		if err != nil {
			return RepoInfo{}, err
		}
	}

	return RepoInfo{
		GitDir:       gitDir,
		GitCommonDir: gitCommonDir,
		IsBare:       isBare,
	}, nil
}

func isBareAt(dir string) (bool, error) {
	out, err := Run(dir, "rev-parse", "--is-bare-repository")
	if err != nil {
		return false, err
	}
	return out == "true", nil
}

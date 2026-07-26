package archive

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
)

// ErrDirtyWorktree is returned when the issue's worktree has uncommitted
// changes — refused rather than silently discarding them.
var ErrDirtyWorktree = errors.New("worktree has uncommitted changes; commit or discard them before closing")

// CloseResult is what Close returns on success.
type CloseResult struct {
	Branch          string
	ArchiveCommit   string
	WorktreeRemoved bool
}

// Close implements `pmt close <type>/<title>`: stamps the issue's
// README.md front matter with status: closed (+ a closed timestamp),
// merges its tree into the archive branch under <type>/<title>/, removes
// its worktree if one is registered, and deletes the branch.
//
// The issue's worktree may be in one of three states, all handled:
// present and clean (stamp via the worktree, then remove it), registered
// but prunable — its directory was deleted without `git worktree remove`
// (stamp via plumbing directly on the branch, since there's no
// directory to write into; `git worktree remove` still cleans up the
// stale registration so the branch can be deleted), or never registered
// at all — a hand-created branch with no worktree (stamp via plumbing,
// skip worktree removal entirely). A present-and-dirty worktree is
// refused, not force-cleaned.
func Close(mainRepoRoot string, repoCfg config.RepoConfig, typeName, title string) (CloseResult, error) {
	branch := typeName + "/" + title
	refName := "refs/heads/" + branch

	exists, err := git.RefExists(mainRepoRoot, refName)
	if err != nil {
		return CloseResult{}, err
	}
	if !exists {
		return CloseResult{}, fmt.Errorf("issue branch %q does not exist", branch)
	}

	registeredPath, isRegistered, err := registeredWorktreePath(mainRepoRoot, branch)
	if err != nil {
		return CloseResult{}, err
	}
	worktreeOnDisk := isRegistered && dirExists(registeredPath)

	if worktreeOnDisk {
		dirty, err := git.IsWorktreeDirty(registeredPath)
		if err != nil {
			return CloseResult{}, err
		}
		if dirty {
			return CloseResult{}, ErrDirtyWorktree
		}
	}

	closedAt := time.Now().UTC().Format(time.RFC3339)
	mutate := func(meta *issue.Meta) {
		meta.Status = "closed"
		meta.Closed = closedAt
	}

	message := fmt.Sprintf("pmt: close issue %s", branch)
	var stampedTip string
	if worktreeOnDisk {
		stampedTip, err = issue.StampReadmeInWorktree(registeredPath, message, mutate)
	} else {
		stampedTip, err = issue.StampReadmeViaPlumbing(mainRepoRoot, refName, message, mutate)
	}
	if err != nil {
		return CloseResult{}, err
	}

	archiveCommit, err := archiveIssue(mainRepoRoot, typeName, title, stampedTip)
	if err != nil {
		return CloseResult{}, err
	}

	if isRegistered {
		if _, err := git.Run(mainRepoRoot, "worktree", "remove", registeredPath); err != nil {
			return CloseResult{}, err
		}
	}
	if _, err := git.Run(mainRepoRoot, "branch", "-D", branch); err != nil {
		return CloseResult{}, err
	}

	return CloseResult{Branch: branch, ArchiveCommit: archiveCommit, WorktreeRemoved: isRegistered}, nil
}

func registeredWorktreePath(mainRepoRoot, branch string) (path string, ok bool, err error) {
	worktrees, err := git.ListWorktrees(mainRepoRoot)
	if err != nil {
		return "", false, err
	}
	for _, w := range worktrees {
		if w.Branch == branch {
			return w.Path, true, nil
		}
	}
	return "", false, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

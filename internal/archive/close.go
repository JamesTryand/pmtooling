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

// ErrArchiveWorktreeDirty is returned when a worktree has pmt/archive
// checked out and holds uncommitted changes: closing would move the branch
// under it, so it is refused rather than leaving that worktree inconsistent.
var ErrArchiveWorktreeDirty = errors.New("the pmt/archive worktree has uncommitted changes; commit or discard them before closing")

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

	// Closing moves refs/heads/pmt/archive. If a worktree has that branch
	// checked out, its index and files would be left at the old tip, showing
	// the newly archived issue as staged deletions. Refuse up front if that
	// worktree has uncommitted work (before anything is stamped or moved);
	// otherwise it is brought level after the ref moves.
	archiveWT, archiveWTPresent, err := archiveWorktree(mainRepoRoot)
	if err != nil {
		return CloseResult{}, err
	}
	if archiveWTPresent {
		dirty, err := git.IsWorktreeDirty(archiveWT)
		if err != nil {
			return CloseResult{}, err
		}
		if dirty {
			return CloseResult{}, fmt.Errorf("%w: %s", ErrArchiveWorktreeDirty, archiveWT)
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
	if archiveWTPresent {
		// The branch ref now points at archiveCommit but this worktree's
		// index and files are still at the previous tip. It was verified
		// clean above, so resetting to HEAD (the new tip) loses nothing.
		if _, err := git.Run(archiveWT, "reset", "--hard", "-q", "HEAD"); err != nil {
			return CloseResult{}, fmt.Errorf("archive updated (%s) but its checked-out worktree %s could not be brought level: %w", archiveCommit, archiveWT, err)
		}
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

// archiveWorktree returns the path of the worktree that has pmt/archive
// checked out, if one exists on disk. A registered worktree whose directory
// is missing is ignored: there is nothing on disk to keep consistent.
func archiveWorktree(mainRepoRoot string) (path string, ok bool, err error) {
	path, registered, err := registeredWorktreePath(mainRepoRoot, "pmt/archive")
	if err != nil || !registered || !dirExists(path) {
		return "", false, err
	}
	return path, true, nil
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

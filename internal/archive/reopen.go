package archive

import (
	"fmt"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
)

// ReopenResult is what Reopen returns on success.
type ReopenResult struct {
	Branch       string
	WorktreePath string
}

// Reopen implements `pmt reopen <type>/<title>`: finds the most recent
// archive entry for that name (findArchiveCommit), recreates the branch
// at its exact original tip — full pre-close history intact, unmodified
// — checks out a worktree, and restamps the README front matter back to
// status: open. The archive entry itself is left untouched; see the
// package doc for why the archive is append-only.
func Reopen(mainRepoRoot string, repoCfg config.RepoConfig, typeName, title string) (ReopenResult, error) {
	branch := typeName + "/" + title
	refName := "refs/heads/" + branch

	exists, err := git.RefExists(mainRepoRoot, refName)
	if err != nil {
		return ReopenResult{}, err
	}
	if exists {
		return ReopenResult{}, fmt.Errorf("branch %q already exists; cannot reopen over a live branch", branch)
	}

	archiveCommit, err := findArchiveCommit(mainRepoRoot, typeName, title)
	if err != nil {
		return ReopenResult{}, err
	}
	originalTip, err := git.Run(mainRepoRoot, "rev-parse", archiveCommit+"^1")
	if err != nil {
		return ReopenResult{}, err
	}

	worktreePath := git.ComputeWorktreePath(mainRepoRoot, repoCfg.WorktreesDir, typeName, title)
	if err := issue.CheckWorktreePathFree(worktreePath); err != nil {
		return ReopenResult{}, err
	}

	if _, err := git.Run(mainRepoRoot, "branch", branch, originalTip); err != nil {
		return ReopenResult{}, err
	}
	if err := git.WorktreeAdd(mainRepoRoot, worktreePath, branch); err != nil {
		return ReopenResult{}, err
	}

	_, err = issue.StampReadmeInWorktree(worktreePath, fmt.Sprintf("pmt: reopen issue %s", branch), func(meta *issue.Meta) {
		meta.Status = "open"
		meta.Closed = ""
	})
	if err != nil {
		return ReopenResult{}, err
	}

	return ReopenResult{Branch: branch, WorktreePath: worktreePath}, nil
}

package archive

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
)

// PruneOutcome is what Prune decided for one leftover local branch.
type PruneOutcome string

const (
	PruneDeleted PruneOutcome = "pruned"  // branch deleted (or would be, in a dry run)
	PruneRemoved PruneOutcome = "removed" // clean worktree removed, then branch deleted
	PruneInUse   PruneOutcome = "in-use"  // a worktree has the branch checked out; kept
	PruneDiffers PruneOutcome = "differs" // the tip holds something the archive lacks; kept
)

// PruneResult describes one archived issue whose local branch still exists.
type PruneResult struct {
	Branch  string
	Outcome PruneOutcome
	Tip     string   // full SHA of the branch tip when it was examined
	Detail  string   // worktree path (in-use) or differing paths (differs)
	Diffs   []string // paths in the tip that the archive lacks or holds differently
}

// PruneOptions tunes PruneWith.
type PruneOptions struct {
	DryRun bool
	// RemoveCleanWorktrees lets a branch that is checked out in a clean
	// worktree be pruned too: the worktree is removed first, then the
	// branch. Only after the same safety checks have passed; a worktree with
	// uncommitted or ignored files, or a missing directory, is still kept.
	RemoveCleanWorktrees bool
	// Filter, when set, restricts candidates to branches it returns true for.
	Filter func(branch string) bool
	// ArchiveRef is the ref holding the archive to judge against; empty
	// means the local refs/heads/pmt/archive. A dry-run sync points this at
	// the remote-tracking copy to preview what the real run will see.
	ArchiveRef string
}

// Prune finds local branches of issues that have been closed (they appear in
// the archive) and deletes each one only when ALL of these hold:
//
//  1. the issue is in the archive (the archive is pmt's record of "closed",
//     never the remote's `[gone]` state), and the branch has not been
//     reopened since (a reopened branch contains the archived tip);
//  2. no worktree has the branch checked out;
//  3. the branch tip holds nothing the archive lacks: every file in the tip's
//     tree exists, byte-identical, in the archived copy. The only tolerated
//     difference is the close stamp (`status`/`closed`) in README.md's
//     front matter. A real content comparison, not just the stamp.
//
// A branch that fails (2) or (3) is kept and reported. The deletion is
// guarded by the tip it examined, so a branch that moved in the meantime is
// not deleted. Nothing is deleted when dryRun is set.
func Prune(dir string, dryRun bool) ([]PruneResult, error) {
	return PruneWith(dir, PruneOptions{DryRun: dryRun})
}

// PruneWith is Prune with options; see PruneOptions.
func PruneWith(dir string, opts PruneOptions) ([]PruneResult, error) {
	archiveRef := opts.ArchiveRef
	if archiveRef == "" {
		archiveRef = Ref
	}
	archived, err := ListArchivedAt(dir, archiveRef, "")
	if err != nil {
		return nil, err
	}
	if len(archived) == 0 {
		return nil, nil
	}
	archivedSet := make(map[string]bool, len(archived))
	for _, a := range archived {
		archivedSet[a.Branch] = true
	}

	branches, err := git.ForEachRef(dir, "refs/heads", "%(refname:short)")
	if err != nil {
		return nil, err
	}
	worktrees, err := git.ListWorktrees(dir)
	if err != nil {
		return nil, err
	}
	inUse := map[string]git.Worktree{}
	for _, w := range worktrees {
		if w.Branch != "" {
			inUse[w.Branch] = w
		}
	}

	var results []PruneResult
	for _, branch := range branches {
		if !archivedSet[branch] {
			continue // an open issue (or not an issue at all): never a prune candidate
		}
		if opts.Filter != nil && !opts.Filter(branch) {
			continue
		}
		tip, err := git.RevParse(dir, "refs/heads/"+branch)
		if err != nil {
			return nil, err
		}

		reopened, err := ReopenedSinceArchiveAt(dir, archiveRef, branch, tip)
		if err != nil {
			return nil, err
		}
		if reopened {
			continue // it was closed and then reopened: an open issue with a stale archive entry
		}

		res := PruneResult{Branch: branch, Tip: tip}
		wt, used := inUse[branch]
		if used && !opts.RemoveCleanWorktrees {
			res.Outcome, res.Detail = PruneInUse, wt.Path
			results = append(results, res)
			continue
		}

		diffs, err := diffAgainstArchive(dir, archiveRef, tip, branch)
		if err != nil {
			return nil, err
		}
		if len(diffs) > 0 {
			res.Outcome, res.Diffs = PruneDiffers, diffs
			res.Detail = summarizeDiffs(diffs)
			results = append(results, res)
			continue
		}

		if used {
			blocker, err := worktreeBlocker(wt)
			if err != nil {
				return nil, err
			}
			if blocker != "" {
				res.Outcome, res.Detail = PruneInUse, blocker
				results = append(results, res)
				continue
			}
		}

		res.Outcome = PruneDeleted
		res.Detail = fmt.Sprintf("tip %s (restore: git branch %s %s)", git.ShortSHA(tip), branch, tip)
		if used {
			res.Outcome = PruneRemoved
			res.Detail = fmt.Sprintf("worktree %s and branch; tip %s (restore: git branch %s %s, then git worktree add)", wt.Path, git.ShortSHA(tip), branch, tip)
		}
		if !opts.DryRun {
			if used {
				if _, err := git.Run(dir, "worktree", "remove", wt.Path); err != nil {
					return nil, fmt.Errorf("removing worktree of %s: %w", branch, err)
				}
			}
			// the old-value argument makes this a failure, not a deletion, if the branch moved
			if _, err := git.Run(dir, "update-ref", "-d", "refs/heads/"+branch, tip); err != nil {
				return nil, fmt.Errorf("deleting %s: %w", branch, err)
			}
		}
		results = append(results, res)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Branch < results[j].Branch })
	return results, nil
}

// worktreeBlocker returns why a worktree must not be removed ("" when it is
// safe): a missing directory, uncommitted changes, or ignored files (which
// `git worktree remove` would delete without a word).
func worktreeBlocker(wt git.Worktree) (string, error) {
	if wt.Prunable || !dirExists(wt.Path) {
		return "worktree directory is missing: " + wt.Path + " (run git worktree prune)", nil
	}
	dirty, err := git.IsWorktreeDirty(wt.Path)
	if err != nil {
		return "", err
	}
	if dirty {
		return "uncommitted changes in " + wt.Path, nil
	}
	ignored, err := git.WorktreeIgnoredFiles(wt.Path)
	if err != nil {
		return "", err
	}
	if len(ignored) > 0 {
		return fmt.Sprintf("ignored files in %s would be lost (%s)", wt.Path, summarizeDiffs(ignored)), nil
	}
	return "", nil
}

// ReopenedSinceArchive reports whether tip (a commit of branch, local or
// remote) descends from the tip that was archived for it, which is exactly
// what a `pmt reopen` produces (the branch is recreated at the archived tip
// and a restamp commit added). Such a branch is an open issue whose archive
// entry is simply stale, never a leftover. A leftover on another machine
// sits at (or before) the archived tip instead. Returns false if the branch
// was never archived.
func ReopenedSinceArchive(dir, branch, tip string) (bool, error) {
	return ReopenedSinceArchiveAt(dir, Ref, branch, tip)
}

// ReopenedSinceArchiveAt is ReopenedSinceArchive judged against the archive
// at ref.
func ReopenedSinceArchiveAt(dir, ref, branch, tip string) (bool, error) {
	typeName, title := issue.Split(branch)
	commit, err := findArchiveCommitAt(dir, ref, typeName, title)
	if err != nil {
		if err == ErrNotArchived {
			return false, nil
		}
		return false, err
	}
	archivedTip, err := git.Run(dir, "rev-parse", commit+"^1")
	if err != nil {
		return false, err
	}
	if archivedTip == tip {
		return false, nil
	}
	return git.IsAncestor(dir, archivedTip, tip)
}

// diffAgainstArchive returns the paths in tip's tree that the archived copy
// of branch does not hold identically. Files only the archive has are fine
// (the tip is a subset). README.md is compared ignoring the close stamp.
func diffAgainstArchive(dir, archiveRef, tip, branch string) ([]string, error) {
	archivedTree, ok, err := git.TreeEntrySHA(dir, archiveRef, branch)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []string{"(archived copy not found)"}, nil
	}
	tipFiles, err := git.LsTreeFiles(dir, tip+"^{tree}")
	if err != nil {
		return nil, err
	}
	archFiles, err := git.LsTreeFiles(dir, archivedTree)
	if err != nil {
		return nil, err
	}

	var diffs []string
	for path, sha := range tipFiles {
		archSHA, present := archFiles[path]
		switch {
		case !present:
			diffs = append(diffs, path+" (not in archive)")
		case archSHA == sha:
			// identical
		case path == "README.md":
			same, err := sameExceptCloseStamp(dir, sha, archSHA)
			if err != nil {
				return nil, err
			}
			if !same {
				diffs = append(diffs, path+" (differs beyond the close stamp)")
			}
		default:
			diffs = append(diffs, path+" (differs)")
		}
	}
	sort.Strings(diffs)
	return diffs, nil
}

// sameExceptCloseStamp reports whether two README blobs are equal once
// `status` and `closed` are ignored in their front matter. If either has no
// parseable front matter, only byte equality counts.
func sameExceptCloseStamp(dir, shaA, shaB string) (bool, error) {
	a, err := git.ReadBlob(dir, shaA)
	if err != nil {
		return false, err
	}
	b, err := git.ReadBlob(dir, shaB)
	if err != nil {
		return false, err
	}
	metaA, bodyA, okA := issue.Parse(a)
	metaB, bodyB, okB := issue.Parse(b)
	if !okA || !okB {
		return string(a) == string(b), nil
	}
	metaA.Status, metaA.Closed = "", ""
	metaB.Status, metaB.Closed = "", ""
	return metaA == metaB && bodyA == bodyB, nil
}

func summarizeDiffs(diffs []string) string {
	const max = 5
	shown := diffs
	if len(shown) > max {
		shown = shown[:max]
	}
	s := strings.Join(shown, "; ")
	if len(diffs) > max {
		s += fmt.Sprintf("; and %d more", len(diffs)-max)
	}
	return s
}

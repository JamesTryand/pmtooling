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

// Prune finds local branches of issues that have been closed (they appear in
// the archive) and deletes each one only when ALL of these hold:
//
//  1. the issue is in the archive (the archive is pmt's record of "closed",
//     never the remote's `[gone]` state);
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
	archived, err := ListArchived(dir, "")
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
	inUse := map[string]string{}
	for _, w := range worktrees {
		if w.Branch != "" {
			inUse[w.Branch] = w.Path
		}
	}

	var results []PruneResult
	for _, branch := range branches {
		if !archivedSet[branch] {
			continue // an open issue (or not an issue at all): never a prune candidate
		}
		tip, err := git.RevParse(dir, "refs/heads/"+branch)
		if err != nil {
			return nil, err
		}
		res := PruneResult{Branch: branch, Tip: tip}

		if path, used := inUse[branch]; used {
			res.Outcome, res.Detail = PruneInUse, path
			results = append(results, res)
			continue
		}

		diffs, err := diffAgainstArchive(dir, tip, branch)
		if err != nil {
			return nil, err
		}
		if len(diffs) > 0 {
			res.Outcome, res.Diffs = PruneDiffers, diffs
			res.Detail = summarizeDiffs(diffs)
			results = append(results, res)
			continue
		}

		res.Outcome = PruneDeleted
		res.Detail = fmt.Sprintf("tip %s (restore: git branch %s %s)", git.ShortSHA(tip), branch, tip)
		if !dryRun {
			// the old-value argument makes this a no-op failure if the branch moved
			if _, err := git.Run(dir, "update-ref", "-d", "refs/heads/"+branch, tip); err != nil {
				return nil, fmt.Errorf("deleting %s: %w", branch, err)
			}
		}
		results = append(results, res)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Branch < results[j].Branch })
	return results, nil
}

// diffAgainstArchive returns the paths in tip's tree that the archived copy
// of branch does not hold identically. Files only the archive has are fine
// (the tip is a subset). README.md is compared ignoring the close stamp.
func diffAgainstArchive(dir, tip, branch string) ([]string, error) {
	archivedTree, ok, err := git.TreeEntrySHA(dir, Ref, branch)
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

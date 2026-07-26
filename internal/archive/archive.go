// Package archive implements pmt's issue archive: `pmt close` merges a
// closed issue's tree into a dedicated refs/heads/pmt/archive branch
// under <type>/<title>/, and `pmt reopen` restores it. The archive is
// append-only in spirit — reopening never removes anything from it, so
// every commit on pmt/archive keeps the same simple shape (an issue tip
// plus, after the first, the previous archive tip as a second parent),
// which is what lets reopen find the right history purely from git
// structure with no separate manifest. See task_plan.md's Phase 7b
// design notes for the full rationale, including the two designs (a
// commit-parent-position convention, and a .pmt-archive.yaml metadata
// file) that were tried and rejected before this one.
package archive

import (
	"errors"
	"fmt"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// Ref is the archive branch's full ref name.
const Ref = "refs/heads/pmt/archive"

// ErrNotArchived is returned when no archived entry exists for a given
// <type>/<title>.
var ErrNotArchived = errors.New("no archived issue found with that name")

// archiveIssue merges issueTip's tree into the archive branch under
// path "<typeName>/<title>/", creating refs/heads/pmt/archive if it
// doesn't exist yet. If that path was already archived before (a prior
// close, before a reopen), its entry is replaced with the new content;
// every sibling entry — every other archived issue, and any earlier
// content for a *different* issue — is preserved unchanged by SHA
// reference (ls-tree + mktree tree surgery, not read-tree --prefix,
// which can only add a new path, not replace an existing one — verified
// empirically). Returns the new archive commit SHA.
func archiveIssue(dir, typeName, title, issueTip string) (string, error) {
	issueTree, err := git.Run(dir, "rev-parse", issueTip+"^{tree}")
	if err != nil {
		return "", err
	}

	prevTip, prevExists, err := git.RevParseQuiet(dir, Ref)
	if err != nil {
		return "", err
	}

	newTypeTree, err := replaceEntry(dir, prevTip, prevExists, typeName, title, issueTree)
	if err != nil {
		return "", err
	}
	newRootTree, err := replaceEntry(dir, prevTip, prevExists, "", typeName, newTypeTree)
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("archive: close %s/%s", typeName, title)
	var commit string
	if prevExists {
		commit, err = git.CommitTree(dir, newRootTree, message, issueTip, prevTip)
	} else {
		commit, err = git.CommitTree(dir, newRootTree, message, issueTip)
	}
	if err != nil {
		return "", err
	}
	if err := git.UpdateRef(dir, Ref, commit); err != nil {
		return "", err
	}
	return commit, nil
}

// replaceEntry returns a new tree SHA equal to prevTip's tree at
// prefixPath (or an empty tree if prevTip doesn't exist or has no such
// path), with its entry named name replaced (or added) to point at
// newEntrySHA — every other entry is kept unchanged by SHA reference.
// prefixPath == "" means the tree root itself.
func replaceEntry(dir, prevTip string, prevExists bool, prefixPath, name, newEntrySHA string) (string, error) {
	var entries []git.TreeEntry
	if prevExists {
		treeish := prevTip
		if prefixPath != "" {
			treeish = prevTip + ":" + prefixPath
		}
		if sha, ok, err := git.RevParseQuiet(dir, treeish); err != nil {
			return "", err
		} else if ok {
			existing, err := git.LsTree(dir, sha)
			if err != nil {
				return "", err
			}
			for _, e := range existing {
				if e.Name != name {
					entries = append(entries, e)
				}
			}
		}
	}
	entries = append(entries, git.TreeEntry{Mode: "040000", Type: "tree", SHA: newEntrySHA, Name: name})
	return git.Mktree(dir, entries)
}

// findArchiveCommit walks refs/heads/pmt/archive backward via each
// commit's second parent, looking for the most recent commit whose tree
// entry at "<typeName>/<title>" differs from (or is newly present
// versus) its predecessor's — found by comparing tree content, not just
// path existence, so a re-close after a reopen is found correctly
// instead of the stale first-ever archive. Returns ErrNotArchived if the
// path was never archived, or the archive branch doesn't exist.
func findArchiveCommit(dir, typeName, title string) (string, error) {
	path := typeName + "/" + title

	current, ok, err := git.RevParseQuiet(dir, Ref)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotArchived
	}

	for current != "" {
		curSHA, curOK, err := git.TreeEntrySHA(dir, current, path)
		if err != nil {
			return "", err
		}
		if !curOK {
			return "", ErrNotArchived
		}

		parent2, has2, err := git.RevParseQuiet(dir, current+"^2")
		if err != nil {
			return "", err
		}
		var prevSHA string
		if has2 {
			prevSHA, _, err = git.TreeEntrySHA(dir, parent2, path)
			if err != nil {
				return "", err
			}
		}

		if curSHA != prevSHA {
			return current, nil
		}
		current = parent2
	}
	return "", ErrNotArchived
}

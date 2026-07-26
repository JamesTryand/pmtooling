package issue

import (
	"os"
	"sort"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/template"
)

// WorktreeState describes the relationship between an issue branch and
// its expected worktree, per doc/edge-cases.md.
type WorktreeState string

const (
	WorktreeOK       WorktreeState = "ok"       // registered and present on disk
	WorktreePrunable WorktreeState = "prunable" // registered, but its directory is missing
	WorktreeOrphaned WorktreeState = "orphaned" // a directory exists at the expected path, but isn't registered
	WorktreeMissing  WorktreeState = "missing"  // no worktree registered, no directory found either
)

// Issue is one row of `pmt list`.
type Issue struct {
	Branch        string        `json:"branch"`
	Status        string        `json:"status"`
	Created       string        `json:"created"`
	Unparseable   bool          `json:"unparseable"`
	WorktreePath  string        `json:"worktree_path,omitempty"`
	WorktreeState WorktreeState `json:"worktree_state"`
}

// ListIssues implements `pmt list`'s core logic (doc/commands.md#pmt-list):
// it scans every ref under refs/heads/, keeps only branches whose first
// '/'-segment matches an existing template (excluding unrelated branches,
// e.g. release/1.0, and the template branches themselves, from ever
// appearing as an issue), optionally filtered by typeFilter, cross-
// references them with `git worktree list --porcelain`, and reads each
// issue's metadata via `git show <branch>:README.md` — which works even
// when no worktree is checked out, since there's no central manifest
// (doc/architecture.md). A branch whose README.md is missing entirely or
// fails to parse is reported non-fatally via Unparseable, never dropped
// or allowed to abort the whole listing.
func ListIssues(mainRepoRoot, worktreesDirOverride, typeFilter string) ([]Issue, error) {
	refs, err := git.ForEachRef(mainRepoRoot, "refs/heads/", "%(refname:short)")
	if err != nil {
		return nil, err
	}

	templates, err := template.List(mainRepoRoot)
	if err != nil {
		return nil, err
	}
	templateSet := make(map[string]bool, len(templates))
	for _, name := range templates {
		templateSet[name] = true
	}

	worktrees, err := git.ListWorktrees(mainRepoRoot)
	if err != nil {
		return nil, err
	}
	worktreeByBranch := make(map[string]git.Worktree, len(worktrees))
	for _, w := range worktrees {
		if w.Branch != "" {
			worktreeByBranch[w.Branch] = w
		}
	}

	var issues []Issue
	for _, branch := range refs {
		typeName, title := Split(branch)
		if !templateSet[typeName] {
			continue // not a pmt-managed issue branch (includes template branches themselves: "pmt" never names a template)
		}
		if typeFilter != "" && typeName != typeFilter {
			continue
		}

		iss := Issue{Branch: branch}

		content, showErr := git.Run(mainRepoRoot, "show", branch+":README.md")
		if showErr != nil {
			iss.Unparseable = true
		} else if meta, _, ok := Parse([]byte(content)); ok {
			iss.Status = meta.Status
			iss.Created = meta.Created
		} else {
			iss.Unparseable = true
		}

		if wt, ok := worktreeByBranch[branch]; ok {
			iss.WorktreePath = wt.Path
			if wt.Prunable {
				iss.WorktreeState = WorktreePrunable
			} else {
				iss.WorktreeState = WorktreeOK
			}
		} else {
			expected := git.ComputeWorktreePath(mainRepoRoot, worktreesDirOverride, typeName, title)
			if _, statErr := os.Stat(expected); statErr == nil {
				iss.WorktreePath = expected
				iss.WorktreeState = WorktreeOrphaned
			} else {
				iss.WorktreeState = WorktreeMissing
			}
		}

		issues = append(issues, iss)
	}

	sort.Slice(issues, func(i, j int) bool { return issues[i].Branch < issues[j].Branch })
	return issues, nil
}

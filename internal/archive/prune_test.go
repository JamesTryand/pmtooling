package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
)

// closedElsewhere simulates the other-machine case: the issue was closed
// (so it is in the archive and its branch is gone), but this machine still
// has a local branch at the given pre-close tip.
func closedElsewhere(t *testing.T, dir, title, tip string) {
	t.Helper()
	if _, err := Close(dir, defaultCfg(), "bug", title); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := git.Run(dir, "branch", "bug/"+title, tip); err != nil {
		t.Fatalf("recreating leftover branch: %v", err)
	}
}

func tipOf(t *testing.T, dir, branch string) string {
	t.Helper()
	sha, err := git.Run(dir, "rev-parse", "refs/heads/"+branch)
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func only(t *testing.T, rs []PruneResult) PruneResult {
	t.Helper()
	if len(rs) != 1 {
		t.Fatalf("results = %+v, want exactly one", rs)
	}
	return rs[0]
}

func TestPruneDeletesLeftoverThatDiffersOnlyByCloseStamp(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	tip := tipOf(t, dir, "bug/one")
	closedElsewhere(t, dir, "one", tip)

	rs, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	r := only(t, rs)
	if r.Outcome != PruneDeleted || r.Tip != tip {
		t.Fatalf("result = %+v, want pruned with the examined tip", r)
	}
	if !strings.Contains(r.Detail, tip) || !strings.Contains(r.Detail, "git branch bug/one") {
		t.Errorf("detail should print the tip and a restore command: %q", r.Detail)
	}
	if ok, _ := git.RefExists(dir, "refs/heads/bug/one"); ok {
		t.Error("branch should be deleted")
	}
	// recoverable from the printed tip
	if _, err := git.Run(dir, "branch", "bug/restored", tip); err != nil {
		t.Errorf("could not restore from the printed tip: %v", err)
	}
}

func TestPruneLeavesOpenIssuesAndNonIssuesAloneWhenNothingIsArchived(t *testing.T) {
	dir, _ := repoWithIssue(t, "open-one")
	if _, err := git.Run(dir, "branch", "release/1.0"); err != nil {
		t.Fatal(err)
	}

	rs, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 0 {
		t.Errorf("results = %+v, want none (nothing is archived)", rs)
	}
	for _, b := range []string{"bug/open-one", "release/1.0"} {
		if ok, _ := git.RefExists(dir, "refs/heads/"+b); !ok {
			t.Errorf("%s must survive", b)
		}
	}
}

func TestPruneKeepsOpenIssueWhenAnotherIsArchived(t *testing.T) {
	dir, _ := repoWithIssue(t, "closed-one")
	if _, err := issue.Create(dir, defaultCfg(), "bug", "still-open"); err != nil {
		t.Fatal(err)
	}
	if _, err := Close(dir, defaultCfg(), "bug", "closed-one"); err != nil {
		t.Fatal(err)
	}

	rs, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 0 {
		t.Errorf("results = %+v, want none: the open issue is not archived", rs)
	}
	if ok, _ := git.RefExists(dir, "refs/heads/bug/still-open"); !ok {
		t.Error("an open issue's branch was deleted")
	}
}

func TestPruneKeepsBranchStillCheckedOut(t *testing.T) {
	dir, result := repoWithIssue(t, "one")
	tip := tipOf(t, dir, "bug/one")
	if _, err := Close(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "branch", "bug/one", tip); err != nil {
		t.Fatal(err)
	}
	if err := git.WorktreeAdd(dir, result.WorktreePath, "bug/one"); err != nil {
		t.Fatal(err)
	}

	rs, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	r := only(t, rs)
	if r.Outcome != PruneInUse || r.Detail == "" {
		t.Fatalf("result = %+v, want in-use naming the worktree", r)
	}
	if tipOf(t, dir, "bug/one") != tip {
		t.Error("an in-use branch moved")
	}
}

func TestPruneKeepsBranchWithWorkTheArchiveLacks(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	// extra work on the branch that is NOT in what gets archived: archive
	// the issue first, then keep committing on a recreated branch
	if _, err := Close(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatal(err)
	}
	// recreate the branch from the archive copy of the issue, then add work
	archiveTip, _ := git.Run(dir, "rev-parse", Ref)
	original, err := git.Run(dir, "rev-parse", archiveTip+"^1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "branch", "bug/one", original); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(t.TempDir(), "w")
	if err := git.WorktreeAdd(dir, wt, "bug/one"); err != nil {
		t.Fatal(err)
	}
	addCommitInWorktree(t, wt, "later-work.txt", "important", "work after close")
	if _, err := git.Run(dir, "worktree", "remove", wt); err != nil {
		t.Fatal(err)
	}
	tip := tipOf(t, dir, "bug/one")

	rs, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	r := only(t, rs)
	if r.Outcome != PruneDiffers || !strings.Contains(r.Detail, "later-work.txt (not in archive)") {
		t.Fatalf("result = %+v, want differs naming later-work.txt", r)
	}
	if tipOf(t, dir, "bug/one") != tip {
		t.Error("a branch with unarchived work was deleted or moved")
	}
}

func TestPruneKeepsBranchWhoseREADMEChangedBeyondTheStamp(t *testing.T) {
	dir, result := repoWithIssue(t, "one")
	readme := filepath.Join(result.WorktreePath, "README.md")
	body, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	// edit the README's body on the branch, committed, then close from that
	// state would archive the edit; so edit AFTER archiving instead
	tip := tipOf(t, dir, "bug/one")
	closedElsewhere(t, dir, "one", tip)

	wt := filepath.Join(t.TempDir(), "w")
	if err := git.WorktreeAdd(dir, wt, "bug/one"); err != nil {
		t.Fatal(err)
	}
	addCommitInWorktree(t, wt, "README.md", string(body)+"\nnew open item\n", "edit readme")
	if _, err := git.Run(dir, "worktree", "remove", wt); err != nil {
		t.Fatal(err)
	}

	rs, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	r := only(t, rs)
	if r.Outcome != PruneDiffers || !strings.Contains(r.Detail, "README.md (differs beyond the close stamp)") {
		t.Fatalf("result = %+v, want a README difference beyond the stamp", r)
	}
	if ok, _ := git.RefExists(dir, "refs/heads/bug/one"); !ok {
		t.Error("branch deleted despite a README change the archive lacks")
	}
}

func TestPruneDeletesWhenArchiveHasLaterWorkThanTheTip(t *testing.T) {
	dir, result := repoWithIssue(t, "one")
	oldTip := tipOf(t, dir, "bug/one")
	// later work lands on the issue (as on another machine), then it's closed
	addCommitInWorktree(t, result.WorktreePath, "extra.txt", "more", "later work")
	closedElsewhere(t, dir, "one", oldTip)

	rs, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if r := only(t, rs); r.Outcome != PruneDeleted {
		t.Fatalf("result = %+v, want pruned: the older tip is a subset of the archive", r)
	}
}

func TestPruneDryRunDeletesNothingAndMatchesRealRun(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	tip := tipOf(t, dir, "bug/one")
	closedElsewhere(t, dir, "one", tip)

	before, _ := git.Run(dir, "for-each-ref", "--format=%(refname) %(objectname)")
	dry, err := Prune(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := git.Run(dir, "for-each-ref", "--format=%(refname) %(objectname)")
	if before != after {
		t.Fatal("dry run changed refs")
	}

	real, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(dry) != 1 || len(real) != 1 ||
		dry[0].Branch != real[0].Branch || dry[0].Outcome != real[0].Outcome || dry[0].Detail != real[0].Detail {
		t.Errorf("dry run %+v differs from real run %+v", dry, real)
	}
}

func TestPruneWithNoArchiveDoesNothing(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	rs, err := Prune(dir, false)
	if err != nil || len(rs) != 0 {
		t.Errorf("Prune with no archive = %+v, %v; want nothing", rs, err)
	}
}

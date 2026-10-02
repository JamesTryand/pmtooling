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
	// The other machine did work after the pre-close tip; the machine that
	// closed the issue never saw it, so the archive lacks it and the two
	// histories diverged at the pre-close tip.
	preClose := tipOf(t, dir, "bug/one")
	if _, err := Close(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "branch", "bug/one", preClose); err != nil {
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

// A branch built on top of the archived tip is indistinguishable from a
// reopened issue with new work (that is exactly what reopen produces), so it
// is treated as open: never deleted, never reported as a leftover.
func TestPruneTreatsBranchBuiltOnArchivedTipAsReopened(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	if _, err := Close(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatal(err)
	}
	archiveTip, _ := git.Run(dir, "rev-parse", Ref)
	stamped, _ := git.Run(dir, "rev-parse", archiveTip+"^1")
	if _, err := git.Run(dir, "branch", "bug/one", stamped); err != nil {
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
	if len(rs) != 0 {
		t.Errorf("results = %+v, want none", rs)
	}
	if tipOf(t, dir, "bug/one") != tip {
		t.Error("work on a reopened-style branch was deleted or moved")
	}
}

// A reopened issue has a live branch AND a stale archive entry. It is an open
// issue, so prune must never treat it as a closed-elsewhere leftover, even
// with no worktree to protect it.
func TestPruneSkipsReopenedIssue(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	if _, err := Close(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatal(err)
	}
	res, err := Reopen(dir, defaultCfg(), "bug", "one")
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if _, err := git.Run(dir, "worktree", "remove", res.WorktreePath); err != nil {
		t.Fatal(err)
	}
	tip := tipOf(t, dir, "bug/one")

	rs, err := Prune(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 0 {
		t.Errorf("results = %+v, want none: a reopened issue is open", rs)
	}
	if tipOf(t, dir, "bug/one") != tip {
		t.Error("a reopened issue's branch was deleted or moved")
	}
}

// leftoverWithWorktree is closedElsewhere plus a worktree for the leftover.
func leftoverWithWorktree(t *testing.T, title string) (dir, wt string) {
	t.Helper()
	var result issue.Result
	dir, result = repoWithIssue(t, title)
	tip := tipOf(t, dir, "bug/"+title)
	closedElsewhere(t, dir, title, tip)
	if err := git.WorktreeAdd(dir, result.WorktreePath, "bug/"+title); err != nil {
		t.Fatal(err)
	}
	return dir, result.WorktreePath
}

func TestPruneWithRemoveCleanWorktrees(t *testing.T) {
	dir, wt := leftoverWithWorktree(t, "one")
	tip := tipOf(t, dir, "bug/one")

	// default Prune keeps it: the branch is checked out
	if rs, _ := Prune(dir, false); len(rs) != 1 || rs[0].Outcome != PruneInUse {
		t.Fatalf("default Prune = %+v, want in-use", rs)
	}

	// dry run reports removal and does nothing
	rs, err := PruneWith(dir, PruneOptions{DryRun: true, RemoveCleanWorktrees: true})
	if err != nil || len(rs) != 1 || rs[0].Outcome != PruneRemoved {
		t.Fatalf("dry run = %+v, %v; want removed", rs, err)
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("dry run removed the worktree: %v", err)
	}

	rs, err = PruneWith(dir, PruneOptions{RemoveCleanWorktrees: true})
	if err != nil {
		t.Fatal(err)
	}
	r := only(t, rs)
	if r.Outcome != PruneRemoved || !strings.Contains(r.Detail, tip) {
		t.Fatalf("result = %+v, want removed with the tip", r)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree should be gone, stat err = %v", err)
	}
	if ok, _ := git.RefExists(dir, "refs/heads/bug/one"); ok {
		t.Error("branch should be deleted")
	}
}

func TestPruneWithKeepsWorktreesThatCouldLoseData(t *testing.T) {
	opts := PruneOptions{RemoveCleanWorktrees: true}

	t.Run("uncommitted changes", func(t *testing.T) {
		dir, wt := leftoverWithWorktree(t, "one")
		if err := os.WriteFile(filepath.Join(wt, "scratch.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		r := only(t, mustPrune(t, dir, opts))
		if r.Outcome != PruneInUse || !strings.Contains(r.Detail, "uncommitted") {
			t.Errorf("result = %+v, want in-use (uncommitted)", r)
		}
		if _, err := os.Stat(filepath.Join(wt, "scratch.txt")); err != nil {
			t.Errorf("uncommitted file lost: %v", err)
		}
	})

	t.Run("ignored files", func(t *testing.T) {
		dir, wt := leftoverWithWorktree(t, "one")
		exclude, err := git.Run(dir, "rev-parse", "--git-path", "info/exclude")
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(exclude) {
			exclude = filepath.Join(dir, exclude)
		}
		if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(exclude, []byte("*.local\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wt, "settings.local"), []byte("mine"), 0o644); err != nil {
			t.Fatal(err)
		}
		r := only(t, mustPrune(t, dir, opts))
		if r.Outcome != PruneInUse || !strings.Contains(r.Detail, "ignored files") || !strings.Contains(r.Detail, "settings.local") {
			t.Errorf("result = %+v, want in-use naming the ignored file", r)
		}
		if got, err := os.ReadFile(filepath.Join(wt, "settings.local")); err != nil || string(got) != "mine" {
			t.Errorf("ignored file lost: %q, %v", got, err)
		}
	})

	t.Run("tip has unarchived work", func(t *testing.T) {
		dir, wt := leftoverWithWorktree(t, "one")
		addCommitInWorktree(t, wt, "extra.txt", "unarchived", "extra")
		r := only(t, mustPrune(t, dir, opts))
		if r.Outcome != PruneDiffers || !strings.Contains(r.Detail, "extra.txt") {
			t.Errorf("result = %+v, want differs naming extra.txt", r)
		}
		if _, err := os.Stat(wt); err != nil {
			t.Errorf("worktree removed despite unarchived work: %v", err)
		}
	})

	t.Run("filter excludes", func(t *testing.T) {
		dir, wt := leftoverWithWorktree(t, "one")
		rs := mustPrune(t, dir, PruneOptions{RemoveCleanWorktrees: true, Filter: func(string) bool { return false }})
		if len(rs) != 0 {
			t.Errorf("results = %+v, want none", rs)
		}
		if _, err := os.Stat(wt); err != nil {
			t.Errorf("worktree touched despite the filter: %v", err)
		}
	})
}

func mustPrune(t *testing.T, dir string, opts PruneOptions) []PruneResult {
	t.Helper()
	rs, err := PruneWith(dir, opts)
	if err != nil {
		t.Fatalf("PruneWith: %v", err)
	}
	return rs
}

func TestPruneWithNoArchiveDoesNothing(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	rs, err := Prune(dir, false)
	if err != nil || len(rs) != 0 {
		t.Errorf("Prune with no archive = %+v, %v; want nothing", rs, err)
	}
}

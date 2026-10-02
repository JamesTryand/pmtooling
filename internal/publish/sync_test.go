package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/archive"
	"github.com/JamesTryand/pmtooling/internal/git"
)

// publishAll pushes every managed branch from machine A.
func publishAll(t *testing.T, f fixture) {
	t.Helper()
	branches, err := ManagedBranches(f.root)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := Push(f.root, "origin", branches, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if r.Verb.NeedsAttention() {
			t.Fatalf("publishAll: %+v", r)
		}
	}
}

func syncOpts() SyncOptions { return SyncOptions{Remote: "origin"} }

func mustSync(t *testing.T, root string, opts SyncOptions) []Result {
	t.Helper()
	rs, err := Sync(root, defaultCfg(), opts)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	return rs
}

func verbOf(t *testing.T, rs []Result, branch string) Result {
	t.Helper()
	for _, r := range rs {
		if r.Branch == branch {
			return r
		}
	}
	t.Fatalf("no result for %q in %+v", branch, rs)
	return Result{}
}

func anyAttention(rs []Result) bool {
	for _, r := range rs {
		if r.Verb.NeedsAttention() {
			return true
		}
	}
	return false
}

// localState is everything a dry run must leave alone: local branches and
// the worktree list. (Remote-tracking refs may move: sync always fetches.)
func localState(t *testing.T, dir string) string {
	t.Helper()
	return mustRun(t, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads") +
		"\n" + mustRun(t, dir, "worktree", "list", "--porcelain")
}

func pairs(rs []Result) string {
	var b strings.Builder
	for _, r := range rs {
		b.WriteString(r.Branch + "=" + string(r.Verb) + ";")
	}
	return b.String()
}

func TestSyncCreatesWorktreesAndFastForwards(t *testing.T) {
	f := newFixture(t)
	br := f.newIssue(t, "one")
	publishAll(t, f)
	b := machineB(t, f)

	rs := mustSync(t, b, syncOpts())
	if verbOf(t, rs, "pmt/template/bug").Verb != VerbCreated || verbOf(t, rs, br).Verb != VerbCreated {
		t.Fatalf("first sync = %s, want template and issue created", pairs(rs))
	}
	if anyAttention(rs) {
		t.Errorf("a clean first sync needs no attention: %s", pairs(rs))
	}
	if _, err := os.Stat(filepath.Join(wtPathOn(b, br), "README.md")); err != nil {
		t.Errorf("worktree not created: %v", err)
	}

	// idempotent
	rs = mustSync(t, b, syncOpts())
	if verbOf(t, rs, br).Verb != VerbUpToDate || anyAttention(rs) {
		t.Errorf("second sync = %s, want everything up-to-date", pairs(rs))
	}

	// the remote moves; sync fast-forwards
	commitOnA(t, f, br, "later.txt")
	rs = mustSync(t, b, syncOpts())
	if verbOf(t, rs, br).Verb != VerbUpdated {
		t.Fatalf("sync after remote move = %s, want updated", pairs(rs))
	}
	if _, err := os.Stat(filepath.Join(wtPathOn(b, br), "later.txt")); err != nil {
		t.Errorf("fast-forward did not reach the worktree: %v", err)
	}
}

// The 2026-09-25 incident: two open issues that were never pushed must
// survive a sync untouched, however the remote looks.
func TestSyncNeverRemovesUnpublishedOpenIssues(t *testing.T) {
	f := newFixture(t)
	a := f.newIssue(t, "fresh-a")
	b := f.newIssue(t, "fresh-b")
	tipA := mustRun(t, f.root, "rev-parse", "refs/heads/"+a)
	tipB := mustRun(t, f.root, "rev-parse", "refs/heads/"+b)

	for _, opts := range []SyncOptions{{Remote: "origin", DryRun: true}, syncOpts()} {
		rs := mustSync(t, f.root, opts)
		for _, br := range []string{a, b} {
			r := verbOf(t, rs, br)
			if r.Verb != VerbUnpublished || !r.Verb.NeedsAttention() || !strings.Contains(r.Detail, "pmt push") {
				t.Errorf("%s = %+v, want unpublished pointing at pmt push (dry=%v)", br, r, opts.DryRun)
			}
			if _, err := os.Stat(wtPathOn(f.root, br)); err != nil {
				t.Errorf("%s worktree gone (dry=%v): %v", br, opts.DryRun, err)
			}
		}
	}
	if got := mustRun(t, f.root, "rev-parse", "refs/heads/"+a); got != tipA {
		t.Errorf("%s moved", a)
	}
	if got := mustRun(t, f.root, "rev-parse", "refs/heads/"+b); got != tipB {
		t.Errorf("%s moved", b)
	}
}

// closedElsewhere: machine A closes bug/one, pushes the archive and deletes
// the remote branch, as the notebook's flow does.
func closeOnA(t *testing.T, f fixture, title string) {
	t.Helper()
	if _, err := archive.Close(f.root, defaultCfg(), "bug", title); err != nil {
		t.Fatalf("Close on A: %v", err)
	}
	publishAll(t, f)
	mustRun(t, f.root, "push", "-q", "origin", "--delete", "bug/"+title)
}

func TestSyncRemovesIssueClosedElsewhereAndDryRunPredictsIt(t *testing.T) {
	f := newFixture(t)
	br := f.newIssue(t, "one")
	publishAll(t, f)
	b := machineB(t, f)
	mustSync(t, b, syncOpts())
	wt := wtPathOn(b, br)
	tip := mustRun(t, b, "rev-parse", "refs/heads/"+br)

	closeOnA(t, f, "one")

	before := localState(t, b)
	dry := mustSync(t, b, SyncOptions{Remote: "origin", DryRun: true})
	if after := localState(t, b); after != before {
		t.Fatalf("dry run changed local state:\n%s\n--\n%s", before, after)
	}
	if r := verbOf(t, dry, br); r.Verb != VerbRemoved || !strings.Contains(r.Detail, tip) {
		t.Fatalf("dry run = %+v, want removed with the tip", r)
	}

	real := mustSync(t, b, syncOpts())
	if pairs(real) != pairs(dry) {
		t.Errorf("real run %s differs from dry run %s", pairs(real), pairs(dry))
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree should be removed, stat err = %v", err)
	}
	if ok, _ := git.RefExists(b, "refs/heads/"+br); ok {
		t.Error("branch should be deleted")
	}
	// and it is recoverable from what was printed
	if _, err := git.Run(b, "branch", "bug/restored", tip); err != nil {
		t.Errorf("tip printed by sync cannot be restored: %v", err)
	}
}

func TestSyncKeepsClosedIssueWhenWorkWouldBeLost(t *testing.T) {
	t.Run("work the archive lacks", func(t *testing.T) {
		f := newFixture(t)
		br := f.newIssue(t, "one")
		publishAll(t, f)
		b := machineB(t, f)
		mustSync(t, b, syncOpts())
		wt := wtPathOn(b, br)
		if err := os.WriteFile(filepath.Join(wt, "mine.txt"), []byte("m"), 0o644); err != nil {
			t.Fatal(err)
		}
		mustRun(t, wt, "add", "mine.txt")
		mustRun(t, wt, "commit", "-q", "-m", "work on B")
		tip := mustRun(t, b, "rev-parse", "refs/heads/"+br)

		closeOnA(t, f, "one")
		rs := mustSync(t, b, syncOpts())
		r := verbOf(t, rs, br)
		if r.Verb != VerbDiffers || !strings.Contains(r.Detail, "mine.txt") || !r.Verb.NeedsAttention() {
			t.Fatalf("result = %+v, want differs naming mine.txt", r)
		}
		if got := mustRun(t, b, "rev-parse", "refs/heads/"+br); got != tip {
			t.Error("branch with unarchived work moved")
		}
		if _, err := os.Stat(filepath.Join(wt, "mine.txt")); err != nil {
			t.Errorf("worktree or file lost: %v", err)
		}
	})

	t.Run("uncommitted changes", func(t *testing.T) {
		f := newFixture(t)
		br := f.newIssue(t, "one")
		publishAll(t, f)
		b := machineB(t, f)
		mustSync(t, b, syncOpts())
		scratch := filepath.Join(wtPathOn(b, br), "scratch.txt")
		if err := os.WriteFile(scratch, []byte("uncommitted"), 0o644); err != nil {
			t.Fatal(err)
		}

		closeOnA(t, f, "one")
		rs := mustSync(t, b, syncOpts())
		r := verbOf(t, rs, br)
		if r.Verb != VerbInUse || !strings.Contains(r.Detail, "uncommitted") {
			t.Fatalf("result = %+v, want in-use (uncommitted)", r)
		}
		if got, err := os.ReadFile(scratch); err != nil || string(got) != "uncommitted" {
			t.Errorf("uncommitted file lost: %q, %v", got, err)
		}
	})
}

func TestSyncDoesNotResurrectAClosedIssueButDoesTakeAReopenedOne(t *testing.T) {
	f := newFixture(t)
	br := f.newIssue(t, "one")
	publishAll(t, f)

	// closed on A, archive pushed, but the remote branch NOT deleted
	if _, err := archive.Close(f.root, defaultCfg(), "bug", "one"); err != nil {
		t.Fatal(err)
	}
	publishAll(t, f)

	b2 := machineB(t, f)
	rs := mustSync(t, b2, syncOpts())
	r := verbOf(t, rs, br)
	if r.Verb != VerbClosed || !r.Verb.NeedsAttention() || !strings.Contains(r.Detail, "push origin --delete "+br) {
		t.Fatalf("result = %+v, want closed with the delete command", r)
	}
	if _, err := os.Stat(wtPathOn(b2, br)); !os.IsNotExist(err) {
		t.Errorf("a closed issue's worktree was resurrected (stat err = %v)", err)
	}
	if ok, _ := git.RefExists(b2, "refs/heads/"+br); ok {
		t.Error("a closed issue's branch was resurrected")
	}

	// reopened on A and pushed: now it is open again, and sync takes it
	mustRun(t, f.root, "push", "-q", "origin", "--delete", br)
	if _, err := archive.Reopen(f.root, defaultCfg(), "bug", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(f.root, "origin", []string{br}, false); err != nil {
		t.Fatal(err)
	}
	b3 := machineB(t, f)
	rs = mustSync(t, b3, syncOpts())
	if r := verbOf(t, rs, br); r.Verb != VerbCreated {
		t.Errorf("reopened issue = %+v, want created", r)
	}
}

func TestSyncIgnoresBranchesThatAreNotPmtIssues(t *testing.T) {
	f := newFixture(t)
	f.newIssue(t, "one")
	publishAll(t, f)
	base := mustRun(t, f.root, "rev-parse", "refs/heads/pmt/template/bug")
	mustRun(t, f.root, "push", "-q", "origin", base+":refs/heads/release/1.0", base+":refs/heads/chore/untemplated", base+":refs/heads/loose")
	b := machineB(t, f)

	rs := mustSync(t, b, syncOpts())
	for _, name := range []string{"release/1.0", "chore/untemplated", "loose"} {
		if r := verbOf(t, rs, name); r.Verb != VerbIgnored || r.Verb.NeedsAttention() {
			t.Errorf("%s = %+v, want ignored without attention", name, r)
		}
		if ok, _ := git.RefExists(b, "refs/heads/"+name); ok {
			t.Errorf("%s was created locally", name)
		}
	}
}

func TestSyncDirtyAndDivergedWorktreesAreReportedAndUntouched(t *testing.T) {
	f := newFixture(t)
	br := f.newIssue(t, "one")
	publishAll(t, f)
	b := machineB(t, f)
	mustSync(t, b, syncOpts())
	wt := wtPathOn(b, br)

	// dirty + the remote moved
	commitOnA(t, f, br, "remote1.txt")
	scratch := filepath.Join(wt, "scratch.txt")
	if err := os.WriteFile(scratch, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tip := mustRun(t, b, "rev-parse", "refs/heads/"+br)
	rs := mustSync(t, b, syncOpts())
	if r := verbOf(t, rs, br); r.Verb != VerbDirty {
		t.Fatalf("result = %+v, want dirty", r)
	}
	if mustRun(t, b, "rev-parse", "refs/heads/"+br) != tip {
		t.Error("a dirty worktree's branch moved")
	}

	// clean up, then diverge
	if err := os.Remove(scratch); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "local.txt"), []byte("l"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, wt, "add", "local.txt")
	mustRun(t, wt, "commit", "-q", "-m", "local")
	tip = mustRun(t, b, "rev-parse", "refs/heads/"+br)
	rs = mustSync(t, b, syncOpts())
	if r := verbOf(t, rs, br); r.Verb != VerbDiverged || !r.Verb.NeedsAttention() {
		t.Fatalf("result = %+v, want diverged", r)
	}
	if mustRun(t, b, "rev-parse", "refs/heads/"+br) != tip {
		t.Error("a diverged branch moved")
	}
}

func TestSyncFastForwardsMainCheckoutDefaultBranch(t *testing.T) {
	f := newFixture(t)
	if err := git.Push(f.root, "origin", f.defaultBranch(t)); err != nil {
		t.Fatal(err)
	}
	def := f.defaultBranch(t)
	b := machineB(t, f) // clone checks out the default branch and sets origin/HEAD

	if err := os.WriteFile(filepath.Join(f.root, "next.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, f.root, "add", "next.txt")
	mustRun(t, f.root, "commit", "-q", "-m", "next")
	if err := git.Push(f.root, "origin", def); err != nil {
		t.Fatal(err)
	}

	rs := mustSync(t, b, syncOpts())
	if r := verbOf(t, rs, def); r.Verb != VerbUpdated {
		t.Fatalf("main checkout = %+v, want updated (%s)", r, pairs(rs))
	}
	if _, err := os.Stat(filepath.Join(b, "next.txt")); err != nil {
		t.Errorf("main checkout not fast-forwarded: %v", err)
	}

	// dirty main checkout is left alone
	if err := os.WriteFile(filepath.Join(f.root, "next2.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, f.root, "add", "next2.txt")
	mustRun(t, f.root, "commit", "-q", "-m", "next2")
	if err := git.Push(f.root, "origin", def); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rs = mustSync(t, b, syncOpts())
	if r := verbOf(t, rs, def); r.Verb != VerbDirty {
		t.Errorf("dirty main checkout = %+v, want dirty", r)
	}
}

func TestSyncReportsDetachedAndOrphanedWorktrees(t *testing.T) {
	f := newFixture(t)
	br := f.newIssue(t, "one")
	publishAll(t, f)
	b := machineB(t, f)
	mustSync(t, b, syncOpts())

	detached := filepath.Join(t.TempDir(), "detached-wt")
	mustRun(t, b, "worktree", "add", "-q", "--detach", detached, "refs/heads/"+br)

	orphan := filepath.Join(wtPathOn(b, br), "..", "stray-dir")
	orphan = filepath.Clean(orphan)
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphan, ".git"), []byte("gitdir: nowhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rs := mustSync(t, b, syncOpts())
	if r := verbOf(t, rs, detached); r.Verb != VerbDetached || !r.Verb.NeedsAttention() {
		t.Errorf("detached = %+v", r)
	}
	if r := verbOf(t, rs, orphan); r.Verb != VerbOrphaned {
		t.Errorf("orphan = %+v", r)
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Errorf("sync deleted the orphan directory: %v", err)
	}
	if _, err := os.Stat(detached); err != nil {
		t.Errorf("sync removed the detached worktree: %v", err)
	}
}

func TestSyncSkipUpdateOnlyCreates(t *testing.T) {
	f := newFixture(t)
	one := f.newIssue(t, "one")
	publishAll(t, f)
	b := machineB(t, f)
	mustSync(t, b, syncOpts())
	commitOnA(t, f, one, "later.txt")
	two := f.newIssue(t, "two")
	publishAll(t, f)

	rs := mustSync(t, b, SyncOptions{Remote: "origin", SkipUpdate: true})
	if verbOf(t, rs, two).Verb != VerbCreated {
		t.Errorf("new issue not created: %s", pairs(rs))
	}
	for _, r := range rs {
		if r.Branch == one {
			t.Errorf("existing worktree was handled under --skip-update: %+v", r)
		}
	}
	if _, err := os.Stat(filepath.Join(wtPathOn(b, one), "later.txt")); !os.IsNotExist(err) {
		t.Errorf("--skip-update still fast-forwarded (stat err = %v)", err)
	}
}

func TestSyncErrorsAreActionableAndLeakNothing(t *testing.T) {
	f := newFixture(t)
	if _, err := Sync(f.root, defaultCfg(), SyncOptions{Remote: "nope"}); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("unknown remote error = %v", err)
	}

	mustRun(t, f.root, "config", "--unset-all", "remote.origin.fetch")
	_, err := Sync(f.root, defaultCfg(), syncOpts())
	if err == nil || !strings.Contains(err.Error(), "no fetch refspec") {
		t.Errorf("missing refspec error = %v", err)
	}
	if err != nil && strings.Contains(err.Error(), f.bare) {
		t.Errorf("error leaks the remote location: %v", err)
	}

	mustRun(t, f.root, "remote", "set-url", "origin", "https://user:SECRET@127.0.0.1:1/org/x.git")
	mustRun(t, f.root, "config", "--add", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	_, err = Sync(f.root, defaultCfg(), syncOpts())
	if err == nil {
		t.Fatal("expected a fetch failure")
	}
	for _, leak := range []string{"SECRET", "127.0.0.1", "user:"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error leaks %q: %v", leak, err)
		}
	}
}

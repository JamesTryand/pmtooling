package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// machineB is a second machine's clone of the fixture's remote: it has the
// remote-tracking refs but no local issue branches and no worktrees.
func machineB(t *testing.T, f fixture) string {
	t.Helper()
	b := t.TempDir()
	mustRun(t, b, "clone", "-q", f.bare, ".")
	configureUser(t, b)
	return b
}

// publishedIssue creates an issue on machine A and pushes it.
func publishedIssue(t *testing.T, f fixture, title string) string {
	t.Helper()
	br := f.newIssue(t, title)
	if _, err := Push(f.root, "origin", []string{br}, false); err != nil {
		t.Fatalf("Push: %v", err)
	}
	return br
}

// commitOnA adds a commit on machine A's issue worktree and pushes it.
func commitOnA(t *testing.T, f fixture, branch, file string) {
	t.Helper()
	wt := git.ComputeWorktreePath(f.root, "", "bug", strings.TrimPrefix(branch, "bug/"))
	if err := os.WriteFile(filepath.Join(wt, file), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, wt, "add", file)
	mustRun(t, wt, "commit", "-q", "-m", "A: "+file)
	if _, err := Push(f.root, "origin", []string{branch}, false); err != nil {
		t.Fatalf("Push: %v", err)
	}
}

func wtPathOn(root, branch string) string {
	return git.ComputeWorktreePath(root, "", "bug", strings.TrimPrefix(branch, "bug/"))
}

func snapshotAll(t *testing.T, dir string) string {
	t.Helper()
	return snapshot(t, dir) + "\n" + mustRun(t, dir, "worktree", "list", "--porcelain")
}

func TestPullCreatesBranchAndWorktreeForRemoteOnlyIssue(t *testing.T) {
	f := newFixture(t)
	br := publishedIssue(t, f, "one")
	b := machineB(t, f)

	res, err := Pull(b, "", "origin", br, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verb != VerbCreated {
		t.Fatalf("Pull = %+v, want created", res)
	}
	wt := wtPathOn(b, br)
	if _, err := os.Stat(filepath.Join(wt, "README.md")); err != nil {
		t.Errorf("worktree not populated: %v", err)
	}
	if up := mustRun(t, b, "rev-parse", "--abbrev-ref", br+"@{upstream}"); up != "origin/"+br {
		t.Errorf("upstream = %q, want origin/%s", up, br)
	}

	// idempotent (G4)
	res, err = Pull(b, "", "origin", br, false)
	if err != nil || res.Verb != VerbUpToDate {
		t.Errorf("second Pull = %+v, %v; want up-to-date", res, err)
	}
}

func TestPullFastForwardsCleanWorktree(t *testing.T) {
	f := newFixture(t)
	br := publishedIssue(t, f, "one")
	b := machineB(t, f)
	if _, err := Pull(b, "", "origin", br, false); err != nil {
		t.Fatal(err)
	}

	commitOnA(t, f, br, "later.txt")
	res, err := Pull(b, "", "origin", br, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verb != VerbUpdated || !strings.Contains(res.Detail, "..") {
		t.Fatalf("Pull = %+v, want updated with a range", res)
	}
	if _, err := os.Stat(filepath.Join(wtPathOn(b, br), "later.txt")); err != nil {
		t.Errorf("fast-forward did not update the worktree files: %v", err)
	}
}

func TestPullDirtyWorktreeIsReportedAndUntouched(t *testing.T) {
	f := newFixture(t)
	br := publishedIssue(t, f, "one")
	b := machineB(t, f)
	if _, err := Pull(b, "", "origin", br, false); err != nil {
		t.Fatal(err)
	}
	commitOnA(t, f, br, "later.txt")

	mine := filepath.Join(wtPathOn(b, br), "scratch.txt")
	if err := os.WriteFile(mine, []byte("uncommitted"), 0o644); err != nil {
		t.Fatal(err)
	}
	tip := mustRun(t, b, "rev-parse", "refs/heads/"+br)

	res, err := Pull(b, "", "origin", br, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verb != VerbDirty || !res.Verb.NeedsAttention() {
		t.Fatalf("Pull = %+v, want dirty (needs attention)", res)
	}
	if got := mustRun(t, b, "rev-parse", "refs/heads/"+br); got != tip {
		t.Errorf("a dirty worktree's branch moved: %s -> %s", tip, got)
	}
	if got, err := os.ReadFile(mine); err != nil || string(got) != "uncommitted" {
		t.Errorf("uncommitted file touched: %q, %v", got, err)
	}
}

func TestPullDivergedAndAheadChangeNothing(t *testing.T) {
	f := newFixture(t)
	br := publishedIssue(t, f, "one")
	b := machineB(t, f)
	if _, err := Pull(b, "", "origin", br, false); err != nil {
		t.Fatal(err)
	}
	wt := wtPathOn(b, br)

	// local-only commit => ahead, exit-0 style
	if err := os.WriteFile(filepath.Join(wt, "local.txt"), []byte("l"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, wt, "add", "local.txt")
	mustRun(t, wt, "commit", "-q", "-m", "B: local")
	res, err := Pull(b, "", "origin", br, false)
	if err != nil || res.Verb != VerbAhead || res.Verb.NeedsAttention() {
		t.Fatalf("Pull = %+v, %v; want ahead without needing attention", res, err)
	}

	// remote also moves => diverged
	commitOnA(t, f, br, "remote.txt")
	tipBefore := mustRun(t, b, "rev-parse", "refs/heads/"+br)
	res, err = Pull(b, "", "origin", br, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verb != VerbDiverged || !strings.Contains(res.Detail, "...") {
		t.Fatalf("Pull = %+v, want diverged with a a...b range", res)
	}
	// Pull fetches, so remote-tracking refs may move; the local branch, worktree and files must not.
	if got := mustRun(t, b, "rev-parse", "refs/heads/"+br); got != tipBefore {
		t.Errorf("local branch moved on a diverged pull: %s -> %s", tipBefore, got)
	}
	if status := mustRun(t, wt, "status", "--porcelain"); status != "" {
		t.Errorf("worktree changed on a diverged pull: %q", status)
	}
	if subj := mustRun(t, wt, "log", "-1", "--format=%s"); subj != "B: local" {
		t.Errorf("local commit lost or replaced: HEAD subject = %q", subj)
	}
}

func TestPullExistingBranchWithoutWorktreeBehind(t *testing.T) {
	f := newFixture(t)
	br := publishedIssue(t, f, "one")
	b := machineB(t, f)
	if _, err := Pull(b, "", "origin", br, false); err != nil {
		t.Fatal(err)
	}
	// worktree removed by hand; the remote then moves
	mustRun(t, b, "worktree", "remove", wtPathOn(b, br))
	commitOnA(t, f, br, "later.txt")

	res, err := Pull(b, "", "origin", br, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verb != VerbCreated || !strings.Contains(res.Detail, "fast-forwarded") {
		t.Fatalf("Pull = %+v, want created with a fast-forward", res)
	}
	if _, err := os.Stat(filepath.Join(wtPathOn(b, br), "later.txt")); err != nil {
		t.Errorf("recreated worktree is not at the new tip: %v", err)
	}
}

func TestPullOrphanedAndPrunablePaths(t *testing.T) {
	f := newFixture(t)
	br := publishedIssue(t, f, "one")
	b := machineB(t, f)

	// a directory sits where the worktree should go, unknown to git
	wt := wtPathOn(b, br)
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Pull(b, "", "origin", br, false)
	if err != nil || res.Verb != VerbOrphaned || !res.Verb.NeedsAttention() {
		t.Fatalf("Pull over a stray directory = %+v, %v; want orphaned", res, err)
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}

	// registered worktree whose directory vanished
	if _, err := Pull(b, "", "origin", br, false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	res, err = Pull(b, "", "origin", br, false)
	if err != nil || res.Verb != VerbPrunable {
		t.Fatalf("Pull with a vanished worktree = %+v, %v; want prunable", res, err)
	}
}

func TestPullMissingIssueSuggestsNearMatches(t *testing.T) {
	f := newFixture(t)
	publishedIssue(t, f, "dboverflow")
	b := machineB(t, f)

	_, err := Pull(b, "", "origin", "bug/dboverfloww", false)
	if err == nil {
		t.Fatal("expected an error for an issue that is not on the remote")
	}
	if !strings.Contains(err.Error(), "bug/dboverfloww") || !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("error should name the issue and suggest near matches: %v", err)
	}

	// exists only locally: point at pmt push instead
	mustRun(t, b, "branch", "bug/localonly", "refs/remotes/origin/bug/dboverflow")
	if _, err := Pull(b, "", "origin", "bug/localonly", false); err == nil || !strings.Contains(err.Error(), "pmt push") {
		t.Errorf("local-only issue error = %v, want a pointer to pmt push", err)
	}
}

func TestPullDryRunChangesNothing(t *testing.T) {
	f := newFixture(t)
	br := publishedIssue(t, f, "one")
	b := machineB(t, f)
	before := snapshotAll(t, b)

	res, err := Pull(b, "", "origin", br, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verb != VerbCreated {
		t.Errorf("dry run = %+v, want created (what a real run would do)", res)
	}
	if after := snapshotAll(t, b); after != before {
		t.Errorf("dry run changed repo state:\n%s\n--\n%s", before, after)
	}
	if _, err := os.Stat(wtPathOn(b, br)); !os.IsNotExist(err) {
		t.Errorf("dry run created the worktree directory (stat err = %v)", err)
	}

	real, err := Pull(b, "", "origin", br, false)
	if err != nil || real.Verb != res.Verb {
		t.Errorf("real run = %+v, %v; want same verb as dry run (%s)", real, err, res.Verb)
	}
}

func TestPullRefusesNonIssueBranchesAndUnknownRemote(t *testing.T) {
	f := newFixture(t)
	for _, name := range []string{"pmt/archive", "pmt/template/bug", "main", ""} {
		if _, err := Pull(f.root, "", "origin", name, false); err == nil {
			t.Errorf("Pull(%q) should be refused", name)
		}
	}
	if _, err := Pull(f.root, "", "nope", "bug/x", false); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("unknown remote error = %v", err)
	}
}

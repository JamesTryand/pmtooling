package publish

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/template"
)

func defaultCfg() config.RepoConfig {
	return config.RepoConfig{TitlePadWidth: config.DefaultTitlePadWidth}
}

func mustRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git.Run(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func configureUser(t *testing.T, dir string) {
	t.Helper()
	mustRun(t, dir, "config", "user.email", "test@example.com")
	mustRun(t, dir, "config", "user.name", "pmt test")
}

// fixture is a target repo with a "bug" template and a local bare repo as
// origin (a filesystem path: no credentialed URL is ever needed).
type fixture struct {
	root, bare string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	bare := t.TempDir()
	mustRun(t, bare, "init", "-q", "--bare")

	root := t.TempDir()
	mustRun(t, root, "init", "-q")
	configureUser(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, root, "add", "README.md")
	mustRun(t, root, "commit", "-q", "-m", "init")
	mustRun(t, root, "remote", "add", "origin", bare)

	if _, err := template.New(root, "bug"); err != nil {
		t.Fatalf("template.New: %v", err)
	}
	return fixture{root: root, bare: bare}
}

// defaultBranch is the branch the fixture's target repo has checked out.
func (f fixture) defaultBranch(t *testing.T) string {
	t.Helper()
	b, onBranch, err := git.CurrentBranch(f.root)
	if err != nil || !onBranch {
		t.Fatalf("CurrentBranch: %q, %v, %v", b, onBranch, err)
	}
	return b
}

func (f fixture) newIssue(t *testing.T, title string) string {
	t.Helper()
	res, err := issue.Create(f.root, defaultCfg(), "bug", title)
	if err != nil {
		t.Fatalf("issue.Create: %v", err)
	}
	return res.Branch
}

// snapshot captures every ref in dir, so tests can prove nothing moved.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	return mustRun(t, dir, "for-each-ref", "--format=%(refname) %(objectname)")
}

func byBranch(rs []Result) map[string]Result {
	m := map[string]Result{}
	for _, r := range rs {
		m[r.Branch] = r
	}
	return m
}

func TestManagedBranchesCoversIssuesTemplatesAndArchive(t *testing.T) {
	f := newFixture(t)
	b := f.newIssue(t, "one")

	got, err := ManagedBranches(f.root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{b, "pmt/template/bug"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ManagedBranches = %v, want %v (no default branch, no archive yet)", got, want)
	}

	// once something is archived the archive branch is managed too
	mustRun(t, f.root, "update-ref", "refs/heads/pmt/archive", mustRun(t, f.root, "rev-parse", "HEAD"))
	got, _ = ManagedBranches(f.root)
	if !contains(got, "pmt/archive") {
		t.Errorf("ManagedBranches = %v, want pmt/archive included", got)
	}
	for _, g := range got {
		if g == "master" || g == "main" {
			t.Errorf("default branch %q must never be managed", g)
		}
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestPushNewThenUpToDateThenAhead(t *testing.T) {
	f := newFixture(t)
	b := f.newIssue(t, "one")

	rs, err := Push(f.root, "origin", []string{b, "pmt/template/bug"}, false)
	if err != nil {
		t.Fatal(err)
	}
	m := byBranch(rs)
	if m[b].Verb != VerbNew || m["pmt/template/bug"].Verb != VerbNew {
		t.Fatalf("first push = %+v, want both new", rs)
	}
	if ok, _ := git.RefExists(f.bare, "refs/heads/"+b); !ok {
		t.Fatal("issue branch not on the remote")
	}

	// idempotent (G4)
	rs, _ = Push(f.root, "origin", []string{b}, false)
	if rs[0].Verb != VerbUpToDate {
		t.Errorf("second push = %v, want up-to-date", rs[0].Verb)
	}

	// ahead: add a commit on the issue branch via its worktree
	wt := git.ComputeWorktreePath(f.root, "", "bug", strings.TrimPrefix(b, "bug/"))
	if err := os.WriteFile(filepath.Join(wt, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, wt, "add", "notes.txt")
	mustRun(t, wt, "commit", "-q", "-m", "work")

	rs, _ = Push(f.root, "origin", []string{b}, false)
	if rs[0].Verb != VerbPushed || !strings.Contains(rs[0].Detail, "..") {
		t.Errorf("ahead push = %+v, want pushed with a range", rs[0])
	}
	local := mustRun(t, f.root, "rev-parse", "refs/heads/"+b)
	remote := mustRun(t, f.bare, "rev-parse", "refs/heads/"+b)
	if local != remote {
		t.Errorf("remote tip %s != local %s after push", remote, local)
	}
}

func TestPushDryRunMutatesNothingAndMatchesRealRun(t *testing.T) {
	f := newFixture(t)
	b := f.newIssue(t, "one")
	branches := []string{b, "pmt/template/bug"}

	localBefore, bareBefore := snapshot(t, f.root), snapshot(t, f.bare)
	dry, err := Push(f.root, "origin", branches, true)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot(t, f.root) != localBefore || snapshot(t, f.bare) != bareBefore {
		t.Fatal("dry run changed refs")
	}

	real, err := Push(f.root, "origin", branches, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dry, real) {
		t.Errorf("dry-run actions %v differ from real run %v", dry, real)
	}
}

func TestPushReportsDivergedAndBehindWithoutTouchingRemote(t *testing.T) {
	f := newFixture(t)
	b := f.newIssue(t, "one")
	if _, err := Push(f.root, "origin", []string{b}, false); err != nil {
		t.Fatal(err)
	}

	// someone else moves the remote branch
	other := t.TempDir()
	mustRun(t, other, "clone", "-q", f.bare, ".")
	configureUser(t, other)
	mustRun(t, other, "checkout", "-q", b)
	if err := os.WriteFile(filepath.Join(other, "theirs.txt"), []byte("t"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, other, "add", "theirs.txt")
	mustRun(t, other, "commit", "-q", "-m", "theirs")
	mustRun(t, other, "push", "-q", "origin", "HEAD:refs/heads/"+b)
	remoteTip := mustRun(t, f.bare, "rev-parse", "refs/heads/"+b)

	// we have not fetched: remote has commits we lack
	rs, _ := Push(f.root, "origin", []string{b}, false)
	if rs[0].Verb != VerbMoved || !rs[0].Verb.NeedsAttention() {
		t.Errorf("unfetched remote move = %+v, want moved", rs[0])
	}

	// after fetching, still only behind
	if err := git.FetchRemote(f.root, "origin"); err != nil {
		t.Fatal(err)
	}
	rs, _ = Push(f.root, "origin", []string{b}, false)
	if rs[0].Verb != VerbBehind {
		t.Errorf("after fetch = %+v, want behind", rs[0])
	}

	// local work as well => diverged
	wt := git.ComputeWorktreePath(f.root, "", "bug", strings.TrimPrefix(b, "bug/"))
	if err := os.WriteFile(filepath.Join(wt, "mine.txt"), []byte("m"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, wt, "add", "mine.txt")
	mustRun(t, wt, "commit", "-q", "-m", "mine")
	rs, _ = Push(f.root, "origin", []string{b}, false)
	if rs[0].Verb != VerbDiverged {
		t.Errorf("both moved = %+v, want diverged", rs[0])
	}

	if got := mustRun(t, f.bare, "rev-parse", "refs/heads/"+b); got != remoteTip {
		t.Errorf("remote was moved by a non-pushable branch: %s -> %s", remoteTip, got)
	}
}

func TestPushUnknownRemoteAndUnknownBranch(t *testing.T) {
	f := newFixture(t)
	if _, err := Push(f.root, "nope", nil, false); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("unknown remote error = %v", err)
	}
	if _, err := Push(f.root, "origin", []string{"bug/ghost"}, false); err == nil {
		t.Error("expected an error for a branch that does not exist locally")
	}
}

// TestPushOneBranchFailingDoesNotStopTheRest makes the first push fail via a
// remote-side hook and checks the second branch is still attempted and that
// nothing in the report leaks the remote's location.
func TestPushOneBranchFailingDoesNotStopTheRest(t *testing.T) {
	f := newFixture(t)
	a := f.newIssue(t, "a")
	b := f.newIssue(t, "b")

	hook := filepath.Join(f.bare, "hooks", "update")
	script := "#!/bin/sh\ncase \"$1\" in refs/heads/" + a + ") echo rejected-by-hook >&2; exit 1;; esac\nexit 0\n"
	if err := os.WriteFile(hook, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	rs, err := Push(f.root, "origin", []string{a, b}, false)
	if err != nil {
		t.Fatal(err)
	}
	m := byBranch(rs)
	if m[a].Verb != VerbFailed {
		t.Fatalf("hooked branch = %+v, want failed", m[a])
	}
	if m[b].Verb != VerbNew {
		t.Errorf("second branch = %+v, want new (still pushed)", m[b])
	}
	if strings.Contains(m[a].Detail, f.bare) {
		t.Errorf("failure detail leaks the remote location: %q", m[a].Detail)
	}
}

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// pushFixture is a target repo (via the real `pmt template new` / `pmt new`
// commands) whose origin is a local bare repo.
func pushFixture(t *testing.T) (root, bare string) {
	t.Helper()
	bare = t.TempDir()
	if _, err := git.Run(bare, "init", "-q", "--bare"); err != nil {
		t.Fatal(err)
	}
	root = initRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "README.md"}, {"commit", "-q", "-m", "init"}, {"remote", "add", "origin", bare}} {
		if _, err := git.Run(root, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	if _, err := execRoot(t, "template", "new", "bug", "--repo", root); err != nil {
		t.Fatalf("template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/one", "--repo", root); err != nil {
		t.Fatalf("new: %v", err)
	}
	return root, bare
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var e *ExitCodeError
	if errors.As(err, &e) {
		return e.Code
	}
	return 1
}

func TestPushAllThenIdempotent(t *testing.T) {
	root, bare := pushFixture(t)

	out, err := execRoot(t, "push", "--all", "--repo", root)
	if err != nil {
		t.Fatalf("push --all: %v\n%s", err, out)
	}
	for _, want := range []string{"new         bug/one", "new         pmt/template/bug", "2 new"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, bare) {
		t.Errorf("output leaks the remote location:\n%s", out)
	}
	if ok, _ := git.RefExists(bare, "refs/heads/bug/one"); !ok {
		t.Error("bug/one not on the remote")
	}

	out, err = execRoot(t, "push", "--all", "--repo", root)
	if err != nil || exitCodeOf(err) != 0 {
		t.Fatalf("second push: %v\n%s", err, out)
	}
	if !strings.Contains(out, "2 up-to-date") || strings.Contains(out, "new ") {
		t.Errorf("second push should only report up-to-date:\n%s", out)
	}
}

func TestPushDryRunChangesNothing(t *testing.T) {
	root, bare := pushFixture(t)
	before, _ := git.Run(bare, "for-each-ref")

	out, err := execRoot(t, "push", "--all", "--dry-run", "--repo", root)
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	if after, _ := git.Run(bare, "for-each-ref"); after != before {
		t.Error("dry run changed the remote")
	}
	if !strings.Contains(out, "dry run") || !strings.Contains(out, "bug/one") {
		t.Errorf("dry-run output = %q", out)
	}
}

func TestPushSingleIssueAndGuards(t *testing.T) {
	root, bare := pushFixture(t)

	if out, err := execRoot(t, "push", "bug/one", "--repo", root); err != nil {
		t.Fatalf("push bug/one: %v\n%s", err, out)
	}
	if ok, _ := git.RefExists(bare, "refs/heads/pmt/template/bug"); ok {
		t.Error("pushing one issue must not push the template")
	}

	// the default branch is not pmt's to publish
	def, _, _ := git.CurrentBranch(root)
	if _, err := execRoot(t, "push", def, "--repo", root); err == nil || !strings.Contains(err.Error(), "not a pmt-managed branch") {
		t.Errorf("pushing the default branch: err = %v, want a managed-branch refusal", err)
	}
	if ok, _ := git.RefExists(bare, "refs/heads/"+def); ok {
		t.Error("default branch reached the remote")
	}

	if _, err := execRoot(t, "push", "bug/one", "--all", "--repo", root); err == nil {
		t.Error("issue together with --all should be rejected")
	}
	if _, err := execRoot(t, "push", "--all", "--remote", "nope", "--repo", root); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("unknown remote: err = %v", err)
	}
}

func TestPushInfersIssueFromCwd(t *testing.T) {
	root, bare := pushFixture(t)
	wt := git.ComputeWorktreePath(root, "", "bug", "one")
	chdir(t, wt)

	out, err := execRoot(t, "push")
	if err != nil {
		t.Fatalf("push from the issue worktree: %v\n%s", err, out)
	}
	if ok, _ := git.RefExists(bare, "refs/heads/bug/one"); !ok {
		t.Errorf("bug/one not pushed:\n%s", out)
	}
}

func TestPushDivergedExitsTwo(t *testing.T) {
	root, bare := pushFixture(t)
	if _, err := execRoot(t, "push", "bug/one", "--repo", root); err != nil {
		t.Fatal(err)
	}

	// the remote branch moves without us
	other := t.TempDir()
	if _, err := git.Run(other, "clone", "-q", bare, "."); err != nil {
		t.Fatal(err)
	}
	git.Run(other, "config", "user.email", "t@example.com")
	git.Run(other, "config", "user.name", "t")
	git.Run(other, "checkout", "-q", "bug/one")
	os.WriteFile(filepath.Join(other, "x.txt"), []byte("x"), 0o644)
	git.Run(other, "add", "x.txt")
	git.Run(other, "commit", "-q", "-m", "theirs")
	if _, err := git.Run(other, "push", "-q", "origin", "HEAD:refs/heads/bug/one"); err != nil {
		t.Fatal(err)
	}

	out, err := execRoot(t, "push", "bug/one", "--repo", root)
	if exitCodeOf(err) != ExitCodeAttention {
		t.Fatalf("exit code = %d (err %v), want %d\n%s", exitCodeOf(err), err, ExitCodeAttention, out)
	}
	if !strings.Contains(out, "moved") {
		t.Errorf("output should report the remote move:\n%s", out)
	}
}

// TestPushFailureLeaksNoCredentials points origin at a credentialed URL that
// cannot be reached and checks nothing printed contains the secret or host.
func TestPushFailureLeaksNoCredentials(t *testing.T) {
	root, _ := pushFixture(t)
	if _, err := git.Run(root, "remote", "set-url", "origin", "https://user:SECRET@127.0.0.1:1/org/x.git"); err != nil {
		t.Fatal(err)
	}
	out, err := execRoot(t, "push", "--all", "--repo", root)
	if err == nil {
		t.Fatal("expected an error pushing to an unreachable remote")
	}
	all := out + err.Error()
	for _, leak := range []string{"SECRET", "127.0.0.1", "user:"} {
		if strings.Contains(all, leak) {
			t.Errorf("output leaks %q:\n%s", leak, all)
		}
	}
}

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// machineBClone is a fresh clone of bare, standing in for a second machine.
func machineBClone(t *testing.T, bare string) string {
	t.Helper()
	b := t.TempDir()
	if _, err := git.Run(b, "clone", "-q", bare, "."); err != nil {
		t.Fatal(err)
	}
	git.Run(b, "config", "user.email", "t@example.com")
	git.Run(b, "config", "user.name", "t")
	return b
}

func TestPullCmdCreatesThenReportsDirtyWithExitTwo(t *testing.T) {
	root, bare := pushFixture(t)
	if out, err := execRoot(t, "push", "bug/one", "--repo", root); err != nil {
		t.Fatalf("push: %v\n%s", err, out)
	}
	b := machineBClone(t, bare)

	out, err := execRoot(t, "pull", "bug/one", "--repo", b)
	if err != nil {
		t.Fatalf("pull: %v\n%s", err, out)
	}
	if !strings.HasPrefix(out, "created     bug/one") || !strings.Contains(out, "1 created") {
		t.Errorf("pull output = %q", out)
	}
	if strings.Contains(out, bare) {
		t.Errorf("output leaks the remote location:\n%s", out)
	}
	wt := git.ComputeWorktreePath(b, "", "bug", "one")
	if _, err := os.Stat(filepath.Join(wt, "README.md")); err != nil {
		t.Errorf("worktree not created: %v", err)
	}

	if err := os.WriteFile(filepath.Join(wt, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = execRoot(t, "pull", "bug/one", "--repo", b)
	if exitCodeOf(err) != ExitCodeAttention || !strings.Contains(out, "dirty") {
		t.Errorf("dirty pull: exit %d, output %q; want exit %d and a dirty line", exitCodeOf(err), out, ExitCodeAttention)
	}
}

func TestPullCmdErrorsAreActionable(t *testing.T) {
	root, _ := pushFixture(t)
	if _, err := execRoot(t, "pull", "bug/ghost", "--repo", root); err == nil || !strings.Contains(err.Error(), "not on origin") {
		t.Errorf("missing issue error = %v", err)
	}
	if _, err := execRoot(t, "pull", "--repo", root); err == nil {
		t.Error("pull without an issue should be a usage error")
	}
}

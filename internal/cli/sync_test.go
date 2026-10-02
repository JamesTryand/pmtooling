package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

func TestSyncCmdOnASecondMachineThenUnpublishedExitsTwo(t *testing.T) {
	root, bare := pushFixture(t)
	if out, err := execRoot(t, "push", "--all", "--repo", root); err != nil {
		t.Fatalf("push --all: %v\n%s", err, out)
	}
	b := machineBClone(t, bare)

	out, err := execRoot(t, "sync", "--repo", b)
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, out)
	}
	for _, want := range []string{"created     bug/one", "created     pmt/template/bug", "2 created"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, bare) {
		t.Errorf("output leaks the remote location:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(git.ComputeWorktreePath(b, "", "bug", "one"), "README.md")); err != nil {
		t.Errorf("worktree not created: %v", err)
	}

	out, err = execRoot(t, "sync", "--repo", b)
	if err != nil || !strings.Contains(out, "2 up-to-date") {
		t.Errorf("second sync = %q, %v; want everything up-to-date, exit 0", out, err)
	}

	// an issue created here and never pushed: reported, kept, exit 2
	if _, err := execRoot(t, "new", "bug/two", "--repo", b); err != nil {
		t.Fatal(err)
	}
	out, err = execRoot(t, "sync", "--repo", b)
	if exitCodeOf(err) != ExitCodeAttention || !strings.Contains(out, "unpublished") || !strings.Contains(out, "pmt push bug/two") {
		t.Errorf("exit %d, output %q; want exit %d naming bug/two as unpublished", exitCodeOf(err), out, ExitCodeAttention)
	}
	if ok, _ := git.RefExists(b, "refs/heads/bug/two"); !ok {
		t.Error("the unpublished issue's branch was removed")
	}
}

func TestSyncCmdDryRunChangesNothingLocal(t *testing.T) {
	root, bare := pushFixture(t)
	if _, err := execRoot(t, "push", "--all", "--repo", root); err != nil {
		t.Fatal(err)
	}
	b := machineBClone(t, bare)
	before, _ := git.Run(b, "for-each-ref", "refs/heads")

	out, err := execRoot(t, "sync", "--dry-run", "--repo", b)
	if err != nil {
		t.Fatalf("sync --dry-run: %v\n%s", err, out)
	}
	if after, _ := git.Run(b, "for-each-ref", "refs/heads"); after != before {
		t.Error("dry run changed local branches")
	}
	if _, err := os.Stat(git.ComputeWorktreePath(b, "", "bug", "one")); !os.IsNotExist(err) {
		t.Errorf("dry run created a worktree (stat err = %v)", err)
	}
	if !strings.Contains(out, "created     bug/one") || !strings.Contains(out, "dry run") {
		t.Errorf("dry-run output = %q", out)
	}
}

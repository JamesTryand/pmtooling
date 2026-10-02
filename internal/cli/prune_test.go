package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

func writeAndCommit(t *testing.T, wt, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(wt, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", name}, {"commit", "-q", "-m", "add " + name}} {
		if _, err := git.Run(wt, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
}

func TestPruneCmdEndToEnd(t *testing.T) {
	root := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", root); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"gone", "mine"} {
		if _, err := execRoot(t, "new", "bug/"+title, "--repo", root); err != nil {
			t.Fatal(err)
		}
	}
	tip, err := git.Run(root, "rev-parse", "refs/heads/bug/gone")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execRoot(t, "close", "bug/gone", "--repo", root); err != nil {
		t.Fatal(err)
	}
	// the leftover as another machine would still have it
	if _, err := git.Run(root, "branch", "bug/gone", tip); err != nil {
		t.Fatal(err)
	}

	out, err := execRoot(t, "prune", "--dry-run", "--repo", root)
	if err != nil {
		t.Fatalf("prune --dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "pruned      bug/gone") || !strings.Contains(out, "dry run") {
		t.Errorf("dry-run output = %q", out)
	}
	if ok, _ := git.RefExists(root, "refs/heads/bug/gone"); !ok {
		t.Fatal("dry run deleted the branch")
	}

	out, err = execRoot(t, "prune", "--repo", root)
	if err != nil {
		t.Fatalf("prune: %v\n%s", err, out)
	}
	if !strings.Contains(out, tip) || !strings.Contains(out, "git branch bug/gone") {
		t.Errorf("output should print the tip and a restore command:\n%s", out)
	}
	if ok, _ := git.RefExists(root, "refs/heads/bug/gone"); ok {
		t.Error("leftover branch still present")
	}
	if ok, _ := git.RefExists(root, "refs/heads/bug/mine"); !ok {
		t.Error("an open issue's branch was deleted")
	}

	out, err = execRoot(t, "prune", "--repo", root)
	if err != nil || !strings.Contains(out, "nothing to prune") {
		t.Errorf("second prune = %q, %v; want nothing to prune", out, err)
	}
}

func TestPruneCmdExitTwoWhenBranchHasUnarchivedWork(t *testing.T) {
	root := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", root); err != nil {
		t.Fatal(err)
	}
	if _, err := execRoot(t, "new", "bug/one", "--repo", root); err != nil {
		t.Fatal(err)
	}
	if _, err := execRoot(t, "close", "bug/one", "--repo", root); err != nil {
		t.Fatal(err)
	}
	// a branch with a file the archive never saw
	archiveTip, _ := git.Run(root, "rev-parse", "refs/heads/pmt/archive")
	stamped, _ := git.Run(root, "rev-parse", archiveTip+"^1")
	if _, err := git.Run(root, "branch", "bug/one", stamped); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(t.TempDir(), "w")
	if err := git.WorktreeAdd(root, wt, "bug/one"); err != nil {
		t.Fatal(err)
	}
	writeAndCommit(t, wt, "extra.txt", "unarchived")
	if _, err := git.Run(root, "worktree", "remove", wt); err != nil {
		t.Fatal(err)
	}

	out, err := execRoot(t, "prune", "--repo", root)
	if exitCodeOf(err) != ExitCodeAttention || !strings.Contains(out, "differs") || !strings.Contains(out, "extra.txt") {
		t.Errorf("exit %d, output %q; want exit %d naming extra.txt", exitCodeOf(err), out, ExitCodeAttention)
	}
	if ok, _ := git.RefExists(root, "refs/heads/bug/one"); !ok {
		t.Error("branch with unarchived work was deleted")
	}
}

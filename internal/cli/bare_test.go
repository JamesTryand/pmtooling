package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// initBareRepo creates a bare repo with one commit on master (via
// plumbing, since a bare repo has no working tree to commit normally) so
// there's a valid start-point for branches created from it.
func initBareRepo(t *testing.T, dirName string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "init", "-q", "--bare"); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	if _, err := git.Run(dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "config", "user.name", "pmt test"); err != nil {
		t.Fatal(err)
	}
	blob, err := git.HashObject(dir, []byte("# bare target repo\n"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := git.Mktree(dir, []git.TreeEntry{{Mode: "100644", Type: "blob", SHA: blob, Name: "README.md"}})
	if err != nil {
		t.Fatal(err)
	}
	commit, err := git.CommitTree(dir, tree, "init")
	if err != nil {
		t.Fatal(err)
	}
	if err := git.UpdateRef(dir, "refs/heads/master", commit); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTemplateNewCmdBareRepo(t *testing.T) {
	dir := initBareRepo(t, "clientA.git")
	out, err := execRoot(t, "template", "new", "bug", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt template new (bare repo): %v", err)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("output %q should mention the created template", out)
	}

	listOut, err := execRoot(t, "template", "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt template list (bare repo): %v", err)
	}
	if !strings.Contains(listOut, "bug") {
		t.Errorf("output %q should list the bug template", listOut)
	}
}

func TestNewCmdBareRepoEndToEnd(t *testing.T) {
	dir := initBareRepo(t, "clientA.git")
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}

	out, err := execRoot(t, "new", "bug/dboverflow", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt new (bare repo): %v", err)
	}
	if !strings.Contains(out, "bug/dboverflow") {
		t.Errorf("output %q should mention the created issue", out)
	}

	// Verify the sibling worktree convention strips the .git suffix:
	// clientA.git -> clientA.worktrees, not clientA.git.worktrees.
	wantWorktree := git.ComputeWorktreePath(dir, "", "bug", "dboverflow")
	if !strings.Contains(out, wantWorktree) {
		t.Errorf("output %q should mention worktree path %q", out, wantWorktree)
	}
	if !strings.Contains(filepath.Base(filepath.Dir(filepath.Dir(wantWorktree))), "clientA.worktrees") {
		t.Errorf("worktree path %q should live under a clientA.worktrees sibling, not clientA.git.worktrees", wantWorktree)
	}
	if _, err := os.Stat(filepath.Join(wantWorktree, "README.md")); err != nil {
		t.Errorf("expected a real checked-out worktree at %s: %v", wantWorktree, err)
	}
}

func TestListCmdBareRepoEndToEnd(t *testing.T) {
	dir := initBareRepo(t, "clientA.git")
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/dboverflow", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	out, err := execRoot(t, "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list (bare repo): %v", err)
	}
	if !strings.Contains(out, "bug/dboverflow") {
		t.Errorf("output %q should list bug/dboverflow", out)
	}

	jsonOut, err := execRoot(t, "list", "--json", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list --json (bare repo): %v", err)
	}
	var issues []map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &issues); err != nil {
		t.Fatalf("--json output invalid: %v", err)
	}
	if len(issues) != 1 || issues[0]["branch"] != "bug/dboverflow" {
		t.Errorf("decoded JSON = %+v, want one issue bug/dboverflow", issues)
	}
}

func TestCloseCmdBareRepoEndToEnd(t *testing.T) {
	// Close/reopen are pure plumbing too -- confirm they work against a
	// bare target repo just like new/list do.
	dir := initBareRepo(t, "clientA.git")
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/dboverflow", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	if _, err := execRoot(t, "close", "bug/dboverflow", "--repo", dir); err != nil {
		t.Fatalf("pmt close (bare repo): %v", err)
	}
	if _, err := execRoot(t, "reopen", "bug/dboverflow", "--repo", dir); err != nil {
		t.Fatalf("pmt reopen (bare repo): %v", err)
	}

	out, err := execRoot(t, "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if !strings.Contains(out, "bug/dboverflow") {
		t.Errorf("output %q should list the reopened issue", out)
	}
}

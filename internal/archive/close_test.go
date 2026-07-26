package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
)

func TestCloseEndToEnd(t *testing.T) {
	dir, result := repoWithIssue(t, "dboverflow")

	closeResult, err := Close(dir, defaultCfg(), "bug", "dboverflow")
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closeResult.Branch != "bug/dboverflow" {
		t.Errorf("Branch = %q, want bug/dboverflow", closeResult.Branch)
	}
	if !closeResult.WorktreeRemoved {
		t.Error("expected WorktreeRemoved=true")
	}

	exists, err := git.RefExists(dir, "refs/heads/bug/dboverflow")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("issue branch should be deleted after Close")
	}
	if _, err := os.Stat(result.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree dir should be removed after Close, stat err = %v", err)
	}

	content, err := git.ReadBlob(dir, closeResult.ArchiveCommit+":bug/dboverflow/README.md")
	if err != nil {
		t.Fatalf("ReadBlob archived README: %v", err)
	}
	meta, _, ok := issue.Parse(content)
	if !ok {
		t.Fatalf("archived README.md should parse:\n%s", content)
	}
	if meta.Status != "closed" {
		t.Errorf("Status = %q, want closed", meta.Status)
	}
	if _, err := time.Parse(time.RFC3339, meta.Closed); err != nil {
		t.Errorf("Closed = %q is not a valid RFC3339 timestamp: %v", meta.Closed, err)
	}
}

func TestCloseNonexistentBranch(t *testing.T) {
	dir := initRepo(t)
	_, err := Close(dir, defaultCfg(), "bug", "never-existed")
	if err == nil {
		t.Fatal("expected error closing a branch that doesn't exist")
	}
}

func TestCloseDirtyWorktreeRefused(t *testing.T) {
	dir, result := repoWithIssue(t, "dboverflow")
	if err := os.WriteFile(filepath.Join(result.WorktreePath, "README.md"), []byte("uncommitted change"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Close(dir, defaultCfg(), "bug", "dboverflow")
	if err != ErrDirtyWorktree {
		t.Fatalf("Close: got %v, want ErrDirtyWorktree", err)
	}

	// nothing should have been destroyed
	exists, existsErr := git.RefExists(dir, "refs/heads/bug/dboverflow")
	if existsErr != nil {
		t.Fatal(existsErr)
	}
	if !exists {
		t.Error("branch should still exist after a refused close")
	}
	if _, err := os.Stat(result.WorktreePath); err != nil {
		t.Errorf("worktree should still exist after a refused close: %v", err)
	}
}

func TestClosePrunableWorktree(t *testing.T) {
	dir, result := repoWithIssue(t, "dboverflow")
	if err := os.RemoveAll(result.WorktreePath); err != nil {
		t.Fatal(err)
	}

	closeResult, err := Close(dir, defaultCfg(), "bug", "dboverflow")
	if err != nil {
		t.Fatalf("Close (prunable worktree): %v", err)
	}
	if !closeResult.WorktreeRemoved {
		t.Error("expected WorktreeRemoved=true even for a prunable (dir-already-gone) worktree")
	}

	exists, err := git.RefExists(dir, "refs/heads/bug/dboverflow")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("issue branch should be deleted after Close")
	}
	content, err := git.ReadBlob(dir, closeResult.ArchiveCommit+":bug/dboverflow/README.md")
	if err != nil {
		t.Fatalf("ReadBlob archived README: %v", err)
	}
	if meta, _, ok := issue.Parse(content); !ok || meta.Status != "closed" {
		t.Errorf("archived README meta = %+v (ok=%v), want Status=closed", meta, ok)
	}
}

func TestCloseHandCreatedBranchNoWorktree(t *testing.T) {
	dir := initRepo(t)
	if _, err := git.Run(dir, "branch", "bug/handmade"); err != nil {
		t.Fatal(err)
	}
	// give it a minimal README so stamping has something to parse
	wt := filepath.Join(t.TempDir(), "wt")
	if err := git.WorktreeAdd(dir, wt, "bug/handmade"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "README.md"), []byte("# hand-made\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(wt, "add", "README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(wt, "commit", "-q", "-m", "add readme"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "worktree", "remove", wt); err != nil {
		t.Fatal(err) // fully removed -- never registered as far as Close is concerned afterward... actually it WAS registered; remove it so Close sees "not registered"
	}

	closeResult, err := Close(dir, defaultCfg(), "bug", "handmade")
	if err != nil {
		t.Fatalf("Close (never had a worktree at close time): %v", err)
	}
	if closeResult.WorktreeRemoved {
		t.Error("expected WorktreeRemoved=false: no worktree was registered at close time")
	}
	content, err := git.ReadBlob(dir, closeResult.ArchiveCommit+":bug/handmade/README.md")
	if err != nil {
		t.Fatalf("ReadBlob: %v", err)
	}
	if !strings.Contains(string(content), "closed") {
		t.Errorf("expected the plumbing-stamped README to show status: closed:\n%s", content)
	}
}

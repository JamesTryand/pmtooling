package archive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := git.Run(dir, "init", "-q"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := git.Run(dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "config", "user.name", "pmt test"); err != nil {
		t.Fatal(err)
	}
	// A real target repo always has commits on its default branch; this
	// also gives `master` a valid HEAD so plain `git branch <name>` (used
	// by commitInWorktree below) has a start-point to default to.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# target repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "add", "README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "commit", "-q", "-m", "init"); err != nil {
		t.Fatal(err)
	}
	return dir
}

// commitOnDetachedHead makes one commit with the given file content,
// detached from any branch, and returns the commit SHA. Used to build
// standalone "issue" histories without disturbing whatever's checked
// out — these tests operate directly on the bare repo dir (no working
// tree of their own beyond git's default checkout), so we explicitly
// manage a temp worktree per commit sequence instead.
func commitInWorktree(t *testing.T, mainDir, worktreeDir, branch, filename, content string) string {
	t.Helper()
	if _, err := git.Run(mainDir, "branch", branch); err != nil {
		t.Fatalf("git branch %s: %v", branch, err)
	}
	if err := git.WorktreeAdd(mainDir, worktreeDir, branch); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreeDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(worktreeDir, "add", filename); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(worktreeDir, "commit", "-q", "-m", "commit for "+branch); err != nil {
		t.Fatal(err)
	}
	tip, err := git.Run(worktreeDir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return tip
}

func addCommitInWorktree(t *testing.T, worktreeDir, filename, content, message string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(worktreeDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(worktreeDir, "add", filename); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(worktreeDir, "commit", "-q", "-m", message); err != nil {
		t.Fatal(err)
	}
	tip, err := git.Run(worktreeDir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return tip
}

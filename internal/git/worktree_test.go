package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeWorktreePathDefault(t *testing.T) {
	mainRoot := filepath.Join(t.TempDir(), "clientA")
	got := ComputeWorktreePath(mainRoot, "", "bug", "dboverflow")
	want := filepath.Join(filepath.Dir(mainRoot), "clientA.worktrees", "bug", "dboverflow")
	if got != want {
		t.Errorf("ComputeWorktreePath = %q, want %q", got, want)
	}
}

func TestComputeWorktreePathOverride(t *testing.T) {
	mainRoot := filepath.Join(t.TempDir(), "clientA")
	got := ComputeWorktreePath(mainRoot, "../custom.worktrees", "bug", "dboverflow")
	want := filepath.Join(filepath.Dir(mainRoot), "custom.worktrees", "bug", "dboverflow")
	if got != want {
		t.Errorf("ComputeWorktreePath (override) = %q, want %q", got, want)
	}
}

func TestComputeWorktreePathStripsGitSuffixForBareRepoConvention(t *testing.T) {
	mainRoot := filepath.Join(t.TempDir(), "clientA.git")
	got := ComputeWorktreePath(mainRoot, "", "bug", "dboverflow")
	want := filepath.Join(filepath.Dir(mainRoot), "clientA.worktrees", "bug", "dboverflow")
	if got != want {
		t.Errorf("ComputeWorktreePath = %q, want %q (clientA.worktrees, not clientA.git.worktrees)", got, want)
	}
}

func TestWorktreeAdd(t *testing.T) {
	mainDir := initRepo(t)
	commitFile(t, mainDir, "f.txt", "hello")
	if _, err := Run(mainDir, "branch", "bug/0001"); err != nil {
		t.Fatalf("git branch: %v", err)
	}

	worktreePath := filepath.Join(t.TempDir(), "bug", "0001")
	if err := WorktreeAdd(mainDir, worktreePath, "bug/0001"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	if _, err := os.Stat(filepath.Join(worktreePath, "f.txt")); err != nil {
		t.Errorf("expected checked-out file in new worktree: %v", err)
	}

	out, err := Run(mainDir, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("worktree list: %v", err)
	}
	if !strings.Contains(out, "bug/0001") {
		t.Errorf("`git worktree list` should mention the new branch:\n%s", out)
	}
}

func TestIsWorktreeDirtyClean(t *testing.T) {
	mainDir := initRepo(t)
	commitFile(t, mainDir, "f.txt", "hello")
	if _, err := Run(mainDir, "branch", "bug/0001"); err != nil {
		t.Fatal(err)
	}
	worktreePath := filepath.Join(t.TempDir(), "bug", "0001")
	if err := WorktreeAdd(mainDir, worktreePath, "bug/0001"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	dirty, err := IsWorktreeDirty(worktreePath)
	if err != nil {
		t.Fatalf("IsWorktreeDirty: %v", err)
	}
	if dirty {
		t.Error("freshly checked-out worktree should not be dirty")
	}
}

func TestIsWorktreeDirtyWithUncommittedChanges(t *testing.T) {
	mainDir := initRepo(t)
	commitFile(t, mainDir, "f.txt", "hello")
	if _, err := Run(mainDir, "branch", "bug/0001"); err != nil {
		t.Fatal(err)
	}
	worktreePath := filepath.Join(t.TempDir(), "bug", "0001")
	if err := WorktreeAdd(mainDir, worktreePath, "bug/0001"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "f.txt"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty, err := IsWorktreeDirty(worktreePath)
	if err != nil {
		t.Fatalf("IsWorktreeDirty: %v", err)
	}
	if !dirty {
		t.Error("worktree with an uncommitted modification should be dirty")
	}
}

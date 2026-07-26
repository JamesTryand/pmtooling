package git

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// assertMainRoot verifies root resolves to the same repo as wantDir by
// statting a marker file, rather than comparing path strings — sidesteps
// symlink/case-normalization differences between git's output and t.TempDir().
func assertMainRoot(t *testing.T, root, wantDir string) {
	t.Helper()
	marker := "marker-" + t.Name() + ".txt"
	if err := os.WriteFile(filepath.Join(wantDir, marker), []byte("x"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, marker)); err != nil {
		t.Errorf("MainRoot() = %q, does not resolve to expected repo %q: %v", root, wantDir, err)
	}
}

func TestDiscoverMainCheckout(t *testing.T) {
	dir := initRepo(t)

	info, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if info.InLinkedWorktree() {
		t.Errorf("main checkout reported as a linked worktree")
	}
	if info.IsBare {
		t.Errorf("non-bare repo reported as bare")
	}
	assertMainRoot(t, info.MainRoot(), dir)
}

func TestDiscoverNotARepo(t *testing.T) {
	dir := t.TempDir() // deliberately not git-init'd
	_, err := Discover(dir)
	if !errors.Is(err, ErrNotARepo) {
		t.Fatalf("Discover in non-repo dir: got %v, want ErrNotARepo", err)
	}
}

func TestDiscoverLinkedWorktree(t *testing.T) {
	mainDir := initRepo(t)
	commitFile(t, mainDir, "f.txt", "hello")

	worktreeDir := filepath.Join(t.TempDir(), "wt")
	if _, err := Run(mainDir, "worktree", "add", "-b", "feature/x", worktreeDir); err != nil {
		t.Fatalf("git worktree add: %v", err)
	}

	info, err := Discover(worktreeDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !info.InLinkedWorktree() {
		t.Errorf("expected linked worktree, GitDir == GitCommonDir (%q)", info.GitDir)
	}
	// The whole point of MainRoot(): pmt invoked from inside its own issue
	// worktree must still resolve back to the main repo checkout.
	assertMainRoot(t, info.MainRoot(), mainDir)
}

func TestDiscoverBareRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := Run(dir, "init", "-q", "--bare"); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	info, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !info.IsBare {
		t.Errorf("expected IsBare == true for a bare repo")
	}
	// For a bare repo, MainRoot() must be the repo itself, not its parent
	// (filepath.Dir(GitCommonDir) would incorrectly return the parent,
	// since GitCommonDir already *is* the bare repo's own path).
	assertMainRoot(t, info.MainRoot(), dir)
}

// TestDiscoverBareRepoFromLinkedWorktree covers the case that broke a
// naive "IsBare = whatever rev-parse says for cwd" implementation:
// --is-bare-repository reports false from inside a linked worktree
// (which always has a real working tree) even though the repo it
// belongs to is bare. IsBare/MainRoot must still resolve correctly.
func TestDiscoverBareRepoFromLinkedWorktree(t *testing.T) {
	bareDir := t.TempDir()
	if _, err := Run(bareDir, "init", "-q", "--bare"); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	blob, err := HashObject(bareDir, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := Mktree(bareDir, []TreeEntry{{Mode: "100644", Type: "blob", SHA: blob, Name: "f.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	commit, err := CommitTree(bareDir, tree, "init")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateRef(bareDir, "refs/heads/master", commit); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(bareDir, "branch", "bug/one", "refs/heads/master"); err != nil {
		t.Fatal(err)
	}

	worktreeDir := filepath.Join(t.TempDir(), "wt")
	if err := WorktreeAdd(bareDir, worktreeDir, "bug/one"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	info, err := Discover(worktreeDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !info.InLinkedWorktree() {
		t.Errorf("expected linked worktree, GitDir == GitCommonDir (%q)", info.GitDir)
	}
	if !info.IsBare {
		t.Errorf("IsBare = false, want true: the main repo is bare even though this invocation is from inside a (necessarily non-bare) linked worktree")
	}
	// The whole point of this test: MainRoot() must resolve to the bare
	// repo itself, not its parent, even when invoked from inside a
	// linked worktree.
	assertMainRoot(t, info.MainRoot(), bareDir)
}

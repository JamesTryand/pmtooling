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
}

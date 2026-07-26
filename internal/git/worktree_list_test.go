package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListWorktreesBasic(t *testing.T) {
	mainDir := initRepo(t)
	commitFile(t, mainDir, "f.txt", "hello")
	if _, err := Run(mainDir, "branch", "bug/one"); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(mainDir, "branch", "bug/two"); err != nil {
		t.Fatal(err)
	}

	wt1 := filepath.Join(t.TempDir(), "wt1")
	wt2 := filepath.Join(t.TempDir(), "wt2")
	if err := WorktreeAdd(mainDir, wt1, "bug/one"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if err := WorktreeAdd(mainDir, wt2, "bug/two"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	worktrees, err := ListWorktrees(mainDir)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(worktrees) != 3 { // main checkout + 2 linked worktrees
		t.Fatalf("ListWorktrees returned %d entries, want 3: %+v", len(worktrees), worktrees)
	}

	byBranch := map[string]Worktree{}
	for _, w := range worktrees {
		byBranch[w.Branch] = w
	}
	if w, ok := byBranch["bug/one"]; !ok || w.Prunable {
		t.Errorf("bug/one entry = %+v, want present and not prunable", w)
	}
	if w, ok := byBranch["bug/two"]; !ok || w.Prunable {
		t.Errorf("bug/two entry = %+v, want present and not prunable", w)
	}
}

func TestListWorktreesPrunable(t *testing.T) {
	mainDir := initRepo(t)
	commitFile(t, mainDir, "f.txt", "hello")
	if _, err := Run(mainDir, "branch", "bug/one"); err != nil {
		t.Fatal(err)
	}

	wtParent := t.TempDir()
	wt := filepath.Join(wtParent, "wt1")
	if err := WorktreeAdd(mainDir, wt, "bug/one"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if err := os.RemoveAll(wt); err != nil { // delete the dir without `git worktree remove`
		t.Fatal(err)
	}

	worktrees, err := ListWorktrees(mainDir)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	var found *Worktree
	for i := range worktrees {
		if worktrees[i].Branch == "bug/one" {
			found = &worktrees[i]
		}
	}
	if found == nil {
		t.Fatal("expected bug/one worktree entry to still be listed (as prunable)")
	}
	if !found.Prunable {
		t.Errorf("expected bug/one worktree to be marked Prunable after its directory was deleted")
	}
}

func TestParseWorktreeListEmpty(t *testing.T) {
	if got := parseWorktreeList(""); len(got) != 0 {
		t.Errorf("parseWorktreeList(\"\") = %+v, want empty", got)
	}
}

func TestParseWorktreeListDetached(t *testing.T) {
	input := "worktree /path/to/repo\n" +
		"HEAD abc123\n" +
		"detached\n"
	got := parseWorktreeList(input)
	if len(got) != 1 {
		t.Fatalf("parseWorktreeList = %+v, want 1 entry", got)
	}
	if got[0].Branch != "" {
		t.Errorf("detached entry Branch = %q, want empty", got[0].Branch)
	}
	if want := filepath.FromSlash("/path/to/repo"); got[0].Path != want {
		t.Errorf("Path = %q, want %q", got[0].Path, want)
	}
}

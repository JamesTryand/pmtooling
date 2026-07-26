package git

import (
	"strings"
	"testing"
)

func TestHashObject(t *testing.T) {
	dir := initRepo(t)
	sha, err := HashObject(dir, []byte("hello world"))
	if err != nil {
		t.Fatalf("HashObject: %v", err)
	}
	out, err := Run(dir, "cat-file", "-p", sha)
	if err != nil {
		t.Fatalf("cat-file: %v", err)
	}
	if out != "hello world" {
		t.Errorf("cat-file -p %s = %q, want %q", sha, out, "hello world")
	}
}

func TestMktree(t *testing.T) {
	dir := initRepo(t)
	shaA, err := HashObject(dir, []byte("alpha"))
	if err != nil {
		t.Fatal(err)
	}
	shaB, err := HashObject(dir, []byte("beta"))
	if err != nil {
		t.Fatal(err)
	}

	tree, err := Mktree(dir, []TreeEntry{
		{Mode: "100644", Type: "blob", SHA: shaA, Name: "alpha.txt"},
		{Mode: "100644", Type: "blob", SHA: shaB, Name: "beta.txt"},
	})
	if err != nil {
		t.Fatalf("Mktree: %v", err)
	}

	out, err := Run(dir, "ls-tree", tree)
	if err != nil {
		t.Fatalf("ls-tree: %v", err)
	}
	if !strings.Contains(out, "alpha.txt") || !strings.Contains(out, "beta.txt") {
		t.Errorf("ls-tree output missing entries: %s", out)
	}
}

func TestMktreeSortsEntries(t *testing.T) {
	dir := initRepo(t)
	shaA, err := HashObject(dir, []byte("alpha"))
	if err != nil {
		t.Fatal(err)
	}
	shaB, err := HashObject(dir, []byte("beta"))
	if err != nil {
		t.Fatal(err)
	}

	unsorted, err := Mktree(dir, []TreeEntry{
		{Mode: "100644", Type: "blob", SHA: shaB, Name: "zeta.txt"},
		{Mode: "100644", Type: "blob", SHA: shaA, Name: "alpha.txt"},
	})
	if err != nil {
		t.Fatalf("Mktree (unsorted input): %v", err)
	}
	sorted, err := Mktree(dir, []TreeEntry{
		{Mode: "100644", Type: "blob", SHA: shaA, Name: "alpha.txt"},
		{Mode: "100644", Type: "blob", SHA: shaB, Name: "zeta.txt"},
	})
	if err != nil {
		t.Fatalf("Mktree (sorted input): %v", err)
	}
	if unsorted != sorted {
		t.Errorf("Mktree order-dependent: unsorted=%s sorted=%s, want equal SHAs", unsorted, sorted)
	}
}

func TestCommitTreeOrphan(t *testing.T) {
	dir := initRepo(t)
	sha, err := HashObject(dir, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := Mktree(dir, []TreeEntry{{Mode: "100644", Type: "blob", SHA: sha, Name: "f.txt"}})
	if err != nil {
		t.Fatal(err)
	}

	commit, err := CommitTree(dir, tree, "orphan test")
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}

	out, err := Run(dir, "cat-file", "-p", commit)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "parent ") {
		t.Errorf("expected an orphan commit (no parent line), got:\n%s", out)
	}
}

func TestUpdateRefAndNestedTreeReadback(t *testing.T) {
	dir := initRepo(t)
	settingsSHA, err := HashObject(dir, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	readmeSHA, err := HashObject(dir, []byte("# template"))
	if err != nil {
		t.Fatal(err)
	}

	claudeTree, err := Mktree(dir, []TreeEntry{
		{Mode: "100644", Type: "blob", SHA: settingsSHA, Name: "settings.json"},
	})
	if err != nil {
		t.Fatalf("Mktree (subtree): %v", err)
	}
	rootTree, err := Mktree(dir, []TreeEntry{
		{Mode: "040000", Type: "tree", SHA: claudeTree, Name: ".claude"},
		{Mode: "100644", Type: "blob", SHA: readmeSHA, Name: "README.md"},
	})
	if err != nil {
		t.Fatalf("Mktree (root): %v", err)
	}
	commit, err := CommitTree(dir, rootTree, "pmt: initialize template 'bug'")
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}

	ref := "refs/heads/pmt/template/bug"
	if err := UpdateRef(dir, ref, commit); err != nil {
		t.Fatalf("UpdateRef: %v", err)
	}

	out, err := Run(dir, "show", "pmt/template/bug:.claude/settings.json")
	if err != nil {
		t.Fatalf("show nested blob: %v", err)
	}
	if out != "{}" {
		t.Errorf("nested blob content = %q, want {}", out)
	}
}

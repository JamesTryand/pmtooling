package git

import "testing"

func TestCheckRefFormat(t *testing.T) {
	valid := []string{"bug", "bug/dboverflow", "pmt/template/bug"}
	for _, v := range valid {
		if err := CheckRefFormat(v); err != nil {
			t.Errorf("CheckRefFormat(%q) = %v, want nil", v, err)
		}
	}

	invalid := []string{
		"", "bug//dboverflow", "bug/.dboverflow", "bug/dboverflow.lock",
		"-bug", "bug/db~overflow", "bug/db overflow", "bug/..",
	}
	for _, v := range invalid {
		if err := CheckRefFormat(v); err == nil {
			t.Errorf("CheckRefFormat(%q) = nil, want error", v)
		}
	}
}

// TestTemplateRefNamespaceAvoidsCollision is a regression test for the core
// design decision in doc/templates.md: template branches must live under
// refs/heads/pmt/template/<type>, not a bare <type> branch, because a bare
// `bug` branch and a `bug/dboverflow` branch cannot coexist (git's ref
// storage is hierarchical). If this test starts failing, the reasoning
// behind that decision needs to be revisited.
func TestBareAndSlashedBranchNamesCollide(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "hello")

	if _, err := Run(dir, "branch", "bug"); err != nil {
		t.Fatalf("git branch bug: %v", err)
	}
	if _, err := Run(dir, "branch", "bug/dboverflow"); err == nil {
		t.Fatal("expected `git branch bug/dboverflow` to fail while branch `bug` exists (ref D/F conflict)")
	}
}

func TestTemplateNamespaceDoesNotCollideWithIssueBranch(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "hello")

	if _, err := Run(dir, "branch", "pmt/template/bug"); err != nil {
		t.Fatalf("git branch pmt/template/bug: %v", err)
	}
	if _, err := Run(dir, "branch", "bug/dboverflow"); err != nil {
		t.Fatalf("git branch bug/dboverflow should succeed alongside pmt/template/bug: %v", err)
	}
}

func TestRefExists(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "hello")
	if _, err := Run(dir, "branch", "bug/one"); err != nil {
		t.Fatalf("git branch: %v", err)
	}

	exists, err := RefExists(dir, "refs/heads/bug/one")
	if err != nil {
		t.Fatalf("RefExists: %v", err)
	}
	if !exists {
		t.Error("expected refs/heads/bug/one to exist")
	}

	exists, err = RefExists(dir, "refs/heads/bug/nope")
	if err != nil {
		t.Fatalf("RefExists: %v", err)
	}
	if exists {
		t.Error("expected refs/heads/bug/nope to not exist")
	}
}

func TestForEachRef(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "hello")
	for _, b := range []string{"bug/one", "bug/two", "pmt/template/bug"} {
		if _, err := Run(dir, "branch", b); err != nil {
			t.Fatalf("git branch %s: %v", b, err)
		}
	}

	refs, err := ForEachRef(dir, "refs/heads/bug/*", "%(refname:short)")
	if err != nil {
		t.Fatalf("ForEachRef: %v", err)
	}
	want := map[string]bool{"bug/one": true, "bug/two": true}
	if len(refs) != len(want) {
		t.Fatalf("ForEachRef returned %v, want 2 entries matching %v", refs, want)
	}
	for _, r := range refs {
		if !want[r] {
			t.Errorf("unexpected ref %q", r)
		}
	}
}

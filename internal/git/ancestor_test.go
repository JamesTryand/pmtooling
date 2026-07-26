package git

import "testing"

func TestIsAncestorSelf(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "v1")
	head, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	ok, err := IsAncestor(dir, head, head)
	if err != nil {
		t.Fatalf("IsAncestor: %v", err)
	}
	if !ok {
		t.Error("a commit should be its own ancestor")
	}
}

func TestIsAncestorTrue(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "v1")
	older, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, "f.txt", "v2")
	newer, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	ok, err := IsAncestor(dir, older, newer)
	if err != nil {
		t.Fatalf("IsAncestor: %v", err)
	}
	if !ok {
		t.Error("older commit should be an ancestor of newer")
	}
}

func TestIsAncestorDivergedIsFalseBothWays(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "base")
	base, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, "branch", "branch-a", base); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, "branch", "branch-b", base); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, "checkout", "-q", "branch-a"); err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, "a.txt", "a")
	tipA, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, "checkout", "-q", "branch-b"); err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, "b.txt", "b")
	tipB, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	if ok, err := IsAncestor(dir, tipA, tipB); err != nil || ok {
		t.Errorf("IsAncestor(a, b) = (%v, %v), want (false, nil) for diverged branches", ok, err)
	}
	if ok, err := IsAncestor(dir, tipB, tipA); err != nil || ok {
		t.Errorf("IsAncestor(b, a) = (%v, %v), want (false, nil) for diverged branches", ok, err)
	}
}

func TestIsAncestorInvalidRevisionIsAnError(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "v1")
	head, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	_, err = IsAncestor(dir, head, "not-a-real-revision")
	if err == nil {
		t.Fatal("expected an error for an invalid revision, not a false answer")
	}
}

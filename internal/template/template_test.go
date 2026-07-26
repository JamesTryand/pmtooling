package template

import (
	"errors"
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
	return dir
}

func TestRefFor(t *testing.T) {
	if got, want := RefFor("bug"), "refs/heads/pmt/template/bug"; got != want {
		t.Errorf("RefFor(bug) = %q, want %q", got, want)
	}
}

func TestNewOnFreshRepoWithNoCommits(t *testing.T) {
	dir := initRepo(t) // deliberately zero commits — commit-tree needs no HEAD/parent
	if _, err := New(dir, "bug"); err != nil {
		t.Fatalf("New: %v", err)
	}
	exists, err := Exists(dir, "bug")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected template 'bug' to exist after New")
	}
}

func TestNewDoesNotMutateWorkingTreeOrHead(t *testing.T) {
	dir := initRepo(t)
	headBefore, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := New(dir, "bug"); err != nil {
		t.Fatalf("New: %v", err)
	}

	headAfter, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	if string(headBefore) != string(headAfter) {
		t.Errorf("HEAD changed: before=%q after=%q", headBefore, headAfter)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != ".git" {
			t.Errorf("New wrote %q into the working tree; it must only touch git objects/refs", e.Name())
		}
	}
}

func TestNewCollision(t *testing.T) {
	dir := initRepo(t)
	if _, err := New(dir, "bug"); err != nil {
		t.Fatalf("New (first): %v", err)
	}
	_, err := New(dir, "bug")
	if !errors.Is(err, ErrExists) {
		t.Fatalf("New (second): got %v, want ErrExists", err)
	}
}

func TestNewInvalidName(t *testing.T) {
	dir := initRepo(t)
	if _, err := New(dir, "bug/sub"); err == nil {
		t.Fatal("expected error for a template name containing '/'")
	}
}

func TestExistsBeforeAndAfter(t *testing.T) {
	dir := initRepo(t)
	exists, err := Exists(dir, "bug")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected 'bug' to not exist before New")
	}

	if _, err := New(dir, "bug"); err != nil {
		t.Fatalf("New: %v", err)
	}
	exists, err = Exists(dir, "bug")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected 'bug' to exist after New")
	}
}

func TestList(t *testing.T) {
	dir := initRepo(t)
	for _, name := range []string{"feature", "bug", "chore"} {
		if _, err := New(dir, name); err != nil {
			t.Fatalf("New(%s): %v", name, err)
		}
	}
	names, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"bug", "chore", "feature"}
	if len(names) != len(want) {
		t.Fatalf("List = %v, want %v", names, want)
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("List[%d] = %q, want %q (List should be sorted)", i, names[i], w)
		}
	}
}

func TestListEmptyWhenNoTemplates(t *testing.T) {
	dir := initRepo(t)
	names, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("List = %v, want empty", names)
	}
}

func TestNewScaffoldReadback(t *testing.T) {
	dir := initRepo(t)
	if _, err := New(dir, "bug"); err != nil {
		t.Fatalf("New: %v", err)
	}

	settings, err := git.Run(dir, "show", "pmt/template/bug:.claude/settings.json")
	if err != nil {
		t.Fatalf("show .claude/settings.json: %v", err)
	}
	if settings != "{}" {
		t.Errorf(".claude/settings.json = %q, want {}", settings)
	}

	readme, err := git.Run(dir, "show", "pmt/template/bug:README.md")
	if err != nil {
		t.Fatalf("show README.md: %v", err)
	}
	if len(readme) == 0 {
		t.Error("README.md is empty")
	}

	for _, path := range []string{"CLAUDE.md", ".gitignore"} {
		if _, err := git.Run(dir, "show", "pmt/template/bug:"+path); err != nil {
			t.Errorf("show %s: %v", path, err)
		}
	}
}

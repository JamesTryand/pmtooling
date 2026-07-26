package issue

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
	return dir
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if _, err := git.Run(dir, "add", name); err != nil {
		t.Fatalf("git add %s: %v", name, err)
	}
	if _, err := git.Run(dir, "commit", "-q", "-m", "add "+name); err != nil {
		t.Fatalf("git commit %s: %v", name, err)
	}
}

func makeBranch(t *testing.T, dir, name string) {
	t.Helper()
	if _, err := git.Run(dir, "branch", name); err != nil {
		t.Fatalf("git branch %s: %v", name, err)
	}
}

package git

import (
	"os"
	"path/filepath"
	"testing"
)

// initRepo creates a scratch git repository in a fresh t.TempDir(), never
// touching the pmtooling repo itself.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := Run(dir, "init", "-q"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := Run(dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if _, err := Run(dir, "config", "user.name", "pmt test"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	return dir
}

// commitFile writes a file with the given content to dir and commits it,
// so the repo has at least one commit (required for `worktree add -b`).
func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if _, err := Run(dir, "add", name); err != nil {
		t.Fatalf("git add %s: %v", name, err)
	}
	if _, err := Run(dir, "commit", "-q", "-m", "add "+name); err != nil {
		t.Fatalf("git commit %s: %v", name, err)
	}
}

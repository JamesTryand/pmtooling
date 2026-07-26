package issue

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

func TestStampReadmeInWorktree(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "README.md", "---\npmt:\n  status: open\n---\n\nbody\n")
	if _, err := git.Run(dir, "branch", "bug/one"); err != nil {
		t.Fatal(err)
	}
	worktreePath := filepath.Join(t.TempDir(), "wt")
	if err := git.WorktreeAdd(dir, worktreePath, "bug/one"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	sha, err := StampReadmeInWorktree(worktreePath, "pmt: test stamp", func(meta *Meta) {
		meta.Status = "closed"
		meta.Closed = "2026-07-26T00:00:00Z"
	})
	if err != nil {
		t.Fatalf("StampReadmeInWorktree: %v", err)
	}
	if sha == "" {
		t.Fatal("expected a non-empty commit SHA")
	}

	log, err := git.Run(worktreePath, "log", "--oneline", "-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "pmt: test stamp") {
		t.Errorf("expected the stamp commit message, got: %q", log)
	}

	content, err := git.ReadBlob(worktreePath, "HEAD:README.md")
	if err != nil {
		t.Fatal(err)
	}
	meta, body, ok := Parse(content)
	if !ok {
		t.Fatalf("stamped README.md should still parse:\n%s", content)
	}
	if meta.Status != "closed" || meta.Closed != "2026-07-26T00:00:00Z" {
		t.Errorf("meta = %+v, want Status=closed Closed=2026-07-26T00:00:00Z", meta)
	}
	if !strings.Contains(body, "body") {
		t.Errorf("body should be preserved: %q", body)
	}
}

func TestStampReadmeViaPlumbingDoesNotNeedAWorktree(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "README.md", "---\npmt:\n  status: open\n---\n\nbody\n")
	commitFile(t, dir, "CLAUDE.md", "sibling file, should be untouched\n")
	if _, err := git.Run(dir, "branch", "bug/one"); err != nil {
		t.Fatal(err)
	}
	// deliberately never create a worktree for bug/one

	sha, err := StampReadmeViaPlumbing(dir, "refs/heads/bug/one", "pmt: test plumbing stamp", func(meta *Meta) {
		meta.Status = "closed"
	})
	if err != nil {
		t.Fatalf("StampReadmeViaPlumbing: %v", err)
	}

	content, err := git.ReadBlob(dir, sha+":README.md")
	if err != nil {
		t.Fatal(err)
	}
	meta, _, ok := Parse(content)
	if !ok || meta.Status != "closed" {
		t.Errorf("stamped commit's README.md meta = %+v (ok=%v), want Status=closed", meta, ok)
	}

	claude, err := git.ReadBlob(dir, sha+":CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(claude) != "sibling file, should be untouched\n" {
		t.Errorf("CLAUDE.md should be preserved unchanged: %q", claude)
	}

	// The branch ref itself is untouched -- StampReadmeViaPlumbing never
	// moves it, only returns the new commit SHA.
	branchTip, err := git.Run(dir, "rev-parse", "refs/heads/bug/one")
	if err != nil {
		t.Fatal(err)
	}
	if branchTip == sha {
		t.Error("StampReadmeViaPlumbing should not move refs/heads/bug/one")
	}
}

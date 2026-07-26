package archive

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
)

func TestReopenEndToEnd(t *testing.T) {
	dir, created := repoWithIssue(t, "dboverflow")
	if err := os.WriteFile(filepath.Join(created.WorktreePath, "work.txt"), []byte("pre-close work"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(created.WorktreePath, "add", "work.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(created.WorktreePath, "commit", "-q", "-m", "pre-close work commit"); err != nil {
		t.Fatal(err)
	}

	if _, err := Close(dir, defaultCfg(), "bug", "dboverflow"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopenResult, err := Reopen(dir, defaultCfg(), "bug", "dboverflow")
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if reopenResult.Branch != "bug/dboverflow" {
		t.Errorf("Branch = %q, want bug/dboverflow", reopenResult.Branch)
	}

	exists, err := git.RefExists(dir, "refs/heads/bug/dboverflow")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("branch should exist after Reopen")
	}
	if _, err := os.Stat(reopenResult.WorktreePath); err != nil {
		t.Errorf("worktree should exist after Reopen: %v", err)
	}

	// Full pre-close history must be intact.
	log, err := git.Run(reopenResult.WorktreePath, "log", "--oneline")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "pre-close work commit") {
		t.Errorf("reopened branch history missing pre-close commit:\n%s", log)
	}
	if !strings.Contains(log, "pmt: initialize issue") {
		t.Errorf("reopened branch history missing original init commit:\n%s", log)
	}

	content, err := git.ReadBlob(reopenResult.WorktreePath, "HEAD:README.md")
	if err != nil {
		t.Fatal(err)
	}
	meta, _, ok := issue.Parse(content)
	if !ok {
		t.Fatalf("reopened README.md should parse:\n%s", content)
	}
	if meta.Status != "open" {
		t.Errorf("Status = %q, want open", meta.Status)
	}
	if meta.Closed != "" {
		t.Errorf("Closed = %q, want cleared", meta.Closed)
	}
}

func TestReopenNotArchived(t *testing.T) {
	dir := initRepo(t)
	_, err := Reopen(dir, defaultCfg(), "bug", "never-archived")
	if !errors.Is(err, ErrNotArchived) {
		t.Fatalf("Reopen: got %v, want ErrNotArchived", err)
	}
}

func TestReopenCollisionWithLiveBranch(t *testing.T) {
	dir, _ := repoWithIssue(t, "dboverflow")
	// bug/dboverflow is live (not closed) -- reopening it should refuse.
	_, err := Reopen(dir, defaultCfg(), "bug", "dboverflow")
	if err == nil {
		t.Fatal("expected error reopening a name that's already a live branch")
	}
}

func TestReopenThenRecloseViaFullAPIKeepsIssuesIndependent(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	if _, err := issue.Create(dir, defaultCfg(), "bug", "two"); err != nil {
		t.Fatalf("issue.Create(two): %v", err)
	}

	if _, err := Close(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatalf("Close(one): %v", err)
	}
	if _, err := Close(dir, defaultCfg(), "bug", "two"); err != nil {
		t.Fatalf("Close(two): %v", err)
	}

	reopened, err := Reopen(dir, defaultCfg(), "bug", "one")
	if err != nil {
		t.Fatalf("Reopen(one): %v", err)
	}
	if err := os.WriteFile(filepath.Join(reopened.WorktreePath, "more-work.txt"), []byte("more"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(reopened.WorktreePath, "add", "more-work.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(reopened.WorktreePath, "commit", "-q", "-m", "post-reopen work"); err != nil {
		t.Fatal(err)
	}

	recloseResult, err := Close(dir, defaultCfg(), "bug", "one")
	if err != nil {
		t.Fatalf("Close(one) again: %v", err)
	}

	// bug/two must be completely unaffected by bug/one's reopen+reclose cycle.
	twoContent, err := git.ReadBlob(dir, recloseResult.ArchiveCommit+":bug/two/README.md")
	if err != nil {
		t.Fatalf("ReadBlob bug/two: %v", err)
	}
	twoMeta, _, ok := issue.Parse(twoContent)
	if !ok || twoMeta.Branch != "bug/two" {
		t.Errorf("bug/two archived meta = %+v (ok=%v), want intact and independent", twoMeta, ok)
	}

	log, err := git.Run(dir, "log", "--oneline", recloseResult.ArchiveCommit+"^1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "post-reopen work") {
		t.Errorf("expected full history including post-reopen work in the re-close, got:\n%s", log)
	}
}

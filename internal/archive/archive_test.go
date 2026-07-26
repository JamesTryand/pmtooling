package archive

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

func TestArchiveIssueFirstClose(t *testing.T) {
	dir := initRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	tip := commitInWorktree(t, dir, wt, "bug/one", "f.txt", "hello")

	commit, err := archiveIssue(dir, "bug", "one", tip)
	if err != nil {
		t.Fatalf("archiveIssue: %v", err)
	}

	content, err := git.ReadBlob(dir, commit+":bug/one/f.txt")
	if err != nil {
		t.Fatalf("ReadBlob: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("archived content = %q, want hello", content)
	}

	parents, err := git.Run(dir, "log", "--format=%P", "-1", commit)
	if err != nil {
		t.Fatal(err)
	}
	if parents != tip {
		t.Errorf("first-ever archive commit should have exactly one parent (the issue tip): got %q, want %q", parents, tip)
	}
}

func TestArchiveIssueTwoIssuesIndependent(t *testing.T) {
	dir := initRepo(t)
	wt1 := filepath.Join(t.TempDir(), "wt1")
	wt2 := filepath.Join(t.TempDir(), "wt2")
	tip1 := commitInWorktree(t, dir, wt1, "bug/one", "f.txt", "one-content")
	tip2 := commitInWorktree(t, dir, wt2, "bug/two", "g.txt", "two-content")

	if _, err := archiveIssue(dir, "bug", "one", tip1); err != nil {
		t.Fatalf("archiveIssue(one): %v", err)
	}
	arc2, err := archiveIssue(dir, "bug", "two", tip2)
	if err != nil {
		t.Fatalf("archiveIssue(two): %v", err)
	}

	// arc2 must still contain bug/one's content, untouched.
	oneContent, err := git.ReadBlob(dir, arc2+":bug/one/f.txt")
	if err != nil {
		t.Fatalf("ReadBlob bug/one: %v", err)
	}
	if string(oneContent) != "one-content" {
		t.Errorf("bug/one content = %q, want one-content (should survive archiving bug/two)", oneContent)
	}
	twoContent, err := git.ReadBlob(dir, arc2+":bug/two/g.txt")
	if err != nil {
		t.Fatalf("ReadBlob bug/two: %v", err)
	}
	if string(twoContent) != "two-content" {
		t.Errorf("bug/two content = %q, want two-content", twoContent)
	}
}

func TestArchiveIssueReCloseAfterReopenUpdatesContentAndKeepsHistory(t *testing.T) {
	dir := initRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	tip1a := commitInWorktree(t, dir, wt, "bug/one", "f.txt", "v1")
	if _, err := archiveIssue(dir, "bug", "one", tip1a); err != nil {
		t.Fatalf("archiveIssue (first close): %v", err)
	}

	// "reopen": remove the worktree (archiveIssue never touches worktrees
	// — that's Close's job), recreate the branch at tip1a in a fresh
	// worktree dir, and add more commits.
	if _, err := git.Run(dir, "worktree", "remove", wt); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "branch", "-D", "bug/one"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "branch", "bug/one", tip1a); err != nil {
		t.Fatal(err)
	}
	wt2 := filepath.Join(t.TempDir(), "wt2")
	if err := git.WorktreeAdd(dir, wt2, "bug/one"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	tip1b := addCommitInWorktree(t, wt2, "f.txt", "v1\nv2 (post-reopen)", "bug/one commit after reopen")

	arc3, err := archiveIssue(dir, "bug", "one", tip1b)
	if err != nil {
		t.Fatalf("archiveIssue (second close): %v", err)
	}

	content, err := git.ReadBlob(dir, arc3+":bug/one/f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "v1\nv2 (post-reopen)" {
		t.Errorf("archived content after re-close = %q, want the updated content", content)
	}

	log, err := git.Run(dir, "log", "--oneline", arc3+"^1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "after reopen") || strings.Count(log, "\n")+1 < 2 {
		t.Errorf("expected full continuous history (pre- and post-reopen commits) via arc3^1, got:\n%s", log)
	}
}

func TestFindArchiveCommitFindsMostRecentClose(t *testing.T) {
	dir := initRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	tip1a := commitInWorktree(t, dir, wt, "bug/one", "f.txt", "v1")
	if _, err := archiveIssue(dir, "bug", "one", tip1a); err != nil {
		t.Fatal(err)
	}

	if _, err := git.Run(dir, "worktree", "remove", wt); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "branch", "-D", "bug/one"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "branch", "bug/one", tip1a); err != nil {
		t.Fatal(err)
	}
	wt2 := filepath.Join(t.TempDir(), "wt2")
	if err := git.WorktreeAdd(dir, wt2, "bug/one"); err != nil {
		t.Fatal(err)
	}
	tip1b := addCommitInWorktree(t, wt2, "f.txt", "v1\nv2", "second commit")
	arc3, err := archiveIssue(dir, "bug", "one", tip1b)
	if err != nil {
		t.Fatal(err)
	}

	found, err := findArchiveCommit(dir, "bug", "one")
	if err != nil {
		t.Fatalf("findArchiveCommit: %v", err)
	}
	if found != arc3 {
		t.Errorf("findArchiveCommit = %s, want the most recent close %s", found, arc3)
	}
	restoredTip, err := git.Run(dir, "rev-parse", found+"^1")
	if err != nil {
		t.Fatal(err)
	}
	if restoredTip != tip1b {
		t.Errorf("restored tip = %s, want %s (the most recent, not the stale first close)", restoredTip, tip1b)
	}
}

func TestFindArchiveCommitNotFound(t *testing.T) {
	dir := initRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	tip := commitInWorktree(t, dir, wt, "bug/one", "f.txt", "v1")
	if _, err := archiveIssue(dir, "bug", "one", tip); err != nil {
		t.Fatal(err)
	}

	_, err := findArchiveCommit(dir, "bug", "never-archived")
	if !errors.Is(err, ErrNotArchived) {
		t.Errorf("findArchiveCommit = %v, want ErrNotArchived", err)
	}
}

func TestFindArchiveCommitNoArchiveBranchYet(t *testing.T) {
	dir := initRepo(t)
	_, err := findArchiveCommit(dir, "bug", "one")
	if !errors.Is(err, ErrNotArchived) {
		t.Errorf("findArchiveCommit = %v, want ErrNotArchived", err)
	}
}

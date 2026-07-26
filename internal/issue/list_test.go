package issue

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/template"
)

func TestListIssuesBasic(t *testing.T) {
	dir := repoWithBugTemplate(t)
	if _, err := Create(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := Create(dir, defaultCfg(), "bug", "two"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	issues, err := ListIssues(dir, "", "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("ListIssues returned %d issues, want 2: %+v", len(issues), issues)
	}
	if issues[0].Branch != "bug/one" || issues[1].Branch != "bug/two" {
		t.Errorf("issues not sorted by branch: %+v", issues)
	}
	for _, iss := range issues {
		if iss.Unparseable {
			t.Errorf("%s: expected parseable front matter", iss.Branch)
		}
		if iss.Status != "open" {
			t.Errorf("%s: Status = %q, want open", iss.Branch, iss.Status)
		}
		if iss.Created == "" {
			t.Errorf("%s: Created is empty", iss.Branch)
		}
		if iss.WorktreeState != WorktreeOK {
			t.Errorf("%s: WorktreeState = %q, want ok", iss.Branch, iss.WorktreeState)
		}
		if iss.WorktreePath == "" {
			t.Errorf("%s: WorktreePath is empty", iss.Branch)
		}
	}
}

func TestListIssuesExcludesUnrelatedAndTemplateBranches(t *testing.T) {
	dir := repoWithBugTemplate(t)
	if _, err := Create(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	makeBranch(t, dir, "release/1.0")

	issues, err := ListIssues(dir, "", "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("ListIssues returned %+v, want only bug/one (release/1.0 and pmt/template/bug excluded)", issues)
	}
	if issues[0].Branch != "bug/one" {
		t.Errorf("Branch = %q, want bug/one", issues[0].Branch)
	}
}

func TestListIssuesTypeFilter(t *testing.T) {
	dir := repoWithBugTemplate(t)
	if _, err := template.New(dir, "feature"); err != nil {
		t.Fatalf("template.New(feature): %v", err)
	}
	if _, err := Create(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatalf("Create(bug): %v", err)
	}
	if _, err := Create(dir, defaultCfg(), "feature", "one"); err != nil {
		t.Fatalf("Create(feature): %v", err)
	}

	issues, err := ListIssues(dir, "", "bug")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 || issues[0].Branch != "bug/one" {
		t.Fatalf("ListIssues(--type bug) = %+v, want only bug/one", issues)
	}
}

func TestListIssuesPrunableWorktree(t *testing.T) {
	dir := repoWithBugTemplate(t)
	result, err := Create(dir, defaultCfg(), "bug", "one")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.RemoveAll(result.WorktreePath); err != nil {
		t.Fatal(err)
	}

	issues, err := ListIssues(dir, "", "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("ListIssues = %+v, want 1 entry", issues)
	}
	if issues[0].WorktreeState != WorktreePrunable {
		t.Errorf("WorktreeState = %q, want prunable", issues[0].WorktreeState)
	}
}

func TestListIssuesMissingWorktree(t *testing.T) {
	dir := repoWithBugTemplate(t)
	result, err := Create(dir, defaultCfg(), "bug", "one")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := git.Run(dir, "worktree", "remove", result.WorktreePath); err != nil {
		t.Fatalf("git worktree remove: %v", err)
	}

	issues, err := ListIssues(dir, "", "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("ListIssues = %+v, want 1 entry", issues)
	}
	if issues[0].WorktreeState != WorktreeMissing {
		t.Errorf("WorktreeState = %q, want missing", issues[0].WorktreeState)
	}
	if issues[0].WorktreePath != "" {
		t.Errorf("WorktreePath = %q, want empty when fully removed", issues[0].WorktreePath)
	}
}

func TestListIssuesOrphanedDirectory(t *testing.T) {
	dir := repoWithBugTemplate(t)
	// A branch matching the issue pattern exists, but no worktree was ever
	// created for it via Create/WorktreeAdd — instead, a stray directory
	// happens to sit at the exact path pmt would have used.
	makeBranch(t, dir, "bug/orphan")
	expected := git.ComputeWorktreePath(dir, "", "bug", "orphan")
	if err := os.MkdirAll(expected, 0o755); err != nil {
		t.Fatal(err)
	}

	issues, err := ListIssues(dir, "", "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("ListIssues = %+v, want 1 entry", issues)
	}
	if issues[0].WorktreeState != WorktreeOrphaned {
		t.Errorf("WorktreeState = %q, want orphaned", issues[0].WorktreeState)
	}
	if issues[0].WorktreePath != expected {
		t.Errorf("WorktreePath = %q, want %q", issues[0].WorktreePath, expected)
	}
}

func TestListIssuesUnparseableReadmeIsNonFatal(t *testing.T) {
	dir := repoWithBugTemplate(t)
	result, err := Create(dir, defaultCfg(), "bug", "one")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Corrupt README.md on the branch itself (not just the working tree)
	// so `git show branch:README.md` sees the corruption.
	readmePath := filepath.Join(result.WorktreePath, "README.md")
	if err := os.WriteFile(readmePath, []byte("---\npmt: [not a map\n---\nbroken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(result.WorktreePath, "add", "README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(result.WorktreePath, "commit", "-q", "-m", "corrupt readme"); err != nil {
		t.Fatal(err)
	}

	// Also create a second, healthy issue, to confirm one corrupted issue
	// doesn't prevent the rest of the list from rendering.
	if _, err := Create(dir, defaultCfg(), "bug", "two"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	issues, err := ListIssues(dir, "", "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("ListIssues = %+v, want 2 entries", issues)
	}
	if !issues[0].Unparseable {
		t.Errorf("bug/one: expected Unparseable=true, got %+v", issues[0])
	}
	if issues[1].Unparseable {
		t.Errorf("bug/two: expected Unparseable=false, got %+v", issues[1])
	}
}

func TestListIssuesEmptyRepo(t *testing.T) {
	dir := repoWithBugTemplate(t) // template exists, but no issues created
	issues, err := ListIssues(dir, "", "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("ListIssues = %+v, want empty", issues)
	}
}

func TestListIssuesWorktreesDirOverride(t *testing.T) {
	dir := repoWithBugTemplate(t)
	cfg := config.RepoConfig{TitlePadWidth: config.DefaultTitlePadWidth, WorktreesDir: "../custom.worktrees"}
	result, err := Create(dir, cfg, "bug", "one")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	issues, err := ListIssues(dir, cfg.WorktreesDir, "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 || issues[0].WorktreePath != result.WorktreePath {
		t.Errorf("ListIssues = %+v, want WorktreePath %q", issues, result.WorktreePath)
	}
}

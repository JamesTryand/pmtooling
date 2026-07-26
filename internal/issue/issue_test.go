package issue

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/template"
)

func repoWithBugTemplate(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	// A real target repo always has commits on its default branch; this
	// also gives `master` a valid HEAD so plain `git branch <name>` (used
	// by makeBranch in these tests) has a start-point to default to.
	// template.New itself works fine on a totally empty repo — that's
	// covered separately in internal/template's own tests.
	commitFile(t, dir, "README.md", "# target repo\n")
	if _, err := template.New(dir, "bug"); err != nil {
		t.Fatalf("template.New: %v", err)
	}
	return dir
}

func defaultCfg() config.RepoConfig {
	return config.RepoConfig{TitlePadWidth: config.DefaultTitlePadWidth}
}

func TestCreateEndToEnd(t *testing.T) {
	dir := repoWithBugTemplate(t)

	result, err := Create(dir, defaultCfg(), "bug", "dboverflow")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Branch != "bug/dboverflow" {
		t.Errorf("Branch = %q, want bug/dboverflow", result.Branch)
	}

	wantWorktree := git.ComputeWorktreePath(dir, "", "bug", "dboverflow")
	if result.WorktreePath != wantWorktree {
		t.Errorf("WorktreePath = %q, want %q", result.WorktreePath, wantWorktree)
	}
	if _, err := os.Stat(filepath.Join(result.WorktreePath, "README.md")); err != nil {
		t.Errorf("expected README.md in worktree: %v", err)
	}

	exists, err := git.RefExists(dir, "refs/heads/bug/dboverflow")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected branch bug/dboverflow to exist")
	}

	log, err := git.Run(result.WorktreePath, "log", "--oneline", "-1")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(log, "pmt: initialize issue bug/dboverflow") {
		t.Errorf("expected the stamping commit message, got: %q", log)
	}
}

func TestCreateReadmeFrontMatterStamped(t *testing.T) {
	dir := repoWithBugTemplate(t)
	result, err := Create(dir, defaultCfg(), "bug", "dboverflow")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(result.WorktreePath, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	meta, _, ok := Parse(content)
	if !ok {
		t.Fatalf("stamped README.md should have parseable front matter:\n%s", content)
	}
	if meta.Type != "bug" {
		t.Errorf("Type = %q, want bug", meta.Type)
	}
	if meta.Title != "dboverflow" {
		t.Errorf("Title = %q, want dboverflow", meta.Title)
	}
	if meta.Branch != "bug/dboverflow" {
		t.Errorf("Branch = %q, want bug/dboverflow", meta.Branch)
	}
	if meta.Status != "open" {
		t.Errorf("Status = %q, want open", meta.Status)
	}
	if _, err := time.Parse(time.RFC3339, meta.Created); err != nil {
		t.Errorf("Created = %q is not a valid RFC3339 timestamp: %v", meta.Created, err)
	}
	templateCommit, err := git.Run(dir, "rev-parse", "refs/heads/pmt/template/bug")
	if err != nil {
		t.Fatal(err)
	}
	if meta.TemplateRef != templateCommit {
		t.Errorf("TemplateRef = %q, want %q (the template's commit at creation time)", meta.TemplateRef, templateCommit)
	}
}

func TestCreateAutoTitle(t *testing.T) {
	dir := repoWithBugTemplate(t)
	result, err := Create(dir, defaultCfg(), "bug", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Branch != "bug/0001" {
		t.Errorf("Branch = %q, want bug/0001", result.Branch)
	}
}

func TestCreateAutoTitleUsesConfiguredPadWidth(t *testing.T) {
	dir := repoWithBugTemplate(t)
	result, err := Create(dir, config.RepoConfig{TitlePadWidth: 6}, "bug", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Branch != "bug/000001" {
		t.Errorf("Branch = %q, want bug/000001", result.Branch)
	}
}

func TestCreateAutoTitleSkipsExistingBranches(t *testing.T) {
	dir := repoWithBugTemplate(t)
	makeBranch(t, dir, "bug/0001")

	result, err := Create(dir, defaultCfg(), "bug", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Branch != "bug/0002" {
		t.Errorf("Branch = %q, want bug/0002 (0001 already taken)", result.Branch)
	}
}

func TestCreateMissingTemplate(t *testing.T) {
	dir := initRepo(t) // no template created
	_, err := Create(dir, defaultCfg(), "bug", "dboverflow")
	if err == nil {
		t.Fatal("expected error when the template doesn't exist")
	}
	if !strings.Contains(err.Error(), "template type") {
		t.Errorf("error should mention the missing template type: %v", err)
	}
}

func TestCreateUserSuppliedTitleCollision(t *testing.T) {
	dir := repoWithBugTemplate(t)
	if _, err := Create(dir, defaultCfg(), "bug", "dboverflow"); err != nil {
		t.Fatalf("Create (first): %v", err)
	}
	_, err := Create(dir, defaultCfg(), "bug", "dboverflow")
	if err == nil {
		t.Fatal("expected error creating an issue with a title that's already taken")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention the collision: %v", err)
	}
}

func TestCreateInvalidTitleRejected(t *testing.T) {
	dir := repoWithBugTemplate(t)
	_, err := Create(dir, defaultCfg(), "bug", "CON")
	if err == nil {
		t.Fatal("expected error for a reserved Windows device name as title")
	}

	exists, existsErr := git.RefExists(dir, "refs/heads/bug/CON")
	if existsErr != nil {
		t.Fatal(existsErr)
	}
	if exists {
		t.Error("no branch should have been created when title validation fails")
	}
}

func TestCreateOrphanedWorktreeDirRejected(t *testing.T) {
	dir := repoWithBugTemplate(t)
	worktreePath := git.ComputeWorktreePath(dir, "", "bug", "dboverflow")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Create(dir, defaultCfg(), "bug", "dboverflow")
	if !errors.Is(err, ErrOrphanedWorktreePath) {
		t.Fatalf("Create: got %v, want ErrOrphanedWorktreePath", err)
	}

	exists, existsErr := git.RefExists(dir, "refs/heads/bug/dboverflow")
	if existsErr != nil {
		t.Fatal(existsErr)
	}
	if exists {
		t.Error("branch should not have been created when the worktree path is occupied")
	}
}

func TestCreateWorktreesDirOverride(t *testing.T) {
	dir := repoWithBugTemplate(t)
	cfg := config.RepoConfig{TitlePadWidth: config.DefaultTitlePadWidth, WorktreesDir: "../custom.worktrees"}

	result, err := Create(dir, cfg, "bug", "dboverflow")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := git.ComputeWorktreePath(dir, "../custom.worktrees", "bug", "dboverflow")
	if result.WorktreePath != want {
		t.Errorf("WorktreePath = %q, want %q", result.WorktreePath, want)
	}
}

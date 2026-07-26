package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/config"
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

func TestResolveRepoFlagPath(t *testing.T) {
	dir := initRepo(t)
	r, err := Resolve(dir, "", config.UserConfig{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	assertSameRepo(t, r.Root, dir)
}

func TestResolveRepoFlagNickname(t *testing.T) {
	dir := initRepo(t)
	userCfg := config.UserConfig{Repos: map[string]string{"clientA": dir}}

	r, err := Resolve("clientA", "", userCfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	assertSameRepo(t, r.Root, dir)
}

func TestResolveRepoFlagUnknownNickname(t *testing.T) {
	userCfg := config.UserConfig{Repos: map[string]string{"clientA": "/anywhere"}}
	_, err := Resolve("nope", "", userCfg)
	if err == nil {
		t.Fatal("expected error for unknown nickname")
	}
	if !strings.Contains(err.Error(), "clientA") {
		t.Errorf("error %q should list known nicknames", err)
	}
}

func TestResolveCwdDiscovery(t *testing.T) {
	dir := initRepo(t)
	r, err := Resolve("", dir, config.UserConfig{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	assertSameRepo(t, r.Root, dir)
}

func TestResolveCwdNotRepoNoDefault(t *testing.T) {
	dir := t.TempDir() // not a git repo
	_, err := Resolve("", dir, config.UserConfig{})
	if err == nil {
		t.Fatal("expected error when cwd isn't a repo and no --repo/default_repo given")
	}
}

func TestResolveCwdNotRepoFallsBackToDefault(t *testing.T) {
	repoDir := initRepo(t)
	notRepoDir := t.TempDir()
	userCfg := config.UserConfig{
		Repos:       map[string]string{"clientA": repoDir},
		DefaultRepo: "clientA",
	}

	r, err := Resolve("", notRepoDir, userCfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	assertSameRepo(t, r.Root, repoDir)
}

func TestResolveBareRepoRejected(t *testing.T) {
	dir := t.TempDir()
	if _, err := git.Run(dir, "init", "-q", "--bare"); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	_, err := Resolve(dir, "", config.UserConfig{})
	if err != ErrBareRepo {
		t.Fatalf("Resolve on bare repo: got %v, want ErrBareRepo", err)
	}
}

func TestResolveLoadsRepoConfig(t *testing.T) {
	dir := initRepo(t)
	content := "worktrees_dir: ../custom.worktrees\ntitle_pad_width: 6\n"
	if err := os.WriteFile(config.RepoConfigPath(dir), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Resolve(dir, "", config.UserConfig{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.Config.WorktreesDir != "../custom.worktrees" {
		t.Errorf("WorktreesDir = %q, want ../custom.worktrees", r.Config.WorktreesDir)
	}
	if r.Config.TitlePadWidth != 6 {
		t.Errorf("TitlePadWidth = %d, want 6", r.Config.TitlePadWidth)
	}
}

func TestResolveFromLinkedWorktree(t *testing.T) {
	mainDir := initRepo(t)
	if err := os.WriteFile(filepath.Join(mainDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(mainDir, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(mainDir, "commit", "-q", "-m", "init"); err != nil {
		t.Fatal(err)
	}

	worktreeDir := filepath.Join(t.TempDir(), "wt")
	if _, err := git.Run(mainDir, "worktree", "add", "-b", "feature/x", worktreeDir); err != nil {
		t.Fatalf("git worktree add: %v", err)
	}

	r, err := Resolve("", worktreeDir, config.UserConfig{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	assertSameRepo(t, r.Root, mainDir)
}

// assertSameRepo verifies got resolves to the same repo as want by
// statting a marker file, sidestepping path-string/symlink differences.
func assertSameRepo(t *testing.T, got, want string) {
	t.Helper()
	marker := "marker-" + t.Name() + ".txt"
	if err := os.WriteFile(filepath.Join(want, marker), []byte("x"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(got, marker)); err != nil {
		t.Errorf("resolved root %q does not match expected repo %q: %v", got, want, err)
	}
}

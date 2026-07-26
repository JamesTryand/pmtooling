package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUserConfigFileMissing(t *testing.T) {
	cfg, err := loadUserConfigFile(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("loadUserConfigFile: %v", err)
	}
	if len(cfg.Repos) != 0 || cfg.DefaultRepo != "" {
		t.Errorf("expected zero-value config for missing file, got %+v", cfg)
	}
}

func TestLoadUserConfigFileParses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "repos:\n  clientA: C:\\work\\clientA\n  clientB: /home/james/work/clientB\ndefault_repo: clientA\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadUserConfigFile(path)
	if err != nil {
		t.Fatalf("loadUserConfigFile: %v", err)
	}
	if cfg.DefaultRepo != "clientA" {
		t.Errorf("DefaultRepo = %q, want clientA", cfg.DefaultRepo)
	}
	if cfg.Repos["clientA"] != `C:\work\clientA` {
		t.Errorf("Repos[clientA] = %q, want C:\\work\\clientA", cfg.Repos["clientA"])
	}
	if cfg.Repos["clientB"] != "/home/james/work/clientB" {
		t.Errorf("Repos[clientB] = %q, want /home/james/work/clientB", cfg.Repos["clientB"])
	}
}

func TestLoadUserConfigFileInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("repos: [this is not a map"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadUserConfigFile(path); err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadRepoConfigMissingFile(t *testing.T) {
	cfg, err := LoadRepoConfig(t.TempDir())
	if err != nil {
		t.Fatalf("LoadRepoConfig: %v", err)
	}
	if cfg.TitlePadWidth != DefaultTitlePadWidth {
		t.Errorf("TitlePadWidth = %d, want default %d", cfg.TitlePadWidth, DefaultTitlePadWidth)
	}
	if cfg.WorktreesDir != "" {
		t.Errorf("WorktreesDir = %q, want empty", cfg.WorktreesDir)
	}
}

func TestLoadRepoConfigOverrides(t *testing.T) {
	dir := t.TempDir()
	content := "worktrees_dir: ../clientA.worktrees\ntitle_pad_width: 6\n"
	if err := os.WriteFile(RepoConfigPath(dir), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRepoConfig(dir)
	if err != nil {
		t.Fatalf("LoadRepoConfig: %v", err)
	}
	if cfg.WorktreesDir != "../clientA.worktrees" {
		t.Errorf("WorktreesDir = %q, want ../clientA.worktrees", cfg.WorktreesDir)
	}
	if cfg.TitlePadWidth != 6 {
		t.Errorf("TitlePadWidth = %d, want 6", cfg.TitlePadWidth)
	}
}

func TestLoadRepoConfigPadWidthDefaultsWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	content := "worktrees_dir: ../wt\n"
	if err := os.WriteFile(RepoConfigPath(dir), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRepoConfig(dir)
	if err != nil {
		t.Fatalf("LoadRepoConfig: %v", err)
	}
	if cfg.TitlePadWidth != DefaultTitlePadWidth {
		t.Errorf("TitlePadWidth = %d, want default %d when omitted from file", cfg.TitlePadWidth, DefaultTitlePadWidth)
	}
}

func TestUserConfigPathHonorsOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PMT_CONFIG_HOME", dir)

	path, err := UserConfigPath()
	if err != nil {
		t.Fatalf("UserConfigPath: %v", err)
	}
	want := filepath.Join(dir, "pmt", "config.yaml")
	if path != want {
		t.Errorf("UserConfigPath() = %q, want %q", path, want)
	}
}

func TestSaveAndLoadUserConfigRoundTrip(t *testing.T) {
	t.Setenv("PMT_CONFIG_HOME", t.TempDir())

	cfg := UserConfig{
		Repos:       map[string]string{"clientA": "/work/clientA"},
		DefaultRepo: "clientA",
	}
	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	got, err := LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if got.DefaultRepo != cfg.DefaultRepo || got.Repos["clientA"] != cfg.Repos["clientA"] {
		t.Errorf("LoadUserConfig() = %+v, want %+v", got, cfg)
	}
}

func TestSaveUserConfigCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PMT_CONFIG_HOME", filepath.Join(dir, "nested", "does-not-exist-yet"))

	if err := SaveUserConfig(UserConfig{DefaultRepo: "x"}); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config file to exist at %s: %v", path, err)
	}
}

func TestLoadRepoConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(RepoConfigPath(dir), []byte("worktrees_dir: [oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepoConfig(dir); err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

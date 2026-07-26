// Package config loads pmt's two YAML config files: a user-level file
// holding repo nicknames, and a repo-local file holding per-target-repo
// overrides. See doc/architecture.md#config.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultTitlePadWidth is the auto-title zero-padding width used when a
// repo-local .pmt.yaml doesn't override it. See doc/commands.md's
// auto-title algorithm.
const DefaultTitlePadWidth = 4

// UserConfig is the user-level config: %APPDATA%\pmt\config.yaml on
// Windows (os.UserConfigDir()/pmt/config.yaml elsewhere).
type UserConfig struct {
	Repos       map[string]string `yaml:"repos"`
	DefaultRepo string            `yaml:"default_repo"`
}

// RepoConfig is the repo-local config: <mainRepoRoot>/.pmt.yaml, meant to
// be committed to the target repo like .editorconfig.
type RepoConfig struct {
	WorktreesDir  string `yaml:"worktrees_dir"`
	TitlePadWidth int    `yaml:"title_pad_width"`
}

// UserConfigPath returns the path pmt reads/writes user-level config from.
func UserConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving user config dir: %w", err)
	}
	return filepath.Join(dir, "pmt", "config.yaml"), nil
}

// LoadUserConfig loads the user-level config. A missing file is not an
// error — it yields a zero-value UserConfig (no nicknames, no default).
func LoadUserConfig() (UserConfig, error) {
	path, err := UserConfigPath()
	if err != nil {
		return UserConfig{}, err
	}
	return loadUserConfigFile(path)
}

func loadUserConfigFile(path string) (UserConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return UserConfig{}, nil
	}
	if err != nil {
		return UserConfig{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return UserConfig{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}

// RepoConfigPath returns the repo-local config path for a resolved main
// repo root.
func RepoConfigPath(mainRepoRoot string) string {
	return filepath.Join(mainRepoRoot, ".pmt.yaml")
}

// LoadRepoConfig loads the repo-local config for mainRepoRoot. A missing
// file is not an error — it yields RepoConfig with pmt's built-in
// defaults (see doc/architecture.md's config-precedence note).
func LoadRepoConfig(mainRepoRoot string) (RepoConfig, error) {
	cfg := RepoConfig{TitlePadWidth: DefaultTitlePadWidth}
	path := RepoConfigPath(mainRepoRoot)

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return RepoConfig{}, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return RepoConfig{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cfg.TitlePadWidth == 0 {
		cfg.TitlePadWidth = DefaultTitlePadWidth
	}
	return cfg, nil
}

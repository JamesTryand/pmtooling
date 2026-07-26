// Package repo resolves which target repository a pmt invocation should
// operate on, and loads its repo-local config. See
// doc/architecture.md#repo-resolution and #config.
package repo

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
)

// ErrBareRepo is returned when the resolved repo is bare. pmt v1 has no
// defined worktree location for a bare repo.
var ErrBareRepo = errors.New("target repo is bare; pmt v1 does not support bare repositories")

// Repo is a fully resolved target repository: its canonical main root
// (see git.RepoInfo.MainRoot) plus repo-local config.
type Repo struct {
	Root   string
	Config config.RepoConfig
}

// Resolve determines the target repo per doc/architecture.md's repo
// selection precedence:
//  1. repoFlag (--repo): an existing directory is used directly;
//     otherwise it's looked up as a nickname in userCfg.Repos.
//  2. No repoFlag: discovered from cwd.
//  3. cwd isn't inside any repo: falls back to userCfg.DefaultRepo, if set.
//
// Whatever the source, the result is re-derived to its canonical main
// root (so invocation from inside a linked/issue worktree still resolves
// correctly) and rejected if bare.
func Resolve(repoFlag, cwd string, userCfg config.UserConfig) (Repo, error) {
	root, err := resolveRoot(repoFlag, cwd, userCfg)
	if err != nil {
		return Repo{}, err
	}

	info, err := git.Discover(root)
	if err != nil {
		if errors.Is(err, git.ErrNotARepo) {
			return Repo{}, fmt.Errorf("%q is not a git repository", root)
		}
		return Repo{}, err
	}
	if info.IsBare {
		return Repo{}, ErrBareRepo
	}

	mainRoot := info.MainRoot()
	repoCfg, err := config.LoadRepoConfig(mainRoot)
	if err != nil {
		return Repo{}, err
	}
	return Repo{Root: mainRoot, Config: repoCfg}, nil
}

func resolveRoot(repoFlag, cwd string, userCfg config.UserConfig) (string, error) {
	if repoFlag != "" {
		return resolveNicknameOrPath(repoFlag, userCfg)
	}

	info, err := git.Discover(cwd)
	if err == nil {
		return info.MainRoot(), nil
	}
	if !errors.Is(err, git.ErrNotARepo) {
		return "", err
	}

	if userCfg.DefaultRepo != "" {
		return resolveNicknameOrPath(userCfg.DefaultRepo, userCfg)
	}
	return "", errors.New("not inside a git repository; pass --repo <path> or --repo <nickname>")
}

func resolveNicknameOrPath(value string, userCfg config.UserConfig) (string, error) {
	if info, err := os.Stat(value); err == nil && info.IsDir() {
		return value, nil
	}
	path, ok := userCfg.Repos[value]
	if !ok {
		return "", fmt.Errorf("unknown repo %q; known nicknames: %s", value, knownNicknames(userCfg))
	}
	return path, nil
}

func knownNicknames(cfg config.UserConfig) string {
	if len(cfg.Repos) == 0 {
		return "(none configured)"
	}
	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

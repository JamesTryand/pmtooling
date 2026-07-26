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

// EnvDefaultRepo is a literal filesystem path (not a nickname) used as a
// repo-selection fallback between cwd discovery and userCfg.DefaultRepo
// — see resolveRoot. A plain env var works identically on Windows/Linux/
// macOS: it's just a string read via os.Getenv, and the path syntax the
// user sets it to is whatever's native to their own OS.
const EnvDefaultRepo = "PMT_DEFAULT_REPO"

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
//  3. cwd isn't inside any repo: falls back to the EnvDefaultRepo env
//     var (a literal path), if set.
//  4. Still nothing: falls back to userCfg.DefaultRepo, if set.
//
// The env var sits ahead of the user-config default because it's
// typically a session/shell-scoped override (e.g. set in a terminal's
// profile for "whatever I'm working on right now"), while default_repo
// is a more permanent, saved setting.
//
// Whatever the source, the result is re-derived to its canonical main
// root (so invocation from inside a linked/issue worktree still resolves
// correctly), including for bare repos (Phase 7c) — MainRoot() resolves
// to the bare repo's own path in that case.
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

	mainRoot := info.MainRoot()
	repoCfg, err := config.LoadRepoConfig(mainRoot)
	if err != nil {
		return Repo{}, err
	}
	return Repo{Root: mainRoot, Config: repoCfg}, nil
}

// ResolveNamed resolves a --from-style value (a path or a configured
// nickname) to another target repo's canonical main root — used by
// `pmt template new/update --from <source>` (Phase 7d). Unlike Resolve,
// this never falls back to cwd or default_repo: --from always names a
// specific other repo explicitly, and the caller's own repo (resolved
// separately via --repo/cwd) is not an implicit fallback for it.
func ResolveNamed(value string, userCfg config.UserConfig) (string, error) {
	root, err := resolveNicknameOrPath(value, userCfg)
	if err != nil {
		return "", err
	}
	info, err := git.Discover(root)
	if err != nil {
		if errors.Is(err, git.ErrNotARepo) {
			return "", fmt.Errorf("%q is not a git repository", root)
		}
		return "", err
	}
	return info.MainRoot(), nil
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

	if envRepo := os.Getenv(EnvDefaultRepo); envRepo != "" {
		info, statErr := os.Stat(envRepo)
		if statErr != nil || !info.IsDir() {
			return "", fmt.Errorf("%s=%q does not exist or is not a directory", EnvDefaultRepo, envRepo)
		}
		return envRepo, nil
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

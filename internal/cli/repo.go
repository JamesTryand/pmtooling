package cli

import (
	"os"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

// resolveRepo resolves the target repo for the current invocation given
// the --repo flag value, per doc/architecture.md#repo-resolution. The
// nickname/default_repo lookup logic itself is tested directly against
// injected config.UserConfig values in internal/repo; this just wires the
// real user config file and cwd into that resolution.
func resolveRepo(repoFlag string) (repo.Repo, error) {
	userCfg, err := config.LoadUserConfig()
	if err != nil {
		return repo.Repo{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return repo.Repo{}, err
	}
	return repo.Resolve(repoFlag, cwd, userCfg)
}

// resolveNamedRepo resolves a --from-style value (a path or a configured
// nickname) to another target repo's canonical main root, per
// repo.ResolveNamed. Used by `pmt template new/update --from` (Phase 7d).
func resolveNamedRepo(value string) (string, error) {
	userCfg, err := config.LoadUserConfig()
	if err != nil {
		return "", err
	}
	return repo.ResolveNamed(value, userCfg)
}

package cli

import (
	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/publish"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

func newPullCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	var dryRun bool
	var remote string

	cmd := &cobra.Command{
		Use:   "pull <type>/<title>",
		Short: "Bring one issue from the remote onto this machine",
		Long: `Fetch the remote, then make one issue available here: create its worktree
(and local branch) if missing, or fast-forward a clean existing worktree.

A dirty worktree or a diverged branch is reported and left exactly as it is;
nothing is merged, reset or discarded. Exit status is 0 when the issue is
now present and current, 2 when it needs attention, 1 on error.`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			res, err := publish.Pull(r.Root, r.Config.WorktreesDir, remote, args[0], dryRun)
			if err != nil {
				return err
			}
			return reportResults(cmd.OutOrStdout(), []publish.Result{res}, dryRun, "nothing to do")
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without fetching or changing anything")
	cmd.Flags().StringVar(&remote, "remote", git.DefaultRemote, "remote name to pull from")
	return cmd
}

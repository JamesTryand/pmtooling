package cli

import (
	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/publish"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

func newPruneCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete leftover local branches of issues that were closed elsewhere",
		Long: `When an issue is closed with 'pmt close' on one machine, every other machine
keeps its local branch. prune deletes such a branch only when ALL of these hold:

  1. the issue is in pmt's archive (pmt list --archived), not merely gone
     from the remote;
  2. no worktree has the branch checked out;
  3. the branch tip holds nothing the archive lacks: every file in the tip
     exists, identical, in the archived copy (README.md may differ only by
     its close stamp).

A branch failing (2) or (3) is kept and reported ('in-use', 'differs', with the
paths). Every deletion prints the tip SHA and a command to restore it. Exit
status is 0 when nothing needed attention, 2 otherwise, 1 on error.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			results, err := publish.Prune(r.Root, dryRun)
			if err != nil {
				return err
			}
			return reportResults(cmd.OutOrStdout(), results, dryRun, "nothing to prune")
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without deleting")
	return cmd
}

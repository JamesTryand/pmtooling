package cli

import (
	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/publish"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

func newSyncCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	var opts publish.SyncOptions

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Reconcile this machine's worktrees with the remote",
		Long: `Fetch the remote and bring this machine's issue worktrees in line with it,
using pmt's own issue and archive semantics. sync never pushes (use 'pmt push'),
never merges or resets, and never removes anything that is not provably safe:

  - creates worktrees for remote issues that have none, and fast-forwards
    clean worktrees, pmt/archive, pmt/template/* and the main checkout's
    default branch;
  - reports, and leaves alone: dirty worktrees, diverged branches, local
    commits not yet pushed, detached HEADs, stray directories under the
    worktrees root, and remote branches that are not pmt issues;
  - a local issue with no remote copy is kept and reported 'unpublished' unless
    it is archived (closed) and its tip holds nothing the archive lacks, in
    which case its clean worktree and branch are removed, printing the tip SHA
    and how to restore it.

--dry-run changes no branch, worktree, file or remote; it still fetches, so
the preview reflects the real remote. Exit status is 0 when everything is in
step, 2 when something needs attention, 1 on error.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			results, err := publish.Sync(r.Root, r.Config, opts)
			if err != nil {
				return err
			}
			return reportResults(cmd.OutOrStdout(), results, opts.DryRun, "everything is in step")
		},
	}
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "report what would change without changing anything (still fetches)")
	cmd.Flags().BoolVar(&opts.SkipUpdate, "skip-update", false, "only create missing worktrees; leave existing ones alone")
	cmd.Flags().StringVar(&opts.Remote, "remote", git.DefaultRemote, "remote name to sync with")
	return cmd
}

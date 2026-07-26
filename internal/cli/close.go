package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/archive"
	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

func newCloseCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "close <type>/<title>",
		Short: "Archive and remove a finished issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			typeName, title := issue.Split(args[0])
			result, err := archive.Close(r.Root, r.Config, typeName, title)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Closed issue %s (archived at %s)\n", result.Branch, result.ArchiveCommit)
			return nil
		},
	}
}

func newReopenCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <type>/<title>",
		Short: "Restore a previously closed issue from the archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			typeName, title := issue.Split(args[0])
			result, err := archive.Reopen(r.Root, r.Config, typeName, title)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Reopened issue %s\n", result.Branch)
			fmt.Fprintf(cmd.OutOrStdout(), "Worktree: %s\n", result.WorktreePath)
			return nil
		},
	}
}

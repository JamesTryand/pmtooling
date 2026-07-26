package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

func newNewCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "new <type>[/<title>]",
		Short: "Create a new issue branch and worktree from a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			typeName, title := issue.Split(args[0])
			result, err := issue.Create(r.Root, r.Config, typeName, title)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created issue %s\n", result.Branch)
			fmt.Fprintf(cmd.OutOrStdout(), "Worktree: %s\n", result.WorktreePath)
			return nil
		},
	}
}

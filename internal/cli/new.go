package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/repo"
)

// newNewCmd is a Phase 2 stub: it wires --repo resolution end-to-end but
// doesn't yet create branches/worktrees. Full behavior lands in Phase 4,
// per doc/commands.md and task_plan.md.
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
			fmt.Fprintf(cmd.OutOrStdout(),
				"pmt new %s: not yet implemented (resolved target repo: %s) — see task_plan.md Phase 4\n",
				args[0], r.Root)
			return nil
		},
	}
}

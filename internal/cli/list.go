package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/repo"
)

// newListCmd is a Phase 2 stub; full behavior (ref/worktree
// cross-referencing, README front-matter reads) lands in Phase 5.
func newListCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	var typeFilter string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues in the target repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"pmt list: not yet implemented (type=%q json=%v, resolved target repo: %s) — see task_plan.md Phase 5\n",
				typeFilter, jsonOut, r.Root)
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "filter by issue type")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

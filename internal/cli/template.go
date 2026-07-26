package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/repo"
)

func newTemplateCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage issue templates",
	}
	cmd.AddCommand(newTemplateNewCmd(resolve))
	cmd.AddCommand(newTemplateListCmd(resolve))
	return cmd
}

// newTemplateNewCmd is a Phase 2 stub; full scaffolding behavior lands in
// Phase 3, per doc/templates.md and task_plan.md.
func newTemplateNewCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new template branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"pmt template new %s: not yet implemented (resolved target repo: %s) — see task_plan.md Phase 3\n",
				args[0], r.Root)
			return nil
		},
	}
}

// newTemplateListCmd is a Phase 2 stub; full behavior lands in Phase 3.
func newTemplateListCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available templates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"pmt template list: not yet implemented (resolved target repo: %s) — see task_plan.md Phase 3\n",
				r.Root)
			return nil
		},
	}
}

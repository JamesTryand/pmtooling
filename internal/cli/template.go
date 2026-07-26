package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/repo"
	"github.com/JamesTryand/pmtooling/internal/template"
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
			name := args[0]
			commit, err := template.New(r.Root, name)
			if err != nil {
				return err
			}
			shortRef := strings.TrimPrefix(template.RefFor(name), "refs/heads/")
			fmt.Fprintf(cmd.OutOrStdout(), "Created template %q (%s)\n", name, commit)
			fmt.Fprintf(cmd.OutOrStdout(), "Check it out with `git worktree add <path> %s` or `git switch %s`, edit, and commit.\n", shortRef, shortRef)
			return nil
		},
	}
}

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
			names, err := template.List(r.Root)
			if err != nil {
				return err
			}
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No templates found. Run `pmt template new <name>` to create one.")
				return nil
			}
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	}
}

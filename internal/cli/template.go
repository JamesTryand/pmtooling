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
	cmd.AddCommand(newTemplateUpdateCmd(resolve))
	return cmd
}

func newTemplateNewCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new template branch, or import one from another repo with --from",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			name := args[0]

			if from != "" {
				sourceRoot, err := resolveNamedRepo(from)
				if err != nil {
					return err
				}
				if err := template.Import(r.Root, sourceRoot, name); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Imported template %q from %s\n", name, from)
				return nil
			}

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
	cmd.Flags().StringVar(&from, "from", "", "import this template from another repo (path or configured nickname) instead of scaffolding a new one")
	return cmd
}

func newTemplateUpdateCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "update <name> --from <source>",
		Short: "Pull in changes to an already-imported template from its source repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" {
				return fmt.Errorf("--from <path-or-nickname> is required")
			}
			r, err := resolve()
			if err != nil {
				return err
			}
			name := args[0]
			sourceRoot, err := resolveNamedRepo(from)
			if err != nil {
				return err
			}
			result, err := template.Update(r.Root, sourceRoot, name)
			if err != nil {
				return err
			}
			switch {
			case result.UpToDate:
				fmt.Fprintf(cmd.OutOrStdout(), "Template %q is already up to date.\n", name)
			case result.FastForwarded:
				fmt.Fprintf(cmd.OutOrStdout(), "Updated template %q (fast-forwarded).\n", name)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "Template %q has diverged from %s; not merging automatically.\n", name, from)
				fmt.Fprintf(cmd.OutOrStdout(), "The incoming changes are at %s -- merge manually, e.g.:\n", result.IncomingRef)
				fmt.Fprintf(cmd.OutOrStdout(), "  git worktree add <path> %s\n", strings.TrimPrefix(template.RefFor(name), "refs/heads/"))
				fmt.Fprintf(cmd.OutOrStdout(), "  git -C <path> merge %s\n", result.IncomingRef)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "pull updates from this repo (path or configured nickname)")
	return cmd
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

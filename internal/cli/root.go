// Package cli wires pmt's cobra command surface. See doc/commands.md for
// the command reference this implements.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/repo"
)

// NewRootCmd builds a fresh root command. Called once from cmd/pmt/main.go
// (and repeatedly in tests, each call getting its own --repo flag binding).
func NewRootCmd() *cobra.Command {
	var repoFlag string

	root := &cobra.Command{
		Use:          "pmt",
		Short:        "Manage issues as git branches and worktrees in a target repo",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&repoFlag, "repo", "", "target repo path or configured nickname")

	resolve := func() (repo.Repo, error) {
		return resolveRepo(repoFlag)
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newNewCmd(resolve))
	root.AddCommand(newTemplateCmd(resolve))
	root.AddCommand(newListCmd(resolve))
	root.AddCommand(newRepoCmd())
	root.AddCommand(newCloseCmd(resolve))
	root.AddCommand(newReopenCmd(resolve))
	root.AddCommand(newGetCmd(resolve))
	return root
}

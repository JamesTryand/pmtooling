package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// Version is set at build time via -ldflags "-X .../internal/cli.Version=...".
// `pmt version` is a utility command outside the product command surface
// documented in doc/commands.md.
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the pmt version and check the installed git version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "pmt", Version)
			if err := git.CheckVersion(); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "warning:", err)
			}
			return nil
		},
	}
}

package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// Version reports the module version pmt was built from. `go install
// .../pmt@<version>` (the documented install command) embeds the resolved
// module version into the binary automatically via runtime/debug.BuildInfo
// — no custom -ldflags needed, which matters because a plain `go install
// ...@latest` never passes any. Falls back to "dev" for a local `go build`
// run inside the module's own source tree, where Go reports "(devel)"
// instead of a real version.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

// `pmt version` is a utility command outside the product command surface
// documented in doc/commands.md.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the pmt version and check the installed git version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "pmt", Version())
			if err := git.CheckVersion(); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "warning:", err)
			}
			return nil
		},
	}
}

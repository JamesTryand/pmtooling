// Command pmt manages issues as git branches and worktrees in a target
// repository. The CLI surface (cobra commands) is wired up in Phase 2;
// this skeleton only verifies the git primitives are usable.
package main

import (
	"fmt"
	"os"

	"github.com/JamesTryand/pmtooling/internal/git"
)

func main() {
	if err := git.CheckVersion(); err != nil {
		fmt.Fprintln(os.Stderr, "pmt:", err)
		os.Exit(1)
	}
	fmt.Println("pmt: git primitives OK (CLI not yet implemented — see task_plan.md Phase 2)")
}

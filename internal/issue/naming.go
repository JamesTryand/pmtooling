package issue

import (
	"fmt"
	"strings"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// Split parses a `pmt new` argument into type and optional title, e.g.
// "bug/dboverflow" -> ("bug", "dboverflow"), "bug" -> ("bug", "").
func Split(arg string) (typeName, title string) {
	idx := strings.IndexByte(arg, '/')
	if idx < 0 {
		return arg, ""
	}
	return arg[:idx], arg[idx+1:]
}

// ValidateType checks typeName is a single valid ref segment. See
// doc/commands.md#pmt-new step 2. Split() never produces a type
// containing '/', but the check is repeated here defensively for any
// other caller.
func ValidateType(typeName string) error {
	if strings.Contains(typeName, "/") {
		return fmt.Errorf("type must not contain '/': got %q", typeName)
	}
	if err := git.CheckRefFormat(typeName); err != nil {
		return fmt.Errorf("invalid type %q: %w", typeName, err)
	}
	return nil
}

// reservedWindowsNames are device names that can't be used as a file or
// directory name on Windows, regardless of case.
var reservedWindowsNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// ValidateTitle validates a (possibly auto-generated) title against the
// full set of rules from doc/commands.md#pmt-new step 5: no embedded '/',
// git's own check-ref-format on the full branch name, and a Windows
// filesystem-safety layer — since the title also becomes a worktree
// directory name.
//
// The Windows-safety layer only checks reserved device names (CON, NUL,
// COM1, ...): git itself has no opinion about them, so check-ref-format
// lets them through. Trailing dots and embedded/trailing spaces — the
// other two classic Windows filename gotchas — turn out to already be
// unconditionally rejected by check-ref-format (verified empirically:
// "foo." and "foo " both fail it), so no separate check is needed for
// those; adding one would just be dead code.
func ValidateTitle(typeName, title string) error {
	if strings.Contains(title, "/") {
		return fmt.Errorf("title must not contain '/': got %q", title)
	}
	branch := typeName + "/" + title
	if err := git.CheckRefFormat(branch); err != nil {
		return fmt.Errorf("invalid branch name %q: %w", branch, err)
	}
	if reservedWindowsNames[strings.ToUpper(title)] {
		return fmt.Errorf("title %q is a reserved Windows device name", title)
	}
	return nil
}

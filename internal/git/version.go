package git

import (
	"fmt"
	"regexp"
	"strconv"
)

// MinVersion is the lowest git version pmt supports. 2.31 is required for
// `rev-parse --path-format=absolute`, which Discover relies on.
var MinVersion = Version{2, 31, 0}

type Version struct {
	Major, Minor, Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// DetectVersion runs `git --version` and parses the result, e.g.
// "git version 2.51.0.windows.1" -> Version{2, 51, 0}.
func DetectVersion() (Version, error) {
	out, err := Run("", "--version")
	if err != nil {
		return Version{}, fmt.Errorf("git not found or failed to run: %w", err)
	}
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return Version{}, fmt.Errorf("could not parse git version from %q", out)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return Version{major, minor, patch}, nil
}

// CheckVersion fails fast if the installed git predates MinVersion.
func CheckVersion() error {
	v, err := DetectVersion()
	if err != nil {
		return err
	}
	if v.Less(MinVersion) {
		return fmt.Errorf("git %s found, but pmt requires git >= %s", v, MinVersion)
	}
	return nil
}

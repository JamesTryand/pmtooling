package archive

import (
	"testing"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/template"
)

func defaultCfg() config.RepoConfig {
	return config.RepoConfig{TitlePadWidth: config.DefaultTitlePadWidth}
}

// repoWithIssue sets up a scratch repo with a "bug" template and one real
// issue created via the actual issue.Create pipeline (not a hand-rolled
// double), so Close/Reopen are exercised against exactly what `pmt new`
// produces.
func repoWithIssue(t *testing.T, title string) (dir string, result issue.Result) {
	t.Helper()
	dir = initRepo(t)
	if _, err := template.New(dir, "bug"); err != nil {
		t.Fatalf("template.New: %v", err)
	}
	result, err := issue.Create(dir, defaultCfg(), "bug", title)
	if err != nil {
		t.Fatalf("issue.Create: %v", err)
	}
	return dir, result
}

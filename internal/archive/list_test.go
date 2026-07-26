package archive

import (
	"testing"

	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/template"
)

func TestListArchivedEmpty(t *testing.T) {
	dir := initRepo(t)
	got, err := ListArchived(dir, "")
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListArchived = %+v, want empty (no archive branch yet)", got)
	}
}

func TestListArchivedAfterClose(t *testing.T) {
	dir, _ := repoWithIssue(t, "dboverflow")
	if _, err := Close(dir, defaultCfg(), "bug", "dboverflow"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := ListArchived(dir, "")
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if len(got) != 1 || got[0].Branch != "bug/dboverflow" {
		t.Fatalf("ListArchived = %+v, want one entry bug/dboverflow", got)
	}
	if got[0].Closed == "" {
		t.Error("expected a non-empty Closed timestamp")
	}
	if got[0].Unparseable {
		t.Error("expected Unparseable=false for a normally-closed issue")
	}
}

func TestListArchivedTypeFilter(t *testing.T) {
	dir, _ := repoWithIssue(t, "one")
	if _, err := template.New(dir, "feature"); err != nil {
		t.Fatalf("template.New(feature): %v", err)
	}
	if _, err := issue.Create(dir, defaultCfg(), "feature", "one"); err != nil {
		t.Fatalf("issue.Create(feature/one): %v", err)
	}
	if _, err := Close(dir, defaultCfg(), "bug", "one"); err != nil {
		t.Fatalf("Close(bug/one): %v", err)
	}
	if _, err := Close(dir, defaultCfg(), "feature", "one"); err != nil {
		t.Fatalf("Close(feature/one): %v", err)
	}

	got, err := ListArchived(dir, "bug")
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if len(got) != 1 || got[0].Branch != "bug/one" {
		t.Fatalf("ListArchived(--type bug) = %+v, want only bug/one", got)
	}
}

func TestListArchivedShowsStaleEntryAfterReopen(t *testing.T) {
	// This is the intentional "append-only" behavior confirmed with the
	// user: reopening does not remove the archive entry, so it keeps
	// showing up (now stale) until the issue is closed again.
	dir, _ := repoWithIssue(t, "dboverflow")
	if _, err := Close(dir, defaultCfg(), "bug", "dboverflow"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := Reopen(dir, defaultCfg(), "bug", "dboverflow"); err != nil {
		t.Fatalf("Reopen: %v", err)
	}

	got, err := ListArchived(dir, "")
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if len(got) != 1 || got[0].Branch != "bug/dboverflow" {
		t.Errorf("ListArchived after reopen = %+v, want the stale bug/dboverflow entry still present", got)
	}
}

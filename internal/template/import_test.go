package template

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// advanceTemplate adds one more commit to an existing template branch via
// a scratch worktree (templates aren't normally checked out, but tests
// need a way to simulate the source or destination template evolving).
func advanceTemplate(t *testing.T, dir, typeName, filename, content string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "wt")
	shortRef := "pmt/template/" + typeName
	if err := git.WorktreeAdd(dir, wt, shortRef); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(wt, "add", filename); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(wt, "commit", "-q", "-m", "advance "+typeName); err != nil {
		t.Fatal(err)
	}
	tip, err := git.Run(wt, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "worktree", "remove", wt); err != nil {
		t.Fatal(err)
	}
	return tip
}

func TestImportFreshRepo(t *testing.T) {
	src := initRepo(t)
	srcTip, err := New(src, "bug")
	if err != nil {
		t.Fatalf("New (source): %v", err)
	}

	dst := initRepo(t)
	if err := Import(dst, src, "bug"); err != nil {
		t.Fatalf("Import: %v", err)
	}

	exists, err := Exists(dst, "bug")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected template 'bug' to exist in destination after Import")
	}
	dstTip, err := git.Run(dst, "rev-parse", RefFor("bug"))
	if err != nil {
		t.Fatal(err)
	}
	if dstTip != srcTip {
		t.Errorf("imported tip = %s, want %s (exact same commit as source)", dstTip, srcTip)
	}
}

func TestImportCollisionWithExistingLocalTemplate(t *testing.T) {
	src := initRepo(t)
	if _, err := New(src, "bug"); err != nil {
		t.Fatal(err)
	}
	dst := initRepo(t)
	if _, err := New(dst, "bug"); err != nil {
		t.Fatal(err)
	}

	err := Import(dst, src, "bug")
	if !errors.Is(err, ErrExists) {
		t.Fatalf("Import over existing local template: got %v, want ErrExists", err)
	}
}

func TestImportSourceMissingTemplate(t *testing.T) {
	src := initRepo(t)
	dst := initRepo(t)
	if err := Import(dst, src, "bug"); err == nil {
		t.Fatal("expected error importing a template the source doesn't have")
	}
}

func TestUpdateNotFoundLocally(t *testing.T) {
	src := initRepo(t)
	if _, err := New(src, "bug"); err != nil {
		t.Fatal(err)
	}
	dst := initRepo(t)
	_, err := Update(dst, src, "bug")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update on a template never imported: got %v, want ErrNotFound", err)
	}
}

func TestUpdateAlreadyUpToDate(t *testing.T) {
	src := initRepo(t)
	if _, err := New(src, "bug"); err != nil {
		t.Fatal(err)
	}
	dst := initRepo(t)
	if err := Import(dst, src, "bug"); err != nil {
		t.Fatal(err)
	}

	result, err := Update(dst, src, "bug")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.UpToDate || result.FastForwarded || result.IncomingRef != "" {
		t.Errorf("Update result = %+v, want UpToDate only", result)
	}
	exists, err := git.RefExists(dst, "refs/heads/pmt/template-incoming/bug")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("scratch incoming ref should be cleaned up when already up to date")
	}
}

func TestUpdateFastForward(t *testing.T) {
	src := initRepo(t)
	if _, err := New(src, "bug"); err != nil {
		t.Fatal(err)
	}
	dst := initRepo(t)
	if err := Import(dst, src, "bug"); err != nil {
		t.Fatal(err)
	}

	newTip := advanceTemplate(t, src, "bug", "NOTES.md", "new guidance")

	result, err := Update(dst, src, "bug")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.FastForwarded || result.UpToDate || result.IncomingRef != "" {
		t.Errorf("Update result = %+v, want FastForwarded only", result)
	}
	dstTip, err := git.Run(dst, "rev-parse", RefFor("bug"))
	if err != nil {
		t.Fatal(err)
	}
	if dstTip != newTip {
		t.Errorf("local template tip = %s, want %s (fast-forwarded)", dstTip, newTip)
	}
	exists, err := git.RefExists(dst, "refs/heads/pmt/template-incoming/bug")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("scratch incoming ref should be cleaned up after a fast-forward")
	}
}

func TestUpdateDivergedLeavesIncomingRefAndDoesNotTouchLocal(t *testing.T) {
	src := initRepo(t)
	if _, err := New(src, "bug"); err != nil {
		t.Fatal(err)
	}
	dst := initRepo(t)
	if err := Import(dst, src, "bug"); err != nil {
		t.Fatal(err)
	}

	// Both sides evolve independently -> diverged.
	sourceTip := advanceTemplate(t, src, "bug", "SOURCE.md", "source-side change")
	localTip := advanceTemplate(t, dst, "bug", "LOCAL.md", "local-side change")

	result, err := Update(dst, src, "bug")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if result.IncomingRef != "pmt/template-incoming/bug" {
		t.Errorf("IncomingRef = %q, want pmt/template-incoming/bug", result.IncomingRef)
	}
	if result.FastForwarded || result.UpToDate {
		t.Errorf("Update result = %+v, want IncomingRef only", result)
	}

	// Local template must be completely untouched.
	stillLocalTip, err := git.Run(dst, "rev-parse", RefFor("bug"))
	if err != nil {
		t.Fatal(err)
	}
	if stillLocalTip != localTip {
		t.Errorf("local template tip changed to %s, want unchanged %s", stillLocalTip, localTip)
	}

	// Incoming ref must hold the source's new content, inspectable.
	incomingTip, err := git.Run(dst, "rev-parse", "refs/heads/pmt/template-incoming/bug")
	if err != nil {
		t.Fatal(err)
	}
	if incomingTip != sourceTip {
		t.Errorf("incoming ref tip = %s, want %s", incomingTip, sourceTip)
	}
}

func TestUpdateLocalAlreadyAheadIsUpToDate(t *testing.T) {
	src := initRepo(t)
	if _, err := New(src, "bug"); err != nil {
		t.Fatal(err)
	}
	dst := initRepo(t)
	if err := Import(dst, src, "bug"); err != nil {
		t.Fatal(err)
	}
	// Only the local copy advances; source is unchanged.
	localTip := advanceTemplate(t, dst, "bug", "LOCAL.md", "local-only change")

	result, err := Update(dst, src, "bug")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.UpToDate {
		t.Errorf("Update result = %+v, want UpToDate (local already ahead of source)", result)
	}
	stillLocalTip, err := git.Run(dst, "rev-parse", RefFor("bug"))
	if err != nil {
		t.Fatal(err)
	}
	if stillLocalTip != localTip {
		t.Errorf("local template tip changed to %s, want unchanged %s", stillLocalTip, localTip)
	}
}

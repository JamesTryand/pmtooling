package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/issue"
)

// execRootSplit is like execRoot but keeps stdout and stderr separate,
// needed here (unlike every other command's tests) because `pmt get`'s
// contract specifically depends on stdout staying empty on every failure
// path, so callers can safely do `dir=$(pmt get ...) && cd "$dir"`.
func execRootSplit(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := NewRootCmd()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestGetCmdPrintsWorktreePath(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	issues, err := issue.ListIssues(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %d", len(issues))
	}
	want := issues[0].WorktreePath

	out, errOut, err := execRootSplit(t, "get", "bug/foo", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt get: %v (stderr: %s)", err, errOut)
	}
	if errOut != "" {
		t.Errorf("stderr should be empty on success, got %q", errOut)
	}
	if got := strings.TrimSpace(out); got != want {
		t.Errorf("stdout = %q, want exactly %q", got, want)
	}
}

func TestGetCmdNotFound(t *testing.T) {
	dir := initRepo(t)

	out, errOut, err := execRootSplit(t, "get", "bug/never-existed", "--repo", dir)
	if err == nil {
		t.Fatal("expected error for a nonexistent issue")
	}
	if out != "" {
		t.Errorf("stdout should be empty on failure, got %q", out)
	}
	if !strings.Contains(errOut, "not found") || !strings.Contains(errOut, "pmt list") || !strings.Contains(errOut, "pmt list --archived") {
		t.Errorf("stderr %q should explain not-found and list both list commands", errOut)
	}
}

func TestGetCmdArchived(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	if _, err := execRoot(t, "close", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt close: %v", err)
	}

	out, errOut, err := execRootSplit(t, "get", "bug/foo", "--repo", dir)
	if err == nil {
		t.Fatal("expected error for an archived issue")
	}
	if out != "" {
		t.Errorf("stdout should be empty on failure, got %q", out)
	}
	if !strings.Contains(errOut, "archived") || !strings.Contains(errOut, "pmt reopen bug/foo") {
		t.Errorf("stderr %q should explain it's archived and give the reopen command", errOut)
	}
}

func TestGetCmdWorktreeMissing(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	issues, err := issue.ListIssues(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(issues[0].WorktreePath); err != nil {
		t.Fatal(err)
	}

	out, errOut, err := execRootSplit(t, "get", "bug/foo", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when the worktree directory is gone")
	}
	if out != "" {
		t.Errorf("stdout should be empty on failure, got %q", out)
	}
	if !strings.Contains(errOut, "worktree isn't available") {
		t.Errorf("stderr %q should explain the worktree isn't available", errOut)
	}
}

func TestGetCmdNoArgUsesCurrentBranch(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	issues, err := issue.ListIssues(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	chdir(t, issues[0].WorktreePath)

	out, errOut, err := execRootSplit(t, "get", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt get: %v (stderr: %s)", err, errOut)
	}
	if got := strings.TrimSpace(out); got != issues[0].WorktreePath {
		t.Errorf("stdout = %q, want %q", got, issues[0].WorktreePath)
	}
}

func TestGetCmdNoArgNotOnIssueBranch(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)

	out, errOut, err := execRootSplit(t, "get", "--repo", dir)
	if err == nil {
		t.Fatal("expected error when no issue is given and the current branch isn't an issue")
	}
	if out != "" {
		t.Errorf("stdout should be empty on failure, got %q", out)
	}
	if !strings.Contains(errOut, "doesn't look like an issue") {
		t.Errorf("stderr %q should explain the current branch isn't an issue", errOut)
	}
}

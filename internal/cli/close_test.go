package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCloseCmdEndToEnd(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	out, err := execRoot(t, "close", "bug/foo", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt close: %v", err)
	}
	if !strings.Contains(out, "bug/foo") {
		t.Errorf("output %q should mention the closed issue", out)
	}

	listOut, err := execRoot(t, "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if strings.Contains(listOut, "bug/foo") {
		t.Errorf("closed issue should no longer appear in live `pmt list`: %q", listOut)
	}
}

func TestCloseCmdNonexistent(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "close", "bug/never-existed", "--repo", dir); err == nil {
		t.Fatal("expected error closing a nonexistent issue")
	}
}

func TestReopenCmdEndToEnd(t *testing.T) {
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

	out, err := execRoot(t, "reopen", "bug/foo", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt reopen: %v", err)
	}
	if !strings.Contains(out, "bug/foo") {
		t.Errorf("output %q should mention the reopened issue", out)
	}

	listOut, err := execRoot(t, "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if !strings.Contains(listOut, "bug/foo") {
		t.Errorf("reopened issue should appear back in live `pmt list`: %q", listOut)
	}
}

func TestReopenCmdNotArchived(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "reopen", "bug/never-archived", "--repo", dir); err == nil {
		t.Fatal("expected error reopening an issue that was never archived")
	}
}

func TestListCmdArchivedFlag(t *testing.T) {
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

	out, err := execRoot(t, "list", "--archived", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list --archived: %v", err)
	}
	if !strings.Contains(out, "bug/foo") || !strings.Contains(out, "CLOSED") {
		t.Errorf("output %q should show the archived table with bug/foo", out)
	}
}

func TestListCmdArchivedJSON(t *testing.T) {
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

	out, err := execRoot(t, "list", "--archived", "--json", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list --archived --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("--archived --json output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(rows) != 1 || rows[0]["branch"] != "bug/foo" {
		t.Errorf("decoded JSON = %+v, want one archived issue bug/foo", rows)
	}
}

func TestListCmdArchivedEmpty(t *testing.T) {
	dir := initRepo(t)
	out, err := execRoot(t, "list", "--archived", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list --archived: %v", err)
	}
	if !strings.Contains(out, "No archived issues found") {
		t.Errorf("output %q should indicate no archived issues exist yet", out)
	}
}

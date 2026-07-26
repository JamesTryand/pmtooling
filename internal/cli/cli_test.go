package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// initRepo creates a scratch git repo for --repo <path>-based tests. This
// never touches the real user-level config (verified: no
// %APPDATA%\pmt\config.yaml exists on dev machines at the time these tests
// were written); nickname/default_repo resolution itself is tested
// directly in internal/repo against injected config.UserConfig values.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := git.Run(dir, "init", "-q"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := git.Run(dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dir, "config", "user.name", "pmt test"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func execRoot(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err = root.Execute()
	return buf.String(), err
}

func TestRootCmdHasExpectedSubcommands(t *testing.T) {
	root := NewRootCmd()
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"version", "new", "template", "list"} {
		if !names[want] {
			t.Errorf("root command missing subcommand %q", want)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	out, err := execRoot(t, "version")
	if err != nil {
		t.Fatalf("pmt version: %v", err)
	}
	if !strings.Contains(out, "pmt") {
		t.Errorf("output %q missing version line", out)
	}
}

func TestNewCmdRequiresArg(t *testing.T) {
	if _, err := execRoot(t, "new"); err == nil {
		t.Fatal("expected error for `pmt new` with no args")
	}
}

func TestNewCmdEndToEnd(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}

	out, err := execRoot(t, "new", "bug/foo", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	if !strings.Contains(out, "bug/foo") {
		t.Errorf("output %q should mention the created issue branch", out)
	}
}

func TestNewCmdAutoTitle(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}

	out, err := execRoot(t, "new", "bug", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	if !strings.Contains(out, "bug/0001") {
		t.Errorf("output %q should mention the auto-generated title", out)
	}
}

func TestNewCmdMissingTemplateErrors(t *testing.T) {
	dir := initRepo(t) // no template created
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err == nil {
		t.Fatal("expected error when the template doesn't exist")
	}
}

func TestNewCmdUnknownRepoErrors(t *testing.T) {
	if _, err := execRoot(t, "new", "bug/foo", "--repo", "not-a-real-path-or-nickname"); err == nil {
		t.Fatal("expected error for unresolvable --repo value")
	}
}

func TestTemplateNewCmd(t *testing.T) {
	dir := initRepo(t)
	out, err := execRoot(t, "template", "new", "bug", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("output %q should mention the created template", out)
	}
}

func TestTemplateNewCmdCollision(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new (first): %v", err)
	}
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err == nil {
		t.Fatal("expected error creating a template that already exists")
	}
}

func TestTemplateListCmd(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	out, err := execRoot(t, "template", "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt template list: %v", err)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("output %q should list the 'bug' template", out)
	}
}

func TestTemplateListCmdEmpty(t *testing.T) {
	dir := initRepo(t)
	out, err := execRoot(t, "template", "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt template list: %v", err)
	}
	if !strings.Contains(out, "No templates found") {
		t.Errorf("output %q should indicate no templates exist yet", out)
	}
}

func TestListCmdEmpty(t *testing.T) {
	dir := initRepo(t)
	out, err := execRoot(t, "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if !strings.Contains(out, "No issues found") {
		t.Errorf("output %q should indicate no issues exist yet", out)
	}
}

func TestListCmdTableOutput(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	out, err := execRoot(t, "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if !strings.Contains(out, "BRANCH") || !strings.Contains(out, "bug/foo") || !strings.Contains(out, "open") {
		t.Errorf("output %q should show the table header and the created issue", out)
	}
}

func TestListCmdJSONOutput(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	out, err := execRoot(t, "list", "--json", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	var issues []map[string]any
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(issues) != 1 || issues[0]["branch"] != "bug/foo" {
		t.Errorf("decoded JSON = %+v, want one issue with branch bug/foo", issues)
	}
}

func TestListCmdTypeFilter(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "template", "new", "feature", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	if _, err := execRoot(t, "new", "feature/bar", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	out, err := execRoot(t, "list", "--type", "bug", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if !strings.Contains(out, "bug/foo") || strings.Contains(out, "feature/bar") {
		t.Errorf("output %q should include only bug/foo, not feature/bar", out)
	}
}

func TestListCmdBareRepoRejected(t *testing.T) {
	dir := t.TempDir()
	if _, err := git.Run(dir, "init", "-q", "--bare"); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	if _, err := execRoot(t, "list", "--repo", dir); err == nil {
		t.Fatal("expected error for a bare --repo target")
	}
}

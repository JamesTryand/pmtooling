package cli

import (
	"bytes"
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

func TestNewCmdResolvesRepoFlag(t *testing.T) {
	dir := initRepo(t)
	out, err := execRoot(t, "new", "bug/foo", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	if !strings.Contains(out, "not yet implemented") {
		t.Errorf("output %q should indicate stub behavior", out)
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

func TestListCmdWithFlags(t *testing.T) {
	dir := initRepo(t)
	out, err := execRoot(t, "list", "--type", "bug", "--json", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if !strings.Contains(out, `type="bug"`) || !strings.Contains(out, "json=true") {
		t.Errorf("output %q should reflect --type/--json flags", out)
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

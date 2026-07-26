package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/repo"
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

// chdir changes the process cwd for the duration of the test and
// restores it on cleanup — used only for the cwd-based repo-resolution
// edge cases (no --repo given), since that's the one behavior that can't
// be exercised through execRoot without actually moving the process.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restoring cwd to %s: %v", orig, err)
		}
	})
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

func TestListCmdBareRepoAccepted(t *testing.T) {
	dir := t.TempDir()
	if _, err := git.Run(dir, "init", "-q", "--bare"); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	out, err := execRoot(t, "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list on a bare --repo target: %v (Phase 7c: bare repos are supported)", err)
	}
	if !strings.Contains(out, "No issues found") {
		t.Errorf("output %q should show the normal empty-list message", out)
	}
}

// TestNoRepoAndCwdNotARepo covers doc/edge-cases.md's "running pmt outside
// any git repo, no --repo/config given" row through the actual CLI, not
// just internal/repo.Resolve in isolation.
func TestNoRepoAndCwdNotARepo(t *testing.T) {
	t.Setenv(repo.EnvDefaultRepo, "") // ensure no ambient env value masks this case
	chdir(t, t.TempDir())             // deliberately not a git repo
	if _, err := execRoot(t, "list"); err == nil {
		t.Fatal("expected error when cwd isn't a repo and no --repo was given")
	}
}

// TestListCmdEnvDefaultRepo covers PMT_DEFAULT_REPO through the actual
// CLI: no --repo, cwd outside any git repo, falls back to the env var.
func TestListCmdEnvDefaultRepo(t *testing.T) {
	repoDir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", repoDir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", repoDir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	t.Setenv(repo.EnvDefaultRepo, repoDir)
	chdir(t, t.TempDir()) // deliberately not a git repo, no --repo given

	out, err := execRoot(t, "list")
	if err != nil {
		t.Fatalf("pmt list (via PMT_DEFAULT_REPO): %v", err)
	}
	if !strings.Contains(out, "bug/foo") {
		t.Errorf("output %q should list bug/foo via the env-var fallback repo", out)
	}
}

// TestListCmdFromInsideIssueWorktree and TestNewCmdFromInsideIssueWorktree
// cover doc/edge-cases.md's "running pmt from inside one of its own issue
// worktrees" row through the actual CLI (cwd-based resolution, no
// --repo), not just internal/repo.Resolve/git.Discover in isolation.

func TestListCmdFromInsideIssueWorktree(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	worktreePath := git.ComputeWorktreePath(dir, "", "bug", "foo")
	chdir(t, worktreePath)

	out, err := execRoot(t, "list") // no --repo: must resolve via cwd -> MainRoot()
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if !strings.Contains(out, "bug/foo") {
		t.Errorf("output %q should list bug/foo even when run from inside its own worktree", out)
	}
}

func TestNewCmdFromInsideIssueWorktree(t *testing.T) {
	dir := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dir); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "new", "bug/foo", "--repo", dir); err != nil {
		t.Fatalf("pmt new: %v", err)
	}

	worktreePath := git.ComputeWorktreePath(dir, "", "bug", "foo")
	chdir(t, worktreePath)

	// Creating a second issue from inside the first issue's worktree must
	// still land in the main repo, not get confused into operating on the
	// worktree as if it were its own target repo.
	out, err := execRoot(t, "new", "bug/bar") // no --repo
	if err != nil {
		t.Fatalf("pmt new: %v", err)
	}
	if !strings.Contains(out, "bug/bar") {
		t.Errorf("output %q should mention the newly created bug/bar issue", out)
	}

	list, err := execRoot(t, "list", "--repo", dir)
	if err != nil {
		t.Fatalf("pmt list: %v", err)
	}
	if !strings.Contains(list, "bug/foo") || !strings.Contains(list, "bug/bar") {
		t.Errorf("pmt list --repo %s = %q, want both bug/foo and bug/bar", dir, list)
	}
}

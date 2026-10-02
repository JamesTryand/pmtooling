package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// ACCEPTANCE.md G6: where worktrees live is a per-machine matter, resolved
// from the repo-local .pmt.yaml and the user's own nickname map; commands
// neither assume one layout nor write a machine path into any tracked file.
func TestPullAndSyncHonourWorktreesDirAndNicknameResolution(t *testing.T) {
	root, bare := pushFixture(t)
	if out, err := execRoot(t, "push", "--all", "--repo", root); err != nil {
		t.Fatalf("push --all: %v\n%s", err, out)
	}
	b := machineBClone(t, bare)

	// this machine keeps its worktrees somewhere non-default
	custom := filepath.Join(filepath.Dir(b), "custom-wts")
	if err := os.WriteFile(filepath.Join(b, ".pmt.yaml"), []byte("worktrees_dir: ../custom-wts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// and refers to the repo by a nickname
	t.Setenv("PMT_CONFIG_HOME", t.TempDir())
	if out, err := execRoot(t, "repo", "add", "farm", b); err != nil {
		t.Fatalf("repo add: %v\n%s", err, out)
	}

	out, err := execRoot(t, "pull", "bug/one", "--repo", "farm")
	if err != nil {
		t.Fatalf("pull via nickname: %v\n%s", err, out)
	}
	want := filepath.Join(custom, "bug", "one")
	if _, err := os.Stat(filepath.Join(want, "README.md")); err != nil {
		t.Errorf("worktree not at the configured location %s: %v", want, err)
	}
	if _, err := os.Stat(git.ComputeWorktreePath(b, "", "bug", "one")); !os.IsNotExist(err) {
		t.Errorf("a worktree was also made at the default sibling location (stat err = %v)", err)
	}

	// sync sees the same layout: nothing to do, and a stray directory is
	// found under the configured root, not the default one
	if out, err := execRoot(t, "sync", "--repo", "farm"); err != nil || !strings.Contains(out, "up-to-date") {
		t.Fatalf("sync via nickname = %q, %v; want up-to-date, exit 0", out, err)
	}
	stray := filepath.Join(custom, "stray")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stray, ".git"), []byte("gitdir: nowhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = execRoot(t, "sync", "--repo", "farm")
	if exitCodeOf(err) != ExitCodeAttention || !strings.Contains(out, "orphaned") {
		t.Errorf("stray dir under the configured root: exit %d, output %q; want orphaned, exit 2", exitCodeOf(err), out)
	}

	// no machine path is written into any tracked file or ref
	machinePath := filepath.Dir(b)
	for _, branch := range []string{"bug/one", "pmt/template/bug"} {
		files, err := git.LsTreeFiles(b, "refs/heads/"+branch)
		if err != nil {
			t.Fatal(err)
		}
		for path := range files {
			content, err := git.ReadBlob(b, "refs/heads/"+branch+":"+path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(content), machinePath) {
				t.Errorf("%s:%s contains this machine's path", branch, path)
			}
		}
	}
}

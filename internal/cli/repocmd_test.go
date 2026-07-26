package cli

import (
	"strings"
	"testing"
)

// isolatedUserConfig points the user-level config at a fresh temp dir for
// the duration of the test, so `pmt repo add/remove/set-default` (which
// write that file) never touch a real developer's config.
func isolatedUserConfig(t *testing.T) {
	t.Helper()
	t.Setenv("PMT_CONFIG_HOME", t.TempDir())
}

func TestRepoAddAndList(t *testing.T) {
	isolatedUserConfig(t)
	dir := initRepo(t)

	if _, err := execRoot(t, "repo", "add", "clientA", dir); err != nil {
		t.Fatalf("pmt repo add: %v", err)
	}
	out, err := execRoot(t, "repo", "list")
	if err != nil {
		t.Fatalf("pmt repo list: %v", err)
	}
	if !strings.Contains(out, "clientA") || !strings.Contains(out, dir) {
		t.Errorf("output %q should list clientA -> %s", out, dir)
	}
}

func TestRepoAddInvalidPath(t *testing.T) {
	isolatedUserConfig(t)
	notARepo := t.TempDir()
	if _, err := execRoot(t, "repo", "add", "clientA", notARepo); err == nil {
		t.Fatal("expected error adding a path that isn't a git repository")
	}
}

func TestRepoAddCollisionWithoutForce(t *testing.T) {
	isolatedUserConfig(t)
	dirA := initRepo(t)
	dirB := initRepo(t)

	if _, err := execRoot(t, "repo", "add", "clientA", dirA); err != nil {
		t.Fatalf("pmt repo add (first): %v", err)
	}
	if _, err := execRoot(t, "repo", "add", "clientA", dirB); err == nil {
		t.Fatal("expected error overwriting an existing nickname without --force")
	}
}

func TestRepoAddCollisionWithForce(t *testing.T) {
	isolatedUserConfig(t)
	dirA := initRepo(t)
	dirB := initRepo(t)

	if _, err := execRoot(t, "repo", "add", "clientA", dirA); err != nil {
		t.Fatalf("pmt repo add (first): %v", err)
	}
	if _, err := execRoot(t, "repo", "add", "clientA", dirB, "--force"); err != nil {
		t.Fatalf("pmt repo add --force: %v", err)
	}
	out, err := execRoot(t, "repo", "list")
	if err != nil {
		t.Fatalf("pmt repo list: %v", err)
	}
	if !strings.Contains(out, dirB) || strings.Contains(out, dirA) {
		t.Errorf("output %q should reflect the forced overwrite (dirB, not dirA)", out)
	}
}

func TestRepoListEmpty(t *testing.T) {
	isolatedUserConfig(t)
	out, err := execRoot(t, "repo", "list")
	if err != nil {
		t.Fatalf("pmt repo list: %v", err)
	}
	if !strings.Contains(out, "No repos configured") {
		t.Errorf("output %q should indicate no repos are configured yet", out)
	}
}

func TestRepoRemove(t *testing.T) {
	isolatedUserConfig(t)
	dir := initRepo(t)
	if _, err := execRoot(t, "repo", "add", "clientA", dir); err != nil {
		t.Fatalf("pmt repo add: %v", err)
	}
	if _, err := execRoot(t, "repo", "remove", "clientA"); err != nil {
		t.Fatalf("pmt repo remove: %v", err)
	}
	out, err := execRoot(t, "repo", "list")
	if err != nil {
		t.Fatalf("pmt repo list: %v", err)
	}
	if strings.Contains(out, "clientA") {
		t.Errorf("output %q should no longer list clientA", out)
	}
}

func TestRepoRemoveUnknown(t *testing.T) {
	isolatedUserConfig(t)
	if _, err := execRoot(t, "repo", "remove", "nope"); err == nil {
		t.Fatal("expected error removing an unknown nickname")
	}
}

func TestRepoSetDefault(t *testing.T) {
	isolatedUserConfig(t)
	dirA := initRepo(t)
	dirB := initRepo(t)
	if _, err := execRoot(t, "repo", "add", "clientA", dirA); err != nil {
		t.Fatalf("pmt repo add clientA: %v", err)
	}
	if _, err := execRoot(t, "repo", "add", "clientB", dirB); err != nil {
		t.Fatalf("pmt repo add clientB: %v", err)
	}
	if _, err := execRoot(t, "repo", "set-default", "clientB"); err != nil {
		t.Fatalf("pmt repo set-default: %v", err)
	}

	out, err := execRoot(t, "repo", "list")
	if err != nil {
		t.Fatalf("pmt repo list: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "clientB ") && !strings.Contains(line, "(default)") {
			t.Errorf("clientB line %q should be marked (default)", line)
		}
		if strings.HasPrefix(line, "clientA ") && strings.Contains(line, "(default)") {
			t.Errorf("clientA line %q should not be marked (default)", line)
		}
	}
}

func TestRepoSetDefaultUnknown(t *testing.T) {
	isolatedUserConfig(t)
	if _, err := execRoot(t, "repo", "set-default", "nope"); err == nil {
		t.Fatal("expected error setting default to an unknown nickname")
	}
}

func TestRepoRemoveClearsDefaultMarker(t *testing.T) {
	isolatedUserConfig(t)
	dir := initRepo(t)
	if _, err := execRoot(t, "repo", "add", "clientA", dir); err != nil {
		t.Fatalf("pmt repo add: %v", err)
	}
	if _, err := execRoot(t, "repo", "set-default", "clientA"); err != nil {
		t.Fatalf("pmt repo set-default: %v", err)
	}
	out, err := execRoot(t, "repo", "remove", "clientA")
	if err != nil {
		t.Fatalf("pmt repo remove: %v", err)
	}
	if !strings.Contains(out, "default_repo has been cleared") {
		t.Errorf("output %q should note that default_repo was cleared", out)
	}
}

// TestRepoAddThenUseAsRepoFlag proves repo add's nickname is immediately
// usable by --repo elsewhere, closing the loop this whole subcommand
// exists for (previously only hand-editable YAML).
func TestRepoAddThenUseAsRepoFlag(t *testing.T) {
	isolatedUserConfig(t)
	dir := initRepo(t)
	if _, err := execRoot(t, "repo", "add", "clientA", dir); err != nil {
		t.Fatalf("pmt repo add: %v", err)
	}
	if _, err := execRoot(t, "template", "new", "bug", "--repo", "clientA"); err != nil {
		t.Fatalf("pmt template new --repo clientA: %v", err)
	}
	out, err := execRoot(t, "template", "list", "--repo", "clientA")
	if err != nil {
		t.Fatalf("pmt template list --repo clientA: %v", err)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("output %q should list the bug template via the clientA nickname", out)
	}
}

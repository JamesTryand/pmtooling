package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesTryand/pmtooling/internal/git"
)

func TestTemplateNewCmdFromImport(t *testing.T) {
	src := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", src); err != nil {
		t.Fatalf("pmt template new (source): %v", err)
	}

	dst := initRepo(t)
	out, err := execRoot(t, "template", "new", "bug", "--from", src, "--repo", dst)
	if err != nil {
		t.Fatalf("pmt template new --from: %v", err)
	}
	if !strings.Contains(out, "bug") || !strings.Contains(out, src) {
		t.Errorf("output %q should mention the imported template and source", out)
	}

	listOut, err := execRoot(t, "template", "list", "--repo", dst)
	if err != nil {
		t.Fatalf("pmt template list: %v", err)
	}
	if !strings.Contains(listOut, "bug") {
		t.Errorf("output %q should list the imported bug template", listOut)
	}
}

func TestTemplateNewCmdFromNickname(t *testing.T) {
	isolatedUserConfig(t)
	src := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", src); err != nil {
		t.Fatalf("pmt template new (source): %v", err)
	}
	if _, err := execRoot(t, "repo", "add", "sourceRepo", src); err != nil {
		t.Fatalf("pmt repo add: %v", err)
	}

	dst := initRepo(t)
	out, err := execRoot(t, "template", "new", "bug", "--from", "sourceRepo", "--repo", dst)
	if err != nil {
		t.Fatalf("pmt template new --from sourceRepo: %v", err)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("output %q should mention the imported template", out)
	}
}

func TestTemplateNewCmdFromCollisionWithExistingLocal(t *testing.T) {
	src := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", src); err != nil {
		t.Fatalf("pmt template new (source): %v", err)
	}
	dst := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dst); err != nil {
		t.Fatalf("pmt template new (dest, pre-existing): %v", err)
	}

	if _, err := execRoot(t, "template", "new", "bug", "--from", src, "--repo", dst); err == nil {
		t.Fatal("expected error importing over an existing local template")
	}
}

func TestTemplateUpdateCmdRequiresFrom(t *testing.T) {
	dst := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", dst); err != nil {
		t.Fatalf("pmt template new: %v", err)
	}
	if _, err := execRoot(t, "template", "update", "bug", "--repo", dst); err == nil {
		t.Fatal("expected error when --from is omitted")
	}
}

func TestTemplateUpdateCmdFastForward(t *testing.T) {
	src := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", src); err != nil {
		t.Fatalf("pmt template new (source): %v", err)
	}
	dst := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--from", src, "--repo", dst); err != nil {
		t.Fatalf("pmt template new --from: %v", err)
	}

	// Advance the source template via a scratch worktree.
	wt := filepath.Join(t.TempDir(), "wt")
	if err := git.WorktreeAdd(src, wt, "pmt/template/bug"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, "NOTES.md"), []byte("new guidance"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(wt, "add", "NOTES.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(wt, "commit", "-q", "-m", "advance"); err != nil {
		t.Fatal(err)
	}

	out, err := execRoot(t, "template", "update", "bug", "--from", src, "--repo", dst)
	if err != nil {
		t.Fatalf("pmt template update: %v", err)
	}
	if !strings.Contains(out, "fast-forwarded") {
		t.Errorf("output %q should mention the fast-forward", out)
	}
}

func TestTemplateUpdateCmdAlreadyUpToDate(t *testing.T) {
	src := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", src); err != nil {
		t.Fatalf("pmt template new (source): %v", err)
	}
	dst := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--from", src, "--repo", dst); err != nil {
		t.Fatalf("pmt template new --from: %v", err)
	}

	out, err := execRoot(t, "template", "update", "bug", "--from", src, "--repo", dst)
	if err != nil {
		t.Fatalf("pmt template update: %v", err)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("output %q should say already up to date", out)
	}
}

func TestTemplateUpdateCmdDiverged(t *testing.T) {
	src := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--repo", src); err != nil {
		t.Fatalf("pmt template new (source): %v", err)
	}
	dst := initRepo(t)
	if _, err := execRoot(t, "template", "new", "bug", "--from", src, "--repo", dst); err != nil {
		t.Fatalf("pmt template new --from: %v", err)
	}

	advance := func(dir, filename, content string) {
		t.Helper()
		wt := filepath.Join(t.TempDir(), "wt")
		if err := git.WorktreeAdd(dir, wt, "pmt/template/bug"); err != nil {
			t.Fatalf("WorktreeAdd: %v", err)
		}
		if err := os.WriteFile(filepath.Join(wt, filename), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := git.Run(wt, "add", filename); err != nil {
			t.Fatal(err)
		}
		if _, err := git.Run(wt, "commit", "-q", "-m", "advance"); err != nil {
			t.Fatal(err)
		}
	}
	advance(src, "SOURCE.md", "source change")
	advance(dst, "LOCAL.md", "local change")

	out, err := execRoot(t, "template", "update", "bug", "--from", src, "--repo", dst)
	if err != nil {
		t.Fatalf("pmt template update: %v", err)
	}
	if !strings.Contains(out, "diverged") || !strings.Contains(out, "pmt/template-incoming/bug") {
		t.Errorf("output %q should explain the divergence and name the incoming ref", out)
	}

	// Local template must be untouched -- LOCAL.md still present, SOURCE.md not merged in.
	localTip, err := git.Run(dst, "rev-parse", "refs/heads/pmt/template/bug")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(dst, "show", localTip+":LOCAL.md"); err != nil {
		t.Errorf("local template should still have LOCAL.md: %v", err)
	}
	if _, err := git.Run(dst, "show", localTip+":SOURCE.md"); err == nil {
		t.Error("local template should NOT have SOURCE.md merged in automatically")
	}
}

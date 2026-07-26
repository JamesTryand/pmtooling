package git

import "testing"

func TestRevParseQuietExisting(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "hello")
	head, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	sha, ok, err := RevParseQuiet(dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParseQuiet: %v", err)
	}
	if !ok || sha != head {
		t.Errorf("RevParseQuiet(HEAD) = (%q, %v), want (%q, true)", sha, ok, head)
	}
}

func TestRevParseQuietMissing(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "hello")

	_, ok, err := RevParseQuiet(dir, "refs/heads/does-not-exist")
	if err != nil {
		t.Fatalf("RevParseQuiet: %v", err)
	}
	if ok {
		t.Error("expected ok=false for a nonexistent ref")
	}
}

func TestTreeEntrySHA(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "hello")
	head, err := Run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	sha, ok, err := TreeEntrySHA(dir, "HEAD", "f.txt")
	if err != nil {
		t.Fatalf("TreeEntrySHA: %v", err)
	}
	if !ok || sha == "" {
		t.Fatalf("TreeEntrySHA(HEAD, f.txt) = (%q, %v), want a real blob SHA", sha, ok)
	}

	_, ok, err = TreeEntrySHA(dir, head, "nonexistent.txt")
	if err != nil {
		t.Fatalf("TreeEntrySHA: %v", err)
	}
	if ok {
		t.Error("expected ok=false for a nonexistent path")
	}
}

func TestLsTree(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "a")
	commitFile(t, dir, "b.txt", "b")

	entries, err := LsTree(dir, "HEAD")
	if err != nil {
		t.Fatalf("LsTree: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("LsTree = %+v, want 2 entries", entries)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
		if e.Type != "blob" {
			t.Errorf("entry %q has Type %q, want blob", e.Name, e.Type)
		}
	}
	if !names["a.txt"] || !names["b.txt"] {
		t.Errorf("LsTree = %+v, want entries for a.txt and b.txt", entries)
	}
}

func TestReadBlobPreservesExactBytes(t *testing.T) {
	dir := initRepo(t)
	content := "line one\nline two\n\ntrailing blank line above\n"
	commitFile(t, dir, "f.txt", content)

	got, err := ReadBlob(dir, "HEAD:f.txt")
	if err != nil {
		t.Fatalf("ReadBlob: %v", err)
	}
	if string(got) != content {
		t.Errorf("ReadBlob = %q, want exact original content %q", got, content)
	}
}

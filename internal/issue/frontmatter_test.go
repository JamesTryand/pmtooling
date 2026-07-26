package issue

import (
	"strings"
	"testing"
)

const sampleReadme = "---\n" +
	"pmt:\n" +
	"  type: bug\n" +
	"  title: \"\"\n" +
	"  branch: \"\"\n" +
	"  status: open\n" +
	"  created: \"\"\n" +
	"  template_ref: \"\"\n" +
	"---\n" +
	"\n" +
	"# New issue\n" +
	"\n" +
	"Describe the issue here.\n"

func TestParseValidFrontMatter(t *testing.T) {
	meta, body, ok := Parse([]byte(sampleReadme))
	if !ok {
		t.Fatal("expected ok=true for well-formed front matter")
	}
	if meta.Type != "bug" {
		t.Errorf("Type = %q, want bug", meta.Type)
	}
	if meta.Status != "open" {
		t.Errorf("Status = %q, want open", meta.Status)
	}
	if !strings.Contains(body, "# New issue") {
		t.Errorf("body missing expected content: %q", body)
	}
}

func TestParseNoFrontMatter(t *testing.T) {
	content := "# Just a plain README\n\nNo front matter here.\n"
	meta, body, ok := Parse([]byte(content))
	if ok {
		t.Fatal("expected ok=false for content with no front matter")
	}
	if meta != (Meta{}) {
		t.Errorf("expected zero-value Meta, got %+v", meta)
	}
	if body != content {
		t.Errorf("body = %q, want unchanged input %q", body, content)
	}
}

func TestParseMalformedYAML(t *testing.T) {
	content := "---\npmt: [this is not a map\n---\nbody\n"
	_, _, ok := Parse([]byte(content))
	if ok {
		t.Fatal("expected ok=false for malformed YAML front matter")
	}
}

func TestParseCRLF(t *testing.T) {
	content := strings.ReplaceAll(sampleReadme, "\n", "\r\n")
	meta, body, ok := Parse([]byte(content))
	if !ok {
		t.Fatal("expected ok=true when front matter uses CRLF line endings")
	}
	if meta.Type != "bug" {
		t.Errorf("Type = %q, want bug", meta.Type)
	}
	if !strings.Contains(body, "New issue") {
		t.Errorf("body missing expected content: %q", body)
	}
}

func TestRenderRoundTrip(t *testing.T) {
	meta := Meta{
		Type:        "bug",
		Title:       "0001",
		Branch:      "bug/0001",
		Status:      "open",
		Created:     "2026-07-26T00:00:00Z",
		TemplateRef: "abc123",
	}
	body := "\n# New issue\n\nDescribe the issue here.\n"

	rendered, err := Render(meta, body)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(rendered), "---\n") {
		t.Errorf("rendered output should start with ---\\n: %q", rendered)
	}

	gotMeta, gotBody, ok := Parse(rendered)
	if !ok {
		t.Fatal("expected ok=true parsing Render's own output")
	}
	if gotMeta != meta {
		t.Errorf("round-tripped meta = %+v, want %+v", gotMeta, meta)
	}
	if gotBody != body {
		t.Errorf("round-tripped body = %q, want %q", gotBody, body)
	}
}

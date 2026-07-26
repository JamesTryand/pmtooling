package scaffold

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderProducesExpectedFiles(t *testing.T) {
	files, err := Render(Data{Type: "bug"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := []string{"README.md", "CLAUDE.md", ".gitignore", ".claude/settings.json"}
	if len(files) != len(want) {
		t.Fatalf("Render returned %d files, want %d", len(files), len(want))
	}
	for i, w := range want {
		if files[i].Path != w {
			t.Errorf("files[%d].Path = %q, want %q", i, files[i].Path, w)
		}
	}
}

func TestRenderSubstitutesType(t *testing.T) {
	files, err := Render(Data{Type: "bug"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, f := range files {
		if f.Path == "README.md" && !strings.Contains(string(f.Content), "type: bug") {
			t.Errorf("README.md missing substituted type:\n%s", f.Content)
		}
		if f.Path == "CLAUDE.md" && !strings.Contains(string(f.Content), "`bug`") {
			t.Errorf("CLAUDE.md missing substituted type:\n%s", f.Content)
		}
	}
}

func TestRenderReadmeHasFrontMatter(t *testing.T) {
	files, err := Render(Data{Type: "bug"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var readme string
	for _, f := range files {
		if f.Path == "README.md" {
			readme = string(f.Content)
		}
	}
	if !strings.HasPrefix(readme, "---\n") {
		t.Errorf("README.md should start with a --- front-matter delimiter:\n%s", readme)
	}
	if strings.Count(readme, "---\n") < 2 {
		t.Errorf("README.md should have opening and closing --- delimiters:\n%s", readme)
	}
}

func TestRenderSettingsIsValidJSON(t *testing.T) {
	files, err := Render(Data{Type: "bug"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, f := range files {
		if f.Path == ".claude/settings.json" {
			var v map[string]any
			if err := json.Unmarshal(f.Content, &v); err != nil {
				t.Errorf(".claude/settings.json is not valid JSON: %v (content: %q)", err, f.Content)
			}
		}
	}
}

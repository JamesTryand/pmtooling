// Package scaffold holds the embedded starter files written into every
// new template branch by `pmt template new`. See doc/templates.md's
// starter-scaffold-files section.
package scaffold

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

//go:embed README.md.tmpl
var readmeTmpl string

//go:embed CLAUDE.md.tmpl
var claudeMDTmpl string

//go:embed gitignore.tmpl
var gitignoreTmpl string

//go:embed claude_settings.json.tmpl
var claudeSettingsTmpl string

// Data is the substitution data available to scaffold templates.
type Data struct {
	Type string
}

// File is one starter file to be written into a new template branch's tree.
type File struct {
	Path    string // repo-relative path, e.g. ".claude/settings.json"
	Content []byte
}

var specs = []struct {
	path string
	tmpl string
}{
	{"README.md", readmeTmpl},
	{"CLAUDE.md", claudeMDTmpl},
	{".gitignore", gitignoreTmpl},
	{".claude/settings.json", claudeSettingsTmpl},
}

// Render executes all starter templates with data and returns the
// resulting files, in a stable order.
func Render(data Data) ([]File, error) {
	files := make([]File, 0, len(specs))
	for _, s := range specs {
		content, err := execute(s.path, s.tmpl, data)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: s.path, Content: content})
	}
	return files, nil
}

func execute(name, tmplText string, data Data) ([]byte, error) {
	t, err := template.New(name).Parse(tmplText)
	if err != nil {
		return nil, fmt.Errorf("parsing scaffold template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing scaffold template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

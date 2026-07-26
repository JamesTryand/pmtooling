// Package issue implements pmt new: naming/validation, auto-title
// generation, README front-matter stamping, and worktree creation. See
// doc/commands.md#pmt-new and doc/templates.md#readme-front-matter-schema.
package issue

import (
	"bytes"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Meta is the pmt front-matter schema embedded in every issue's README.md.
// See doc/templates.md#readme-front-matter-schema.
type Meta struct {
	Type        string `yaml:"type"`
	Title       string `yaml:"title"`
	Branch      string `yaml:"branch"`
	Status      string `yaml:"status"`
	Created     string `yaml:"created"`
	TemplateRef string `yaml:"template_ref"`
	Closed      string `yaml:"closed,omitempty"` // RFC3339; set by `pmt close`, cleared by `pmt reopen`
}

type frontMatterRoot struct {
	Pmt Meta `yaml:"pmt"`
}

// frontMatterRe splits a "---"-delimited front matter block from the
// remaining body. \r? tolerates CRLF line endings, since a checked-out
// worktree's files may have been converted by core.autocrlf.
var frontMatterRe = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n(.*)$`)

// Parse splits content into pmt metadata and the remaining body. If
// content has no recognizable "---"-delimited front matter block, or the
// block fails to parse as YAML, ok is false and body is the entire input
// unchanged. Callers (pmt new) insert a fresh block in that case rather
// than erroring, matching doc/templates.md's "inserting a fresh block if
// absent" behavior — this also gracefully handles a hand-edited or
// corrupted template README.
func Parse(content []byte) (meta Meta, body string, ok bool) {
	m := frontMatterRe.FindSubmatch(content)
	if m == nil {
		return Meta{}, string(content), false
	}
	var root frontMatterRoot
	if err := yaml.Unmarshal(m[1], &root); err != nil {
		return Meta{}, string(content), false
	}
	return root.Pmt, string(m[2]), true
}

// Render serializes meta as a "---"-delimited front matter block followed
// by body.
func Render(meta Meta, body string) ([]byte, error) {
	yamlBytes, err := yaml.Marshal(frontMatterRoot{Pmt: meta})
	if err != nil {
		return nil, fmt.Errorf("serializing front matter: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	buf.WriteString(body)
	return buf.Bytes(), nil
}

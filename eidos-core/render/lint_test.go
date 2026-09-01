// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/render"
)

// tree returns a one-template tree.
func tree(name, src string) fstest.MapFS {
	return fstest.MapFS{name: &fstest.MapFile{Data: []byte(src)}}
}

// The lint is the static half of the marker rule: every declared
// template parses against the merged vocabulary before any run
// exists, and a body-claiming template carries its marker or
// fails here rather than in a diff.
func TestLint(t *testing.T) {
	t.Parallel()

	linter := func(t *testing.T) *render.Pass {
		t.Helper()

		l := language()
		l.Funcs = template.FuncMap{"shout": func(s string) string { return s }}
		p, err := render.New("printer", l)
		assert.NoError(t, err, "the language composes")
		return p
	}

	t.Run("a valid tree passes whole", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("method1.tpl", "\t{{shout .Data.x}}()\n{{slots}}"), nil, nil,
		)
		assert.Empty(t, findings, "marker placed, vocabulary known")
	})

	t.Run("a named marker alone suffices", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("method1.tpl", `{{slot "checks"}}`), nil, nil,
		)
		assert.Empty(t, findings, "named placement is placement")
	})

	t.Run("a template that does not parse is a finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(tree("broken.tpl", "{{if}}"), nil, nil)
		assert.Length(t, findings, 1, "one finding per template")
		assert.Contains(t, findings[0].Error(), "broken.tpl", "naming the file")
	})

	t.Run("a call outside the vocabulary is a finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("method1.tpl", "{{mystery .}}{{slots}}"), nil, nil,
		)
		assert.Length(t, findings, 1, "the unknown name reports")
		assert.Contains(t, findings[0].Error(), "mystery", "naming the function")
	})

	t.Run("the plugin's own helpers are vocabulary", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("method1.tpl", "{{mine .}}{{slots}}"),
			template.FuncMap{"mine": func(any) string { return "" }}, nil,
		)
		assert.Empty(t, findings, "declared helpers resolve")
	})

	t.Run("a dropped marker is a finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(tree("bare.tpl", "\treturn nil\n"), nil, nil)
		assert.Length(t, findings, 1, "the marker rule is static here")
		assert.Contains(t, findings[0].Error(), "bare.tpl", "naming the template")
		assert.Contains(t, findings[0].Error(), render.BuiltinSlots,
			"and the marker it lacks")
	})

	t.Run("an undeclared shadow is a finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("method1.tpl", "{{slots}}"),
			template.FuncMap{"shout": func(s string) string { return s }}, nil,
		)
		assert.Length(t, findings, 1, "the shared name needs the declaration")
		assert.Contains(t, findings[0].Error(), "shout", "naming the helper")
	})

	t.Run("an override of nothing shared is a finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("method1.tpl", "{{slots}}"), nil, []string{"ghost"},
		)
		assert.Length(t, findings, 1, "an override replaces something or lies")
		assert.Contains(t, findings[0].Error(), "ghost", "naming the claim")
	})
}

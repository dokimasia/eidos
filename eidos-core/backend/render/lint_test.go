// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
)

// tree returns a one-template tree.
func tree(name, src string) fstest.MapFS {
	return fstest.MapFS{name: &fstest.MapFile{Data: []byte(src)}}
}

// unreadable is a tree that lists its templates and refuses to open
// one of them, which is what a file the walk names and the read
// cannot serve looks like.
type unreadable struct {
	files fstest.MapFS
	deny  string
}

func (u unreadable) Open(name string) (fs.File, error) {
	if name == u.deny {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return u.files.Open(name)
}

func (u unreadable) ReadDir(name string) ([]fs.DirEntry, error) {
	return u.files.ReadDir(name)
}

// sealed is a tree that refuses to open its own root, so the walk
// fails before it names a single template.
type sealed struct{}

func (sealed) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
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

	t.Run("a declared shadow is vocabulary", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("method1.tpl", "{{shout .Data.x}}{{slots}}"),
			template.FuncMap{"shout": func(s string) string { return s }},
			[]string{"shout"},
		)
		assert.Empty(t, findings,
			"a declared override replaces the shared name it claims, "+
				"and the template calling it resolves")
	})

	t.Run("a helper claiming a builtin's name is a finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("method1.tpl", "{{slots}}"),
			template.FuncMap{render.BuiltinBody: func(any) string { return "" }},
			nil,
		)
		assert.Length(t, findings, 1, "the builtins' names are the pass's own")
		assert.Contains(t, findings[0].Error(), render.BuiltinBody,
			"naming the claim")
	})

	t.Run("a template the tree will not open is a finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(unreadable{
			files: tree("method1.tpl", "{{slots}}"), deny: "method1.tpl",
		}, nil, nil)
		assert.Length(t, findings, 1, "a template the lint cannot read is unchecked")
		assert.Contains(t, findings[0].Error(), "reading method1.tpl",
			"naming the file and the step that refused")
	})

	t.Run("a tree that will not list is one finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(sealed{}, nil, nil)
		assert.Length(t, findings, 1, "a tree nothing can walk is one fault")
		assert.Contains(t, findings[0].Error(), "walking the tree",
			"naming the step that refused")
	})

	t.Run("the marker counts wherever the parse tree holds it", func(t *testing.T) {
		t.Parallel()

		placements := []struct{ name, src string }{
			{name: "inside an if", src: "{{if .Decl}}{{slots}}{{end}}"},
			{name: "inside a range", src: "{{range .Decl}}{{slots}}{{end}}"},
			{name: "inside a with", src: "{{with .Decl}}{{slots}}{{end}}"},
			{name: "inside a nested pipeline", src: `{{printf "%s" (slots)}}`},
		}
		for _, tt := range placements {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				findings := linter(t).Lint(tree("method1.tpl", tt.src), nil, nil)
				assert.Empty(t, findings,
					"the parse tree is walked, so a marker anywhere in it counts")
			})
		}
	})

	t.Run("a branch holding no marker is a finding", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("bare.tpl", "{{if .Decl}}\treturn nil\n{{end}}"), nil, nil,
		)
		assert.Length(t, findings, 1,
			"a branch is not a placement, and neither is the else it never declared")
		assert.Contains(t, findings[0].Error(), render.BuiltinSlots,
			"naming the marker it lacks")
	})

	t.Run("a marker in a comment does not count", func(t *testing.T) {
		t.Parallel()

		findings := linter(t).Lint(
			tree("bare.tpl", "{{/* {{slots}} */}}\treturn nil\n"), nil, nil,
		)
		assert.Length(t, findings, 1,
			"the parse tree is walked rather than the text")
		assert.Contains(t, findings[0].Error(), "bare.tpl", "naming the template")
	})
}

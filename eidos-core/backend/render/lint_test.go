// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/backend/render"
)

// The lint's fixture: the helpers a template calls and the templates
// a finding names.
const (
	// shoutHelper is the language's shared helper.
	shoutHelper = "shout"
	// mineHelper is a helper of the plugin's own.
	mineHelper = "mine"
	// mysteryHelper is a name no vocabulary declares.
	mysteryHelper = "mystery"
	// ghostOverride is an override of a name nothing shares.
	ghostOverride = "ghost"
	// brokenTpl and bareTpl name the templates a finding reports.
	brokenTpl = "broken.tpl"
	bareTpl   = "bare.tpl"
)

// lintAllocs is one lint of a tree of one template that places its
// marker: the walk of the tree, the read of the template, and its parse
// against the merged vocabulary.
const lintAllocs = 67

// unreadable is a tree that lists its templates and refuses to open
// one of them: a file the walk names and the read cannot serve.
type unreadable struct {
	files fstest.MapFS
	deny  string
}

// Open opens every file except the denied one.
func (u unreadable) Open(name string) (fs.File, error) {
	if name == u.deny {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return u.files.Open(name)
}

// ReadDir lists the files, the denied one included.
func (u unreadable) ReadDir(name string) ([]fs.DirEntry, error) {
	return u.files.ReadDir(name)
}

// sealed is a tree that refuses to open its own root, so the walk
// fails before it names a single template.
type sealed struct{}

// Open refuses every name.
func (sealed) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
}

// The lint is the static half of the marker rule: every template of a
// plugin's tree parses against the merged vocabulary before any run,
// and a template that places no marker is reported before a render
// strands a contribution.
func TestLint(t *testing.T) {
	t.Parallel()

	linter := func(t *testing.T) *render.Pass {
		t.Helper()

		p, _ := lintFixture(t)
		return p
	}
	slots := action(render.BuiltinSlots)

	t.Run("Lint", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a tree that places its marker", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(refName, "\t"+action(shoutHelper, ".Data."+markKey)+"()\n"+slots), nil, nil,
			)
			assert.Empty(t, findings, "the marker is placed and every helper resolves")
		})

		t.Run("returns nothing for a tree whose one marker names a slot", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(refName, action(render.BuiltinSlot, `"checks"`)), nil, nil,
			)
			assert.Empty(t, findings, "a named placement is a placement")
		})

		t.Run("returns a finding for a template that does not parse", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(tree(brokenTpl, "{{if}}"), nil, nil)
			assert.Length(t, findings, 1, "one finding per template")
			assert.Contains(t, findings[0].Error(), brokenTpl, "naming the file")
		})

		t.Run("returns a finding for a call outside the vocabulary", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(refName, action(mysteryHelper, ".")+slots), nil, nil,
			)
			assert.Length(t, findings, 1, "the unknown name is one finding")
			assert.Contains(t, findings[0].Error(), mysteryHelper, "naming the function")
		})

		t.Run("resolves the plugin's own helpers", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(refName, action(mineHelper, ".")+slots),
				template.FuncMap{mineHelper: func(any) string { return "" }}, nil,
			)
			assert.Empty(t, findings, "a declared helper resolves")
		})

		t.Run("returns a finding for a template that places no marker", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(tree(bareTpl, "\treturn nil\n"), nil, nil)
			assert.Length(t, findings, 1, "the marker rule is checked statically")
			assert.Contains(t, findings[0].Error(), bareTpl, "naming the template")
			assert.Contains(t, findings[0].Error(), render.BuiltinSlots,
				"naming the marker it lacks")
		})

		t.Run("returns a finding for a helper that shadows the shared vocabulary undeclared", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(refName, slots), template.FuncMap{shoutHelper: identity}, nil,
			)
			assert.Length(t, findings, 1, "the shared name needs the declaration")
			assert.Contains(t, findings[0].Error(), shoutHelper, "naming the helper")
		})

		t.Run("returns a finding for an override that replaces nothing shared", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(tree(refName, slots), nil, []string{ghostOverride})
			assert.Length(t, findings, 1, "an override replaces a shared name")
			assert.Contains(t, findings[0].Error(), ghostOverride, "naming the claim")
		})

		t.Run("resolves a declared override", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(refName, action(shoutHelper, ".Data."+markKey)+slots),
				template.FuncMap{shoutHelper: identity},
				[]string{shoutHelper},
			)
			assert.Empty(t, findings, "the override replaces the shared name it declares")
		})

		t.Run("returns a finding for a helper named after a builtin", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(refName, slots),
				template.FuncMap{render.BuiltinBody: func(any) string { return "" }},
				nil,
			)
			assert.Length(t, findings, 1, "the builtin names are the pass's own")
			assert.Contains(t, findings[0].Error(), render.BuiltinBody, "naming the claim")
		})

		t.Run("returns a finding for a template the tree cannot open", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(unreadable{
				files: tree(refName, slots), deny: refName,
			}, nil, nil)
			assert.Length(t, findings, 1, "a template the lint cannot read is unchecked")
			assert.Contains(t, findings[0].Error(), "reading "+refName,
				"naming the file and the step that failed")
		})

		t.Run("returns one finding for a tree that cannot be walked", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(sealed{}, nil, nil)
			assert.Length(t, findings, 1, "a tree the walk cannot list is one fault")
			assert.Contains(t, findings[0].Error(), "walking the tree",
				"naming the step that failed")
		})

		tests := []struct {
			name string
			give string
		}{
			{name: "counts a marker inside an if", give: "{{if .Decl}}" + slots + "{{end}}"},
			{name: "counts a marker inside a range", give: "{{range .Decl}}" + slots + "{{end}}"},
			{name: "counts a marker inside a with", give: "{{with .Decl}}" + slots + "{{end}}"},
			{
				name: "counts a marker inside a nested pipeline",
				give: action("printf", `"%s"`, "("+render.BuiltinSlots+")"),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				findings := linter(t).Lint(tree(refName, tt.give), nil, nil)
				assert.Empty(t, findings, "the parse tree is walked")
			})
		}

		t.Run("returns a finding for a branch that places no marker", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(bareTpl, "{{if .Decl}}\treturn nil\n{{end}}"), nil, nil,
			)
			assert.Length(t, findings, 1, "neither the branch nor its absent else places one")
			assert.Contains(t, findings[0].Error(), render.BuiltinSlots,
				"naming the marker it lacks")
		})

		t.Run("returns a finding for a template whose one marker is in a comment", func(t *testing.T) {
			t.Parallel()

			findings := linter(t).Lint(
				tree(bareTpl, "{{/* "+slots+" */}}\treturn nil\n"), nil, nil,
			)
			assert.Length(t, findings, 1, "the parse tree is walked, not the text")
			assert.Contains(t, findings[0].Error(), bareTpl, "naming the template")
		})
	})
}

// A lint of a tree of one template allocates within its ceiling in the
// ordinary run, which runs no benchmark.
func TestLintAllocs(t *testing.T) {
	p, marked := lintFixture(t)
	var findings []error
	assert.MaxAllocs(t, func() { findings = p.Lint(marked, nil, nil) }, lintAllocs,
		"Lint allocates the walk and the parse of the template")
	assert.Empty(t, findings, "Lint returns nothing for a tree that places its marker")
}

// BenchmarkLint measures the static check of a plugin's tree of one
// template, which the composition runs once per plugin.
func BenchmarkLint(b *testing.B) {
	b.Run("Lint", func(b *testing.B) {
		b.Run("a tree of one template that places its marker", func(b *testing.B) {
			p, marked := lintFixture(b)
			c := bench.Start(b).MaxAllocs(lintAllocs)
			defer c.End()
			var findings []error
			for c.Loop() {
				findings = p.Lint(marked, nil, nil)
			}
			assert.Empty(b, findings, "the marker is placed and every helper resolves")
		})
	})
}

// tree returns a one-template tree.
func tree(name, src string) fstest.MapFS {
	return fstest.MapFS{name: &fstest.MapFile{Data: []byte(src)}}
}

// identity is the helper body every fixture helper shares.
func identity(s string) string { return s }

// lintFixture returns a pass over the fixture language with the shared
// helper, and a tree of one template that calls the helper and places
// its marker.
func lintFixture(tb assert.TB) (*render.Pass, fs.FS) {
	tb.Helper()

	l := language()
	l.Funcs = helpers(template.FuncMap{shoutHelper: identity})
	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the language composes")
	return p, tree(refName, "\t"+action(shoutHelper, ".Data."+markKey)+"()\n"+action(render.BuiltinSlots))
}

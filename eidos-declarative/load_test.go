// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"errors"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugin/plugintest"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/declarative"
)

// The plugin directory of the cases: its name, its manifest, its template
// trees for the scripted language and for golang, and the families of its
// rules.
const (
	pluginName       = "acme-audit"
	manifestFile     = "plugin.yaml"
	functionTemplate = "function.tmpl"
	methodTemplate   = "method.tmpl"
	testTemplate     = "test.tmpl"
	fakeFunction     = "templates/fake/" + functionTemplate
	fakeMethod       = "templates/fake/" + methodTemplate
	fakeTest         = "templates/fake/" + testTemplate
	goFunction       = "templates/golang/" + functionTemplate
	goMethod         = "templates/golang/" + methodTemplate
	goTest           = "templates/golang/" + testTemplate
	goHelper         = "templates/golang/helper.tmpl"
	goTarget         = "golang"
	contractTag      = "contract"
	shapeTag         = "shape"
	testTag          = "test"
)

// The bodies of the templates of the cases.
const (
	functionBody = "{{.Data.Directive.sink}}({{printf \"%q\" .Decl.Name}})\n{{slots}}"
	methodBody   = "{{.Data.Directive.sink}}({{printf \"%q\" .Decl.Name}})\n{{slots}}" +
		"return {{.Decl.Receiver.Name}}.next.{{.Decl.Name}}()\n"
	helperBody = "{{define \"args\"}}{{range .}}{{.Name}}{{end}}{{end}}"
	testBody   = "{{use \"testing\"}}{{range .Records}}func Test{{.Decl.Name}}(t *testing.T) {}\n{{end}}"
)

// auditManifest is the manifest of the cases: the directive audit, a
// family of each cardinality, the option redact, and a rule of each pair of
// a subject kind and an emission, gated and bare.
const auditManifest = `format: 1
name: acme-audit
version: 0.3.0
directives:
    - name: audit
      doc: Generates code that records each call of the declaration.
      params:
          - key: level
            type: string
            choices: [debug, info, warn]
            required: true
            doc: The level of each record.
          - key: sink
            type: reference
            resolve: callable-in-scope
            required: true
            doc: The function that receives each record.
          - key: fields
            type: list
            of: int
            doc: The positions of the arguments that a record contains.
          - key: strict
            type: bool
            doc: When true, a record that fails stops the call.
outputs:
    - per: source
      word: audit
    - tag: contract
      per: package
      word: contract
    - tag: shape
      per: plan
      word: shape
    - tag: test
      per: source
      word: audit
      template: test.tmpl
provides: [audit]
options:
    - key: redact
      type: bool
      default: true
      doc: When true, a record leaves out the arguments of the call.
rules:
    - on: interface
      directive: audit
      emit: implementation
      template: method.tmpl
    - on: function
      directive: audit
      emit: mirror
      template: function.tmpl
    - on: interface
      output: contract
      emit: mirror
    - on: function
      output: contract
      emit: mirror
      template: function.tmpl
    - on: struct
      output: shape
      emit: mirror
    - on: function
      directive: audit
      output: test
      emit: mirror
`

// The brand and the plan of the composition that a fingerprint case
// builds, and the output directory of the plan, which its family per plan
// writes under.
const (
	auditBrand = "acme"
	auditPlan  = "audit"
	auditDir   = "gen"
)

// printer is the backend of the composition that a fingerprint case
// builds: a name and the target golang. Nothing invokes it.
type printer struct{}

// Name returns the name of the backend.
func (printer) Name() plugin.ID { return "printer" }

// Target returns golang.
func (printer) Target() plugin.Target { return goTarget }

// Load reads a plugin directory into a generator whose version folds the
// directory, and reports each fault of the directory at its file and line.
func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a generator with the name of the manifest", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, auditPlugin(t).Name(), plugin.ID(pluginName), "the generator has the manifest's name")
		})

		t.Run("returns the version of the manifest and the digest of the directory", func(t *testing.T) {
			t.Parallel()

			assert.Matches(t, versionOf(t, auditTree()), `^0\.3\.0\+[0-9a-f]{64}$`,
				"the version is the manifest's version, a plus sign and a SHA-256 in hexadecimal")
		})

		t.Run("returns a generator with a template tree for each directory below templates", func(t *testing.T) {
			t.Parallel()

			provider, is := auditPlugin(t).(plugin.TemplateProvider)
			assert.True(t, is, "the generator declares template trees")
			assert.Equal(t, provider.TemplateTargets(), []plugin.Target{"fake", goTarget},
				"the generator serves the target of each tree")
		})

		t.Run("returns a new generator on each call", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, auditTree())
			first, err := declarative.Load(dir)
			assert.NoError(t, err, "the first load succeeds")
			second, err := declarative.Load(dir)
			assert.NoError(t, err, "the second load succeeds")
			assert.NotEqual(t, second, first, "a plugin instance belongs to one workspace", assert.ByIdentity())
		})

		// Each edit of the directory is two cases: the version of the
		// generator, and the fingerprint of a composition that runs it.
		edits := []struct {
			version     string
			fingerprint string
			edit        func(files.Tree)
		}{
			{
				version:     "changes the version for an edit of one byte of the manifest",
				fingerprint: "changes the fingerprint for an edit of one byte of the manifest",
				edit:        func(tree files.Tree) { tree[manifestFile] = files.Text(auditManifest + "\n") },
			},
			{
				version:     "changes the version for an edit of one byte of a template",
				fingerprint: "changes the fingerprint for an edit of one byte of a template",
				edit:        func(tree files.Tree) { tree[goHelper] = files.Text(helperBody + " ") },
			},
			{
				version:     "changes the version for a template under another name",
				fingerprint: "changes the fingerprint for a template under another name",
				edit: func(tree files.Tree) {
					delete(tree, goHelper)
					tree["templates/golang/helpers.tmpl"] = files.Text(helperBody)
				},
			},
			{
				version:     "changes the version for a new template file",
				fingerprint: "changes the fingerprint for a new template file",
				edit:        func(tree files.Tree) { tree["templates/golang/extra.tmpl"] = files.Text(helperBody) },
			},
		}
		for _, tt := range edits {
			t.Run(tt.version, func(t *testing.T) {
				t.Parallel()

				edited := auditTree()
				tt.edit(edited)
				assert.NotEqual(t, versionOf(t, edited), versionOf(t, auditTree()), "the version folds the edit")
			})

			t.Run(tt.fingerprint, func(t *testing.T) {
				t.Parallel()

				edited := auditTree()
				tt.edit(edited)
				assert.NotEqual(t, fingerprintOf(t, edited), fingerprintOf(t, auditTree()),
					"the fingerprint folds the version")
			})
		}

		t.Run("keeps the version for a new file outside the manifest and the templates", func(t *testing.T) {
			t.Parallel()

			edited := auditTree()
			edited["README.md"] = files.Text("# acme-audit\n")
			assert.Equal(t, versionOf(t, edited), versionOf(t, auditTree()), "the version folds no other file")
		})

		t.Run("keeps the fingerprint for a new file outside the manifest and the templates", func(t *testing.T) {
			t.Parallel()

			edited := auditTree()
			edited["README.md"] = files.Text("# acme-audit\n")
			assert.Equal(t, fingerprintOf(t, edited), fingerprintOf(t, auditTree()),
				"the fingerprint folds no other file")
		})

		t.Run("returns an Error for a relative path", func(t *testing.T) {
			t.Parallel()

			_, err := declarative.Load(filepath.Join("plugins", pluginName))
			fault := assert.ErrorAs[*declarative.Error](t, err, "the fault is an Error")
			assert.Equal(t, fault.Msg, "the plugin directory is not an absolute path", "the Error describes the fault")
		})

		t.Run("returns an Error at the directory for a directory without a manifest", func(t *testing.T) {
			t.Parallel()

			got := loadFaults(t, files.Tree{"README.md": files.Text("# acme-audit\n")})
			assert.Equal(t, got, []declarative.Error{{File: ".", Msg: "the directory has no plugin.yaml"}},
				"the fault is at the directory")
		})

		t.Run("returns an Error at the manifest for a manifest that does not read", func(t *testing.T) {
			t.Parallel()

			got := loadFaults(t, files.Tree{manifestFile + "/README.md": files.Text("# acme-audit\n")})
			assert.Length(t, got, 1, "the manifest is the one fault")
			assert.Equal(t, got[0].File, manifestFile, "the fault is at the manifest, which is a directory")
		})

		t.Run("returns an Error at templates for a templates path that is a file", func(t *testing.T) {
			t.Parallel()

			got := loadFaults(t, files.Tree{
				manifestFile: files.Text("format: 1\nname: acme-audit\nversion: 0.3.0\n" + structRule),
				"templates":  files.Text("# notes\n"),
			})
			assert.Equal(t, got, []declarative.Error{{
				File: "templates",
				Msg:  "the path is not a directory, and each template tree is a directory below templates",
			}}, "the fault is at the templates path")
		})

		t.Run("returns an Error at templates for a templates link that leaves the directory", func(t *testing.T) {
			t.Parallel()

			got := loadFaults(t, files.Tree{
				manifestFile: files.Text("format: 1\nname: acme-audit\nversion: 0.3.0\n" + structRule),
				"templates":  files.Link("../escape"),
			})
			assert.Length(t, got, 1, "the templates link is the one fault")
			assert.Equal(t, got[0].File, "templates", "the fault is at the templates link")
			assert.Contains(t, got[0].Msg, "path escapes from parent", "the stat does not leave the directory")
		})

		t.Run("returns an Error at a file directly below templates", func(t *testing.T) {
			t.Parallel()

			tree := auditTree()
			tree["templates/notes.md"] = files.Text("# notes\n")
			assert.Equal(t, loadFaults(t, tree), []declarative.Error{{
				File: "templates/notes.md",
				Msg:  "the file is directly below templates, and each file belongs to the tree of a target",
			}}, "the fault is at the file")
		})

		t.Run("returns an Error at a link that leaves the directory", func(t *testing.T) {
			t.Parallel()

			tree := auditTree()
			tree["templates/golang/escape.tmpl"] = files.Link("../../../escape.tmpl")
			got := loadFaults(t, tree)
			assert.Length(t, got, 1, "the link is the one fault")
			assert.Equal(t, got[0].File, "templates/golang/escape.tmpl", "the fault is at the link")
			assert.Contains(t, got[0].Msg, "path escapes from parent", "the read does not leave the directory")
		})

		t.Run("returns the faults in the order of the files and their lines", func(t *testing.T) {
			t.Parallel()

			got := loadFaults(t, files.Tree{
				manifestFile: files.Text("format: 1\nname: acme-audit\nversion: 0.3.0\n" +
					"rules:\n    - on: struct\n      emit: mirror\n      template: function.tmpl\n" +
					"outputs:\n    - per: source\n      word: \"\"\n"),
				"templates/notes.md": files.Text("# notes\n"),
			})
			assert.Equal(t, got, []declarative.Error{
				{File: manifestFile, Line: 5, Msg: "the rule has a template, and a mirror of a struct has no body"},
				{File: manifestFile, Line: 9, Msg: "the family has no word"},
				{
					File: "templates/notes.md",
					Msg:  "the file is directly below templates, and each file belongs to the tree of a target",
				},
			}, "the faults are sorted by file, then by line")
		})

		t.Run("returns a generator that passes the plugin suite", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, auditTree())
			plugintest.RunPluginSuite(t, func(tb assert.TB) (plugin.Plugin, *plugintest.Fixture) {
				g, err := declarative.Load(dir)
				assert.NoError(tb, err, "the plugin directory loads")
				return g, ledgerFixture(tb)
			})
		})
	})
}

// auditTree returns the plugin directory of the cases: the manifest and the
// template trees of the scripted language and of golang.
func auditTree() files.Tree {
	return files.Tree{
		manifestFile: files.Text(auditManifest),
		fakeFunction: files.Text(functionBody),
		fakeMethod:   files.Text(methodBody),
		fakeTest:     files.Text(testBody),
		goFunction:   files.Text(functionBody),
		goMethod:     files.Text(methodBody),
		goTest:       files.Text(testBody),
		goHelper:     files.Text(helperBody),
	}
}

// versionOf loads the plugin directory tree, and returns the version of the
// generator.
func versionOf(tb testing.TB, tree files.Tree) string {
	tb.Helper()

	g, err := declarative.Load(files.Workspace(tb, tree))
	assert.NoError(tb, err, "the plugin directory loads")
	v, versioned := g.(plugin.Versioned)
	assert.True(tb, versioned, "the generator declares a version")
	return v.Version()
}

// fingerprintOf loads the plugin directory tree, and returns the fingerprint
// of a composition with one plan of the generator toward golang.
func fingerprintOf(tb testing.TB, tree files.Tree) []byte {
	tb.Helper()

	g, err := declarative.Load(files.Workspace(tb, tree))
	assert.NoError(tb, err, "the plugin directory loads")
	w, err := workspace.New().
		Brand(auditBrand).
		Targets(goTarget).
		Plans(workspace.Plan{
			Name:       auditPlan,
			Generators: []plugin.Generator{g},
			Backend:    printer{},
			Layout:     layout.Config{Dir: auditDir},
		}).
		Build()
	assert.NoError(tb, err, "the composition builds")
	return w.Fingerprint()
}

// loadFaults writes the plugin directory tree into a directory of the
// test's own, loads it, and returns the faults of Load in their order. The
// file of each fault is relative to the plugin directory, in slash form.
func loadFaults(tb testing.TB, tree files.Tree) []declarative.Error {
	tb.Helper()

	dir := files.Workspace(tb, tree)
	_, err := declarative.Load(dir)
	assert.HasError(tb, err, "the plugin directory has a fault")
	errs := []error{err}
	if joined, ok := errors.AsType[interface {
		error
		Unwrap() []error
	}](err); ok {
		errs = joined.Unwrap()
	}
	out := make([]declarative.Error, 0, len(errs))
	for _, e := range errs {
		fault := *assert.ErrorAs[*declarative.Error](tb, e, "each fault is a *declarative.Error")
		rel, rerr := filepath.Rel(dir, fault.File)
		assert.NoError(tb, rerr, "the file of the fault is in the plugin directory")
		fault.File = filepath.ToSlash(rel)
		out = append(out, fault)
	}
	return out
}

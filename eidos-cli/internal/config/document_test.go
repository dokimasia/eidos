// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli/internal/config"
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/declarative"
)

// The fixture composition has these names and values.
const (
	brand    output.Brand  = "acme"
	target   plugin.Target = "stub"
	planName               = "stubs"
	genID    plugin.ID     = "stubgen"
	suffix                 = "_stub"
	// memoLimit is a memo size that keeps every entry of a fixture run.
	memoLimit config.Bytes = 1048576
	// widthKey is a policy key that the fixture's backend does not
	// declare, and narrow is a choice of it.
	widthKey plugin.PolicyKey = "stub.width"
	narrow   plugin.Choice    = "narrow"
	// secondPlan is the second plan of a composition with two plans.
	secondPlan = "shapes"
)

// The declarative plugin of the plugin cases: its name, the directory that
// a root contains it in, and its manifest, which mirrors each struct into
// the file of its package and has no template tree.
const (
	shapeID       plugin.ID = "acme-shape"
	shapeDir                = "plugins/acme-shape"
	shapeManifest           = "format: 1\nname: acme-shape\nversion: 0.1.0\n" +
		"outputs:\n    - per: package\n      word: shape\nrules:\n    - on: struct\n      emit: mirror\n"
)

// stubBackend is the backend of the fixture plan. It does not implement a
// renderer.
type stubBackend struct{}

// Name returns the name of the backend.
func (stubBackend) Name() plugin.ID { return "stub-printer" }

// Target returns the target of the backend.
func (stubBackend) Target() plugin.Target { return target }

// stubOptions are the options of the generator of the fixture plan.
type stubOptions struct {
	Suffix string `opt:"suffix" doc:"the suffix of a generated file"`
}

// Apply sets on a builder each value that a document sets, and no other
// value.
func TestDocument(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves the composition unchanged for a document of only a version", func(t *testing.T) {
			t.Parallel()

			want := built(t, composition(t, &stubOptions{}))
			b := composition(t, &stubOptions{})
			assert.NoError(t, (&config.Document{Version: config.Current}).Apply(b, t.TempDir()), "the document applies")
			assert.Equal(t, built(t, b).Describe(), want.Describe(), "the document changes nothing")
		})

		t.Run("sets the name of the workspace", func(t *testing.T) {
			t.Parallel()

			b := composition(t, &stubOptions{})
			assert.NoError(t, (&config.Document{Workspace: "platform"}).Apply(b, t.TempDir()), "the document applies")
			assert.Equal(t, built(t, b).Describe().Name, "platform", "the workspace has the name of the document")
		})

		tests := []struct {
			name    string
			give    config.Document
			markers []string
		}{
			{
				name:    "passes workers to Parallel",
				give:    config.Document{Workers: new(config.Count(-1))},
				markers: []string{"Parallel(-1)"},
			},
			{
				name:    "passes ignore to Ignore",
				give:    config.Document{Ignore: []directive.Name{directive.KernelSkip}},
				markers: []string{string(directive.KernelSkip), "kernel"},
			},
			{
				name:    "passes the memo limit to Memo",
				give:    config.Document{Memo: &config.Memo{Limit: -1}},
				markers: []string{"memo limit -1"},
			},
			{
				name:    "passes the options to Config",
				give:    config.Document{Options: map[string]map[string]any{string(genID): {"suffix": 1}}},
				markers: []string{`"suffix"`, "string", "int"},
			},
			{
				name:    "passes the plans to Config",
				give:    config.Document{Plans: map[string]config.Plan{"ghost": {}}},
				markers: []string{`"ghost"`},
			},
			{
				name:    "passes the policies to Config",
				give:    config.Document{Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: narrow}},
				markers: []string{string(widthKey), "no plan's backend declares a policy"},
			},
			{
				name: "passes the policies of a plan to Config",
				give: config.Document{Plans: map[string]config.Plan{
					planName: {Policies: map[plugin.PolicyKey]plugin.Choice{widthKey: narrow}},
				}},
				markers: []string{`"` + planName + `"`, string(widthKey)},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b := composition(t, &stubOptions{})
				assert.NoError(t, tt.give.Apply(b, t.TempDir()), "the document applies")
				_, err := b.Build()
				assert.HasError(t, err, "Build returns the fault of the value")
				for _, marker := range tt.markers {
					assert.Contains(t, err.Error(), marker, "the error describes the value")
				}
			})
		}

		t.Run("sets the option values of a plugin", func(t *testing.T) {
			t.Parallel()

			opts := &stubOptions{}
			b := composition(t, opts)
			doc := &config.Document{Options: map[string]map[string]any{string(genID): {"suffix": suffix}}}
			assert.NoError(t, doc.Apply(b, t.TempDir()), "the document applies")
			built(t, b)
			assert.Equal(t, opts.Suffix, suffix, "Build writes the value into the options of the generator")
		})

		t.Run("refines the sources and the layout of a plan", func(t *testing.T) {
			t.Parallel()

			b := composition(t, &stubOptions{})
			assert.NoError(t, (&config.Document{Plans: map[string]config.Plan{planName: {
				Sources: &config.Sources{Lang: frontendtest.ScriptedLang, Packages: []string{"./svc/..."}},
				Layout:  &config.Layout{Policy: config.Policy(layout.PolicyCentralised), Dir: "gen"},
			}}}).Apply(b, t.TempDir()), "the document applies")
			plans := built(t, b).Describe().Plans
			assert.Length(t, plans, 1, "the composition has the plan")
			assert.Equal(t, plans[0].Sources, workspace.Sources{
				Lang: frontendtest.ScriptedLang, Packages: []string{"./svc/..."},
			}, "the plan has the sources of the document")
			assert.Equal(t, plans[0].Layout, layout.Config{Policy: layout.PolicyCentralised, Dir: "gen"},
				"the plan has the layout of the document")
		})

		t.Run("leaves out a plan that the document disables", func(t *testing.T) {
			t.Parallel()

			b := composition(t, &stubOptions{})
			doc := &config.Document{Plans: map[string]config.Plan{planName: {Enabled: new(false)}}}
			assert.NoError(t, doc.Apply(b, t.TempDir()), "the document applies")
			assert.Empty(t, built(t, b).Describe().Plans, "the composition has no plan")
		})

		t.Run("passes the generator of a plugin directory to its plan", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{"plugin.yaml": files.Text(shapeManifest)})
			b := extensible(t, planName)
			assert.NoError(t, plugins(dir, planName).Apply(b, t.TempDir()), "the plugin directory loads")
			assert.Contains(t, generators(built(t, b), planName), shapeID, "the plan runs the plugin")
		})

		t.Run("loads a plugin directory relative to the root", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{shapeDir + "/plugin.yaml": files.Text(shapeManifest)})
			b := extensible(t, planName)
			assert.NoError(t, plugins(shapeDir, planName).Apply(b, root), "the plugin directory loads")
			assert.Contains(t, generators(built(t, b), planName), shapeID, "the plan runs the plugin")
		})

		t.Run("loads a plugin directory once for two plans", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{shapeDir + "/plugin.yaml": files.Text(shapeManifest)})
			b := extensible(t, planName, secondPlan)
			assert.NoError(t, plugins(shapeDir, planName, secondPlan).Apply(b, root), "the plugin directory loads")
			assert.Contains(t, generators(built(t, b), secondPlan), shapeID,
				"Build admits one plugin under the name in both plans")
		})

		t.Run("returns the error of a plugin directory that does not load", func(t *testing.T) {
			t.Parallel()

			err := plugins(shapeDir, planName).Apply(extensible(t, planName), t.TempDir())
			fault := assert.ErrorAs[*declarative.Error](t, err, "the error is the fault of the plugin directory")
			assert.Equal(t, fault.Msg, "the directory has no plugin.yaml", "the fault describes the directory")
		})

		t.Run("returns the error of a plugin directory once for two plans", func(t *testing.T) {
			t.Parallel()

			err := plugins(shapeDir, planName, secondPlan).Apply(extensible(t, planName, secondPlan), t.TempDir())
			joined, is := errors.AsType[interface {
				error
				Unwrap() []error
			}](err)
			assert.True(t, is, "the error joins the errors of the loads")
			assert.Length(t, joined.Unwrap(), 1, "Apply loads the directory once")
		})

		t.Run("passes the plugins of a plan that is not extensible to Build", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{"plugin.yaml": files.Text(shapeManifest)})
			b := composition(t, &stubOptions{})
			assert.NoError(t, plugins(dir, planName).Apply(b, t.TempDir()), "the plugin directory loads")
			_, err := b.Build()
			assert.HasError(t, err, "Build refuses the plugins")
			assert.Contains(t, err.Error(), `plan "stubs", which is not extensible`, "the error names the plan")
		})

		t.Run("opens the memo in a directory relative to the root", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			memoRun(t, root, "cache/memo")
			files.IsDir(t, filepath.Join(root, "cache", "memo"), "the first write of the memo creates the directory")
		})

		t.Run("opens the memo in an absolute directory", func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "memo")
			memoRun(t, t.TempDir(), filepath.ToSlash(dir))
			files.IsDir(t, dir, "the first write of the memo creates the directory")
		})
	})
}

// composition returns the builder of a composition over the scripted
// frontend with one plan. The generator of the plan has opts as its
// options.
func composition(tb testing.TB, opts *stubOptions) *workspace.Builder {
	tb.Helper()

	return workspace.New().
		Brand(brand).
		Frontends(frontendtest.NewScripted()).
		Targets(target).
		Plans(workspace.Plan{Name: planName, Generators: []plugin.Generator{stub(tb, opts)}, Backend: stubBackend{}})
}

// extensible returns the builder of a composition over the scripted
// frontend with an extensible plan for each name of names. The plans share
// one generator.
func extensible(tb testing.TB, names ...string) *workspace.Builder {
	tb.Helper()

	gen := stub(tb, &stubOptions{})
	plans := make([]workspace.Plan, 0, len(names))
	for _, name := range names {
		plans = append(plans, workspace.Plan{
			Name: name, Generators: []plugin.Generator{gen}, Backend: stubBackend{}, Extensible: true,
		})
	}
	return workspace.New().Brand(brand).Frontends(frontendtest.NewScripted()).Targets(target).Plans(plans...)
}

// stub returns the generator of the fixture plans, with opts as its
// options.
func stub(tb testing.TB, opts *stubOptions) plugin.Generator {
	tb.Helper()

	gen, isGenerator := eidos.NewPlugin(genID).
		Options(opts).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "stub"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
		Build().(plugin.Generator)
	assert.True(tb, isGenerator, "an emitter rule lowers to the generator role")
	return gen
}

// plugins returns a document that lists the plugin directory dir in each
// plan of names.
func plugins(dir string, names ...string) *config.Document {
	plans := make(map[string]config.Plan, len(names))
	for _, name := range names {
		plans[name] = config.Plan{Plugins: []string{filepath.ToSlash(dir)}}
	}
	return &config.Document{Plans: plans}
}

// generators returns the name of each generator of the plan name of w.
func generators(w *workspace.Workspace, name string) []plugin.ID {
	var out []plugin.ID
	for _, p := range w.Describe().Plans {
		if p.Name != name {
			continue
		}
		for _, c := range p.Generate {
			out = append(out, c.Name)
		}
	}
	return out
}

// built builds the composition of b, and fails the test on a fault.
func built(tb testing.TB, b *workspace.Builder) *workspace.Workspace {
	tb.Helper()

	w, err := b.Build()
	assert.NoError(tb, err, "the composition builds")
	return w
}

// memoRun runs a composition without plans over a tree of one package. A
// document sets the memo directory dir, and root is the workspace root.
func memoRun(tb testing.TB, root, dir string) {
	tb.Helper()

	b := workspace.New().
		Brand(brand).
		Frontends(frontendtest.NewScripted()).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return ledger.NewMem(), nil })
	assert.NoError(tb, (&config.Document{Memo: &config.Memo{Limit: memoLimit, Dir: dir}}).Apply(b, root),
		"the document applies")
	tree := fstest.MapFS{"svc/api.zz": {Data: []byte("package svc/api\ntype User string\n")}}
	_, err := built(tb, b).Run(tb.Context(), workspace.Input{Tree: tree})
	assert.NoError(tb, err, "the run succeeds")
}

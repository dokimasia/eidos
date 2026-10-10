// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
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
			(&config.Document{Version: config.Current}).Apply(b, t.TempDir())
			assert.Equal(t, built(t, b).Describe(), want.Describe(), "the document changes nothing")
		})

		t.Run("sets the name of the workspace", func(t *testing.T) {
			t.Parallel()

			b := composition(t, &stubOptions{})
			(&config.Document{Workspace: "platform"}).Apply(b, t.TempDir())
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
				tt.give.Apply(b, t.TempDir())
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
			doc.Apply(b, t.TempDir())
			built(t, b)
			assert.Equal(t, opts.Suffix, suffix, "Build writes the value into the options of the generator")
		})

		t.Run("refines the sources and the layout of a plan", func(t *testing.T) {
			t.Parallel()

			b := composition(t, &stubOptions{})
			(&config.Document{Plans: map[string]config.Plan{planName: {
				Sources: &config.Sources{Lang: frontendtest.ScriptedLang, Packages: []string{"./svc/..."}},
				Layout:  &config.Layout{Policy: config.Policy(layout.PolicyCentralised), Dir: "gen"},
			}}}).Apply(b, t.TempDir())
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
			(&config.Document{Plans: map[string]config.Plan{planName: {Enabled: new(false)}}}).Apply(b, t.TempDir())
			assert.Empty(t, built(t, b).Describe().Plans, "the composition has no plan")
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

	gen, isGenerator := eidos.NewPlugin(genID).
		Options(opts).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "stub"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
		Build().(plugin.Generator)
	assert.True(tb, isGenerator, "an emitter rule lowers to the generator role")
	return workspace.New().
		Brand(brand).
		Frontends(frontendtest.NewScripted()).
		Targets(target).
		Plans(workspace.Plan{Name: planName, Generators: []plugin.Generator{gen}, Backend: stubBackend{}})
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
	(&config.Document{Memo: &config.Memo{Limit: memoLimit, Dir: dir}}).Apply(b, root)
	tree := fstest.MapFS{"svc/api.zz": {Data: []byte("package svc/api\ntype User string\n")}}
	_, err := built(tb, b).Run(tb.Context(), workspace.Input{Tree: tree})
	assert.NoError(tb, err, "the run succeeds")
}

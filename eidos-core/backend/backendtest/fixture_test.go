// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugin/plugintest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The policy of the fixture: a key of the stub target and the choice that
// the fixture selects for it.
const (
	widthKey    plugin.PolicyKey = "stub.width"
	narrowWidth plugin.Choice    = "narrow"
	wideWidth   plugin.Choice    = "wide"
)

// closedPkg is the package that the import rule of the routing case closes
// to every file.
const closedPkg = "example.com/closed"

// family is a plugin that declares the per-plan family index and runs no
// rule, so the layout of a fixture routes a unit of the family.
type family struct{}

// Name returns the name of the plugin of the unit of the routing case.
func (family) Name() plugin.ID { return "gen" }

// Outputs returns the per-plan family index.
func (family) Outputs() []plugin.Output { return []plugin.Output{{Per: plugin.PerPlan, Word: "index"}} }

// The fixture is the plan in the form that the renderer receives. Every
// check renders through the context of the fixture, so no check
// exercises a part that the context leaves out.
func TestFixture(t *testing.T) {
	t.Parallel()

	t.Run("Context", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a context with every part of the fixture", func(t *testing.T) {
			t.Parallel()

			policy, err := plugin.NewPolicy(stubTarget, []plugin.PolicySpec{{
				Key: widthKey, Choices: []plugin.Choice{narrowWidth, wideWidth}, Default: narrowWidth,
				Doc: "the width of a line",
			}}, map[plugin.PolicyKey]plugin.Choice{widthKey: wideWidth})
			assert.NoError(t, err, "the policy of the fixture resolves")
			f := &backendtest.Fixture{
				Emit:      plugin.NewEmit(),
				Schedule:  []plugin.ID{"gen"},
				Trees:     map[plugin.ID]fs.FS{"gen": fstest.MapFS{}},
				Funcs:     map[plugin.ID]template.FuncMap{"gen": {}},
				Overrides: map[plugin.ID][]string{"gen": {"up"}},
				Policy:    policy,
			}
			sink := diag.NewSink()
			got, err := f.Context(wellBackend(t), sink)
			assert.NoError(t, err, "the files of the fixture route")
			expect.Equal(t, got.Policy.Choice(widthKey), wideWidth, "the context has the policy of the fixture")
			expect.Equal(t, got.Emit, f.Emit, "the context has the store of the fixture", assert.ByIdentity())
			expect.Equal(t, got.Schedule, f.Schedule, "the context has the schedule of the fixture")
			expect.Equal(t, got.Trees["gen"], f.Trees["gen"], "the context has the trees of the fixture",
				assert.ByIdentity())
			expect.Equal(t, got.Funcs["gen"], f.Funcs["gen"], "the context has the helpers of the fixture",
				assert.ByIdentity())
			expect.Equal(t, got.Overrides["gen"], []string{"up"},
				"the context has the override declarations of the fixture")
			expect.Equal(t, got.Sink, sink, "the context reports into the sink", assert.ByIdentity())
			expect.NotEmpty(t, got.Plugin, "the context renders under the identity of the suite")
		})

		t.Run("routes the store through the layout of the fixture", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(plugin.Unit{
				Plugin: "gen", Per: plugin.PerPlan, Word: "index", Decls: []symbol.Symbol{&emit.Struct{Name: "Index"}},
			}), "the unit of gen is added")
			b, sink := wellBackend(t), diag.NewSink()
			assert.NoError(t, plugin.Settle(e, b, nil, sink), "the store settles")
			f := &backendtest.Fixture{Emit: e, Layout: plugintest.New(t).Layout(t)}
			got, err := f.Context(b, sink)
			assert.NoError(t, err, "the layout routes the store")
			expect.Empty(t, got.Files, "the layout refuses the unit of a family that no plugin of the plan declares")
			coretest.AssertCodes(t, sink, layout.UndeclaredFamily)
		})

		t.Run("routes the store through the import rule of the backend", func(t *testing.T) {
			t.Parallel()

			index := &emit.Struct{Name: "Index"}
			index.Fields.Append(&emit.Field{Name: "row", Type: &emit.TypeRef{Spelling: "Row", Package: closedPkg}})
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(plugin.Unit{
				Plugin: "gen", Per: plugin.PerPlan, Word: "index", Decls: []symbol.Symbol{index},
			}), "the unit of gen is added")
			refusal := errors.New("backendtest_test: no file imports the closed package")
			b := wellBuilder(t).Coverage(total(nil)).ImportRule(func(_ symbol.Identity, ref *emit.TypeRef) error {
				if ref.Package == closedPkg {
					return refusal
				}
				return nil
			}).Build()
			sink := diag.NewSink()
			assert.NoError(t, plugin.Settle(e, b, nil, sink), "the store settles")
			f := &backendtest.Fixture{Emit: e, Layout: plugintest.New(t).Layout(t, family{})}
			got, err := f.Context(b, sink)
			assert.NoError(t, err, "the layout routes the store")
			expect.Empty(t, got.Files, "the layout refuses the declaration whose file cannot import its reference")
			coretest.AssertCodes(t, sink, layout.UnimportableReference)
		})

		t.Run("returns the error of the layout for an input that it cannot route", func(t *testing.T) {
			t.Parallel()

			f := &backendtest.Fixture{Emit: plugin.NewEmit(), Layout: &layout.Input{}}
			_, err := f.Context(wellBackend(t), diag.NewSink())
			assert.HasError(t, err, "the layout refuses a store that did not settle")
		})
	})
}

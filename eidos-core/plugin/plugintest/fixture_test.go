// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugintest_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugin/plugintest"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The target of the translation cases, the key of its one policy, and
// the policy's choices.
const (
	tsxTarget plugin.Target    = "tsx"
	widthKey  plugin.PolicyKey = "tsx.width"
	narrow    plugin.Choice    = "narrow"
	wide      plugin.Choice    = "wide"
)

// panicReason is what the handlers of the panic cases panic with.
const panicReason = "t: the handler panics"

// keyRecorder is a journal that keeps the match key of every
// invocation a phase call hands it.
type keyRecorder struct{ keys []plugin.MatchKey }

// Invoked keeps the invocation's key.
func (r *keyRecorder) Invoked(inv plugin.Invocation) { r.keys = append(r.keys, inv.Match) }

// Evaluated keeps nothing: the cases name no candidate.
func (*keyRecorder) Evaluated(symbol.Identity, []plugin.MatchKey) {}

// choiceSpoke is a spoke that spells every shape as the choice of
// widthKey under the policy that it receives.
type choiceSpoke struct{}

var _ plugin.TypeSpeller = choiceSpoke{}

// SpellType spells the shape as the policy's choice of widthKey.
func (choiceSpoke) SpellType(_ rules.TypeShape, p plugin.Policy) (*emit.TypeRef, error) {
	return &emit.TypeRef{Spelling: string(p.Choice(widthKey))}, nil
}

// schemed is a plugin that declares the schemas of its list.
type schemed []directive.Schema

var _ plugin.DirectiveProvider = schemed(nil)

// Name returns the plugin of the first schema.
func (s schemed) Name() plugin.ID { return plugin.ID(s[0].Plugin) }

// Directives returns the schemas of the list.
func (s schemed) Directives() []directive.Schema { return s }

// probeSchema is the schema of the directive probe of the plugin t.
var probeSchema = directive.Schema{Plugin: "t", Name: "probe", Doc: "marks a subject that the plugin probes"}

// The fixture is the read side of a run, built by hand, so everything
// that a phase call receives through it is contract: the loaded
// packages, the validated gates, the stamped facts, the seeded units,
// the selection and the journal, and the roles that it does not hand to
// a phase call.
func TestFixture(t *testing.T) {
	t.Parallel()

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		// stamper returns an annotator that stamps a flag on every struct
		// of the fixture, and the flag.
		stamper := func(tb assert.TB, f *plugintest.Fixture) (plugin.Plugin, meta.Key[bool]) {
			tb.Helper()

			key := plugintest.Key[bool](tb, f, "t.flag", "marks a subject")
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, key, true)
					return nil
				})).
				Annotator()
			return p, key
		}

		t.Run("restricts the call to the fixture's selection", func(t *testing.T) {
			t.Parallel()

			f, _, beta := twoStructs(t)
			p, key := stamper(t, f)
			f.Select = &plugin.Selection{Matches: []plugin.MatchKey{{Plugin: "t", Subject: beta.ID}}}
			assert.NoError(t, f.Annotate(t, p).Err, "the phase call passes")
			assert.Equal(t, slices.Collect(f.Facts.ByKey(key.ID())), []symbol.Identity{beta.ID},
				"the listed subject alone is stamped")
		})

		t.Run("hands the call the fixture's journal", func(t *testing.T) {
			t.Parallel()

			f, alpha, beta := twoStructs(t)
			p, _ := stamper(t, f)
			rec := &keyRecorder{}
			f.Journal = rec
			assert.NoError(t, f.Annotate(t, p).Err, "the phase call passes")
			assert.Equal(t, rec.keys, []plugin.MatchKey{
				{Plugin: "t", Subject: alpha.ID}, {Plugin: "t", Subject: beta.ID},
			}, "the journal receives both invocations")
		})

		t.Run("binds a gate on a named handle in the fixture's registry", func(t *testing.T) {
			t.Parallel()

			f, alpha, _ := twoStructs(t)
			key := plugintest.Key[bool](t, f, "t.flag", "marks a subject")
			plugintest.Stamp(t, f, key, alpha.ID, true)

			var visited []string
			p := eidos.NewPlugin("t").
				Handle(eidos.Where(eidos.HasKey(meta.Named[bool](key.Name())),
					eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Stamper) error {
						visited = append(visited, m.Struct.Name)
						return nil
					}))).
				Annotator()

			assert.NoError(t, f.Annotate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Alpha"}, "the gate admits the stamped subject through the name")
		})

		t.Run("fails the test for a phase that panics", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a handler that panics must fail the test",
				func(tb assert.TB) {
					f, _, _ := twoStructs(tb)
					p := eidos.NewPlugin("t").
						Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Stamper) error {
							panic(panicReason)
						})).
						Annotator()
					f.Annotate(tb, p)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{"the annotate phase runs without a panic"},
				"the failure names the panic")
		})
	})

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("restricts the call to the fixture's selection", func(t *testing.T) {
			t.Parallel()

			f, _, beta := twoStructs(t)
			var visited []string
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					visited = append(visited, m.Struct.Name)
					return nil
				})).
				Generator()
			f.Select = &plugin.Selection{Matches: []plugin.MatchKey{{Plugin: "t", Subject: beta.ID}}}
			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Beta"}, "the listed subject alone runs")
		})

		t.Run("hands the call the fixture's journal", func(t *testing.T) {
			t.Parallel()

			f, alpha, beta := twoStructs(t)
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
				Generator()
			rec := &keyRecorder{}
			f.Journal = rec
			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, rec.keys, []plugin.MatchKey{
				{Plugin: "t", Subject: alpha.ID}, {Plugin: "t", Subject: beta.ID},
			}, "the journal receives both invocations")
		})

		t.Run("dispatches over what was loaded", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			var visited []string
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					visited = append(visited, m.Struct.Name)
					return nil
				})).
				Generator()

			r := f.Generate(t, p)
			assert.NoError(t, r.Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Alpha", "Beta"},
				"the loaded declarations dispatch in identity order")
		})

		t.Run("sees seeded units the way a later turn does", func(t *testing.T) {
			t.Parallel()

			f, alpha, _ := twoStructs(t)
			f.Seed(t, plugin.Unit{
				Plugin: "earlier", Per: plugin.PerSource, Word: "impl",
				Key:   "a.go",
				Decls: []symbol.Symbol{&emit.Struct{Origin: alpha.ID, Name: "Gen"}},
			})

			var visited int
			p := eidos.NewPlugin("t").
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						visited++
						assert.Equal(t, m.Origin(), alpha.ID,
							"the seeded value names its origin")
						return nil
					})).
				Generator()

			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, 1, "the seeded unit belongs to the earlier turn")
		})

		t.Run("gates on the validated table", func(t *testing.T) {
			t.Parallel()

			f, alpha, _ := twoStructs(t)
			schema := directive.Schema{
				Plugin: "stubgen", Name: "stub", Doc: "a fixture directive",
			}
			f.Validated(t, alpha.ID, directive.Directive{Name: schema.Canonical()})

			var visited []string
			p := eidos.NewPlugin("stubgen").
				Handle(eidos.Directive(schema,
					eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						visited = append(visited, m.Struct.Name)
						return nil
					}))).
				Generator()

			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Alpha"},
				"only the validated carrier matches")
		})

		t.Run("binds the walks over the fixture's rules and the kernel's keys", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			f.Rules = rules.NewRegistry()
			assert.NoError(t, f.Rules.Register(rules.Absent(coretest.Lang)),
				"the fixture language registers its rules")
			var kernel meta.KernelKeys
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					m.Rules()
					kernel = m.Kernel()
					return nil
				})).
				Generator()

			r := f.Generate(t, p)
			assert.NoError(t, r.Err, "the phase call passes")
			assert.Equal(t, kernel, f.Kernel, "the handler reads the kernel's keys the fixture registered")
			assert.Equal(t, kernel.Module.Name(), meta.ModuleKey, "under the kernel's own names")
			assert.Empty(t, slices.Collect(r.Sink.All()),
				"a registered language binds without the absent-rules warning")
		})

		// translated runs a generator over f whose handler translates a
		// reference to beta on alpha, and returns what the translation
		// returned.
		translated := func(tb assert.TB, f *plugintest.Fixture, alpha, beta symbol.Identity) *emit.TypeRef {
			tb.Helper()

			var got *emit.TypeRef
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					if m.Struct.ID == alpha {
						got, _ = e.Type(alpha, &node.TypeRef{Spelling: beta.Name, Target: beta})
					}
					return nil
				})).
				Generator()
			assert.NoError(tb, f.Generate(tb, p).Err, "the phase call passes")
			return got
		}

		t.Run("hands the call the fixture's target", func(t *testing.T) {
			t.Parallel()

			f, alpha, beta := twoStructs(t)
			f.Target = plugin.Target(coretest.Lang)
			got := translated(t, f, alpha.ID, beta.ID)
			assert.Equal(t, got, &emit.TypeRef{Spelling: beta.Name, Target: beta.ID},
				"a plan of the fixture's language translates its references as written")
		})

		t.Run("hands the call the fixture's spoke under the fixture's policy", func(t *testing.T) {
			t.Parallel()

			f, alpha, beta := twoStructs(t)
			policy, err := plugin.NewPolicy(tsxTarget, []plugin.PolicySpec{{
				Key: widthKey, Choices: []plugin.Choice{narrow, wide}, Default: wide, Doc: "the width of an integer",
			}}, nil)
			assert.NoError(t, err, "the policy resolves")
			f.Target, f.Types, f.Policy = tsxTarget, choiceSpoke{}, policy
			got := translated(t, f, alpha.ID, beta.ID)
			assert.NotNil(t, got, "the spoke spells the reference")
			assert.Equal(t, got.Spelling, string(wide), "the spoke reads the policy's choice")
		})

		t.Run("refuses a plugin without the role", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "an annotator is not a generator",
				func(tb assert.TB) {
					f, _, _ := twoStructs(tb)
					key := plugintest.Key[bool](tb, f, "t.flag", "marks a subject")
					p := eidos.NewPlugin("t").
						Handle(eidos.OnStruct(
							func(m *eidos.StructMatch, st *eidos.Stamper) error {
								eidos.Stamp(st, key, true)
								return nil
							},
						)).
						Annotator()
					f.Generate(tb, p)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{"the plugin implements the generator role"},
				"the refusal names the missing role")
		})

		t.Run("fails the test for a phase that panics", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a handler that panics must fail the test",
				func(tb assert.TB) {
					f, _, _ := twoStructs(tb)
					p := eidos.NewPlugin("t").
						Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
							panic(panicReason)
						})).
						Generator()
					f.Generate(tb, p)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{"the generate phase runs without a panic"},
				"the failure names the panic")
		})
	})

	t.Run("Layout", func(t *testing.T) {
		t.Parallel()

		probe := plugin.Output{Per: plugin.PerSource, Word: "probe"}
		probes := eidos.NewPlugin("t").Output(probe).
			Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
			Generator()

		t.Run("returns the families that each plugin declares", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			assert.Equal(t, f.Layout(t, probes, handRolled{}).Outputs, map[plugin.ID][]plugin.Output{
				"t": {probe}, "twin": nil,
			}, "a plugin without families routes no unit")
		})

		t.Run("returns an index over the fixture", func(t *testing.T) {
			t.Parallel()

			f, alpha, _ := twoStructs(t)
			_, found := f.Layout(t, probes).Index.Lookup(alpha.ID)
			assert.True(t, found, "the index resolves a struct of the fixture")
		})

		t.Run("returns the residents of the graph by directory", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			assert.Equal(t, f.Layout(t, probes).Residents("."), []plugin.Resident{
				{File: coretest.UnitFile, Pkg: coretest.PackageID(coretest.StorePath)},
			}, "the file of the store is the resident of its directory")
		})

		t.Run("returns the modules that the facts of the graph state", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			pkg := coretest.PackageID(coretest.StorePath)
			plugintest.Stamp(t, f, f.Kernel.Module, pkg, "example.com/acme")
			plugintest.Stamp(t, f, f.Kernel.ModuleRoot, pkg, ".")
			assert.Equal(t, f.Layout(t, probes).Modules, []plugin.Module{
				{Lang: coretest.Lang, Path: "example.com/acme", Root: "."},
			}, "the module of the store is a module of the plan")
		})

		t.Run("returns a registry with the schema of each plugin", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			_, registered := f.Layout(t, schemed{probeSchema}).Directives.ResolveName(probeSchema.Canonical())
			assert.True(t, registered, "the registry scopes the routing keys of the directive of the plugin")
		})

		t.Run("fails the test for a schema that does not register", func(t *testing.T) {
			t.Parallel()

			failure := assert.Rejects(t, "a schema under a kernel name must fail the test", func(tb assert.TB) {
				f, _, _ := twoStructs(tb)
				claimed := probeSchema
				claimed.Name = directive.KernelOut
				f.Layout(tb, schemed{claimed})
			})
			assert.Equal(t, coretest.Contracts(failure), []string{"the schema t:out registers"},
				"the failure names the schema")
		})
	})

	t.Run("Stamp", func(t *testing.T) {
		t.Parallel()

		t.Run("feeds the fact gates", func(t *testing.T) {
			t.Parallel()

			f, alpha, _ := twoStructs(t)
			key := plugintest.Key[bool](t, f, "t.flag", "marks a subject")
			plugintest.Stamp(t, f, key, alpha.ID, true)

			var visited []string
			p := eidos.NewPlugin("t").
				Handle(eidos.Where(eidos.HasKey(key),
					eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
						visited = append(visited, m.Struct.Name)
						return nil
					}))).
				Generator()

			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Alpha"},
				"the gate admits the stamped subject alone")
		})
	})

	t.Run("LoadDependencies", func(t *testing.T) {
		t.Parallel()

		gamma := coretest.Struct(coretest.CachePath, "Gamma")

		t.Run("adds packages that the graph reports as dependency packages", func(t *testing.T) {
			t.Parallel()

			f := plugintest.New(t).LoadDependencies(t, coretest.Package(coretest.CachePath, gamma))
			f.Graph.Freeze()
			assert.True(t, f.Graph.Dependency(gamma.ID), "the graph marks the declaration of the dependency package")
		})

		t.Run("dispatches no rule of the generate phase on a declaration of a dependency package", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			f.LoadDependencies(t, coretest.Package(coretest.CachePath, gamma))
			var visited []string
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Emitter) error {
					visited = append(visited, m.Struct.Name)
					return nil
				})).
				Generator()
			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Alpha", "Beta"}, "the generator visits the sources of the fixture alone")
		})

		t.Run("dispatches an annotator on a declaration of a dependency package", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			f.LoadDependencies(t, coretest.Package(coretest.CachePath, gamma))
			var visited []string
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, _ *eidos.Stamper) error {
					visited = append(visited, m.Struct.Name)
					return nil
				})).
				Annotator()
			assert.NoError(t, f.Annotate(t, p).Err, "the phase call passes")
			assert.Permutation(t, visited, []string{"Alpha", "Beta", "Gamma"},
				"the annotator visits the dependency package too")
		})
	})

	t.Run("Key", func(t *testing.T) {
		t.Parallel()

		t.Run("claims one namespace once", func(t *testing.T) {
			t.Parallel()

			f, _, _ := twoStructs(t)
			first := plugintest.Key[bool](t, f, "t.first", "the first key")
			second := plugintest.Key[string](t, f, "t.second", "the second key")
			assert.False(t, first.IsZero(), "the first key registers")
			assert.False(t, second.IsZero(),
				"a second key under the claimed namespace registers too")
		})
	})
}

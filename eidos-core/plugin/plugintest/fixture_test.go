// Copyright ThesmOS B.V. 2026
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

// The fixture is the run's read side built by hand, so what a
// phase call sees through it is contract: loaded packages,
// validated gates, stamped facts, seeded units, the selection and
// the journal, and the roles it refuses to hand a phase call to.
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
				Build()
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
				Build()

			assert.NoError(t, f.Annotate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Alpha"}, "the gate admits the stamped subject through the name")
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
				Build()
			f.Select = &plugin.Selection{Matches: []plugin.MatchKey{{Plugin: "t", Subject: beta.ID}}}
			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Beta"}, "the listed subject alone runs")
		})

		t.Run("hands the call the fixture's journal", func(t *testing.T) {
			t.Parallel()

			f, alpha, beta := twoStructs(t)
			p := eidos.NewPlugin("t").
				Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
				Build()
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
				Build()

			r := f.Generate(t, p)
			assert.NoError(t, r.Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Alpha", "Beta"},
				"the loaded declarations dispatch in identity order")
		})

		t.Run("sees seeded units the way a later bucket does", func(t *testing.T) {
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
				Build()

			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, 1, "the seeded unit is the earlier bucket")
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
				Build()

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
				Build()

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
				Build()
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
						Build()
					f.Generate(tb, p)
				})
			assert.Equal(t, coretest.Contracts(failure), []string{"the plugin implements the generator role"},
				"the refusal names the missing role")
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
				Build()

			assert.NoError(t, f.Generate(t, p).Err, "the phase call passes")
			assert.Equal(t, visited, []string{"Alpha"},
				"the stamped subject alone carries the gate")
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

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
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/plugin/plugintest"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture is the run's read side built by hand, so what a
// phase call sees through it is contract: loaded packages,
// validated gates, stamped facts, seeded units, and the roles it
// refuses to hand a phase call to.
func TestFixture(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

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
				"the loaded declarations dispatch in the graph's own order")
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
			assert.Length(t, slices.Collect(r.Sink.All()), 0,
				"a registered language binds without the absent-rules warning")
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
			assert.Contains(t, failure, "generator role",
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

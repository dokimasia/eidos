// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// firstFunction pins the name of the corpus's first declaration: the
// corpus cycles its kinds in kind order, the function kind sorts
// first, and the counter starts at zero.
const firstFunction = "task0"

// roomy is the ceiling a case not about the budget states: high
// enough that the measurement never decides the outcome.
var roomy = backendtest.Budget{MaxAllocs: 1 << 40}

// counted wraps a setup with a call counter, so a case can tell a
// guard that stopped before the fixture was built from one that
// stopped after.
func counted(s backendtest.Setup, calls *int) backendtest.Setup {
	return func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
		*calls++
		return s(tb)
	}
}

// benchRender runs one BenchRender through [testing.Benchmark], so a
// case reads the guard's effect off the iteration count, because
// b.Fatal drops its message.
func benchRender(setup backendtest.Setup, budget backendtest.Budget) testing.BenchmarkResult {
	return testing.Benchmark(func(b *testing.B) {
		b.Helper()
		backendtest.BenchRender(b, setup, budget)
	})
}

// benchSettle runs one BenchSettle the way [benchRender] runs its
// half of the pipeline.
func benchSettle(setup backendtest.Setup, budget backendtest.Budget) testing.BenchmarkResult {
	return testing.Benchmark(func(b *testing.B) {
		b.Helper()
		backendtest.BenchSettle(b, setup, budget)
	})
}

// A benchmark's guards report through b.Fatal, which discards the
// message and returns a zero result, so every guard case here reads
// the effect: whether the loop ran at all, and how far the setup got
// before it stopped.
func TestBench(t *testing.T) {
	t.Parallel()

	t.Run("ScaledFixture", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the canonical scale", func(t *testing.T) {
			t.Parallel()

			f := backendtest.ScaledFixture(t, nil)
			units, decls := 0, 0
			for u := range f.Emit.Units() {
				units++
				decls += len(u.Decls)
				assert.True(t, len(u.Origins) > 0, "every unit has provenance")
			}
			assert.Equal(t, units, backendtest.BenchPackages*backendtest.BenchFiles,
				"one unit per package file")
			assert.Equal(t, decls,
				backendtest.BenchPackages*backendtest.BenchFiles*backendtest.BenchDecls,
				"the declared scale, counted at file level")
		})

		t.Run("leaves out the kinds the backend refuses", func(t *testing.T) {
			t.Parallel()

			refused := map[symbol.Kind]string{
				symbol.KindEnum: kindReason,
				symbol.KindSum:  kindReason,
			}
			f := backendtest.ScaledFixture(t, refused)
			for u := range f.Emit.Units() {
				for _, d := range u.Decls {
					_, out := refused[d.Kind()]
					assert.False(t, out, "no unit emits a refused kind: "+d.Kind().String())
				}
			}
		})

		t.Run("cycles the kinds it emits in kind order", func(t *testing.T) {
			t.Parallel()

			first := unitsOf(t, backendtest.ScaledFixture(t, nil))[0]
			named := false
			for _, d := range first.Decls {
				if fn, isFunction := d.(*emit.Function); isFunction && fn.Name == firstFunction {
					named = true
				}
			}
			assert.True(t, named, "the corpus opens on the kind that sorts first")
		})

		t.Run("orders every unit the way a flush leaves it", func(t *testing.T) {
			t.Parallel()

			assertFlushOrder(t, unitsOf(t, backendtest.ScaledFixture(t, nil)))
		})

		t.Run("builds the same corpus twice", func(t *testing.T) {
			t.Parallel()

			first := unitsOf(t, backendtest.ScaledFixture(t, nil))
			second := unitsOf(t, backendtest.ScaledFixture(t, nil))
			assert.Equal(t, len(first), len(second), "the same unit count")
			for _, i := range []int{0, len(first) / 2, len(first) - 1} {
				assert.Equal(t, first[i], second[i],
					"two builds contain the same unit at "+first[i].Key)
			}
		})

		t.Run("fails a backend refusing every canonical kind", func(t *testing.T) {
			t.Parallel()

			everything := map[symbol.Kind]string{}
			for _, k := range fileLevelKinds() {
				everything[k] = kindReason
			}
			failure := assert.Rejects(t, "a corpus without a declaration must fail",
				func(tb assert.TB) {
					backendtest.ScaledFixture(tb, everything)
				})
			assert.Contains(t, failure, "refuses every canonical kind",
				"the failure names why the corpus is empty")
		})
	})

	t.Run("BenchRender", func(t *testing.T) {
		t.Parallel()

		t.Run("renders once per iteration under the stated ceiling", func(t *testing.T) {
			t.Parallel()

			rendered := 0
			setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
					rendered++
					return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}}, nil
				}}, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}
			result := benchRender(setup, roomy)
			assert.True(t, result.N > 0, "the benchmark ran")
			assert.True(t, rendered >= result.N, "every iteration rendered")
		})

		t.Run("records nothing for a budget stating no ceiling", func(t *testing.T) {
			t.Parallel()

			none := benchRender(wellRendered, backendtest.Budget{})
			some := benchRender(wellRendered, roomy)
			assert.Equal(t, none.N, 0, "a benchmark without a ceiling records nothing")
			assert.True(t, some.N > 0, "the same setup under a stated ceiling runs")
		})

		t.Run("builds no fixture for a budget stating no ceiling", func(t *testing.T) {
			t.Parallel()

			var setups int
			benchRender(counted(wellRendered, &setups), backendtest.Budget{})
			assert.Equal(t, setups, 0, "the guard stops before the fixture it would measure")
		})

		t.Run("records nothing for a setup returning no fixture", func(t *testing.T) {
			t.Parallel()

			var setups int
			result := benchRender(counted(hollowSetup, &setups), roomy)
			assert.Equal(t, result.N, 0, "a corpus that does not exist is not measured")
			assert.Equal(t, setups, 1, "the setup ran once and the guard read it")
		})

		t.Run("settles the corpus once before the loop", func(t *testing.T) {
			t.Parallel()

			var built *backendtest.Fixture
			setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				r, f := wellRendered(tb)
				built = f
				return r, f
			}
			result := benchRender(setup, roomy)
			assert.True(t, result.N > 0, "the benchmark ran")
			assert.True(t, built != nil && built.Emit.Settled(),
				"a backend's corpus settles before the measurement, so the "+
					"number is the render's alone")
		})

		t.Run("records nothing for a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			result := benchRender(driftingSetup, roomy)
			assert.Equal(t, result.N, 0, "a settle that fails the plan measures nothing")
		})

		t.Run("records nothing for a corpus the settle reports on", func(t *testing.T) {
			t.Parallel()

			result := benchRender(refusedSetup, roomy)
			assert.Equal(t, result.N, 0,
				"a ceiling over a partial settle measures the wrong thing")
		})

		t.Run("records nothing for a render that aborts", func(t *testing.T) {
			t.Parallel()

			rendered := 0
			aborting := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
					rendered++
					// The files arrive whole, so the error is the only
					// thing left to stop on.
					return []plugin.RenderedFile{{Path: fileA, Body: []byte(bodyX)}},
						errors.New(abortMessage)
				}}, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}
			result := benchRender(aborting, roomy)
			assert.Equal(t, result.N, 0, "an aborted render records no number")
			assert.True(t, rendered > 0, "and the guard read a real render")
		})

		t.Run("records nothing for a render returning no file", func(t *testing.T) {
			t.Parallel()

			rendered := 0
			empty := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
				return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
					rendered++
					return nil, nil
				}}, &backendtest.Fixture{Emit: plugin.NewEmit()}
			}
			result := benchRender(empty, roomy)
			assert.Equal(t, result.N, 0,
				"a number over a partial render measures the wrong thing")
			assert.True(t, rendered > 0, "and the guard read a real render")
		})
	})

	t.Run("BenchSettle", func(t *testing.T) {
		t.Parallel()

		t.Run("builds a fresh fixture for every iteration", func(t *testing.T) {
			t.Parallel()

			var setups int
			result := benchSettle(counted(wellRendered, &setups), roomy)
			assert.True(t, result.N > 0, "the benchmark ran")
			assert.True(t, setups >= result.N,
				"a settled store settles to itself, so every iteration builds one")
		})

		t.Run("records nothing for a budget stating no ceiling", func(t *testing.T) {
			t.Parallel()

			var setups int
			result := benchSettle(counted(wellRendered, &setups), backendtest.Budget{})
			assert.Equal(t, result.N, 0, "a benchmark without a ceiling records nothing")
			assert.Equal(t, setups, 0,
				"and stops before it builds the fixture it would measure")
		})

		t.Run("records nothing for a setup returning no fixture", func(t *testing.T) {
			t.Parallel()

			var setups int
			result := benchSettle(counted(hollowBacked, &setups), roomy)
			assert.Equal(t, result.N, 0, "a corpus that does not exist is not measured")
			assert.Equal(t, setups, 1, "the setup ran once and the guard read it")
		})

		t.Run("records nothing for a renderer declaring no seams", func(t *testing.T) {
			t.Parallel()

			seamless := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				return nil, nil
			})
			result := benchSettle(seamless, roomy)
			assert.Equal(t, result.N, 0,
				"the settle takes the backend's declared seams, and a "+
					"hand-rolled renderer declares none")
		})

		t.Run("records nothing for a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			result := benchSettle(driftingSetup, roomy)
			assert.Equal(t, result.N, 0, "a settle that fails the plan measures nothing")
		})

		t.Run("records nothing for a corpus the settle reports on", func(t *testing.T) {
			t.Parallel()

			result := benchSettle(refusedSetup, roomy)
			assert.Equal(t, result.N, 0,
				"a ceiling over a partial settle measures the wrong thing")
		})
	})
}

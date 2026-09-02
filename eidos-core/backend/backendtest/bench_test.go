// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

func TestScaledFixture(t *testing.T) {
	t.Parallel()

	t.Run("holds the canonical scale", func(t *testing.T) {
		t.Parallel()

		f := backendtest.ScaledFixture(t, fullInventory())
		units, decls := 0, 0
		for u := range f.Emit.Units() {
			units++
			decls += len(u.Decls)
			assert.True(t, len(u.Origins) > 0, "every unit carries provenance")
		}
		assert.Equal(t, units, backendtest.BenchPackages*backendtest.BenchFiles,
			"one unit per package file")
		assert.Equal(t, decls,
			backendtest.BenchPackages*backendtest.BenchFiles*backendtest.BenchDecls,
			"the declared scale, counted at file level")
	})

	t.Run("keeps every declaration inside the inventory", func(t *testing.T) {
		t.Parallel()

		narrow := map[symbol.Kind]string{
			symbol.KindStruct:    "unread",
			symbol.KindInterface: "unread",
		}
		f := backendtest.ScaledFixture(t, narrow)
		for u := range f.Emit.Units() {
			for _, d := range u.Decls {
				assert.True(t, narrow[d.Kind()] != "",
					"no unit smuggles a kind outside the inventory: "+
						d.Kind().String())
			}
		}
	})

	t.Run("builds the same corpus twice", func(t *testing.T) {
		t.Parallel()

		first := unitsOf(t, backendtest.ScaledFixture(t, fullInventory()))
		second := unitsOf(t, backendtest.ScaledFixture(t, fullInventory()))
		assert.Equal(t, len(first), len(second), "the same unit count")
		for _, i := range []int{0, len(first) / 2, len(first) - 1} {
			assert.Equal(t, first[i], second[i],
				"two builds hold the same unit at "+first[i].Key)
		}
	})

	t.Run("rejects a kind it holds no declaration for", func(t *testing.T) {
		t.Parallel()

		failure := assert.Rejects(t, "an uncovered kind must fail",
			func(tb assert.TB) {
				backendtest.ScaledFixture(tb, map[symbol.Kind]string{
					symbol.KindEnumVariant: "unread",
				})
			})
		assert.Contains(t, failure, symbol.KindEnumVariant.String(),
			"the refusal names the kind the fixture does not hold")
	})
}

// roomy is the ceiling a case not about the budget states: high
// enough that the measurement never decides the outcome.
var roomy = backendtest.Budget{MaxAllocs: 1 << 40}

// counted wraps a setup with a call counter, so a case can tell a
// guard that refused before the fixture was built from one that
// refused after.
func counted(s backendtest.Setup, calls *int) backendtest.Setup {
	return func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
		*calls++
		return s(tb)
	}
}

// benchRender runs one BenchRender under a discarded seat, so a
// case reads the guard's effect off the iteration count rather than
// a message b.Fatal drops.
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
// message and returns a zero result, so every case here reads the
// effect: whether the loop ran at all, and how far the setup got
// before it stopped.
func TestBenchRender(t *testing.T) {
	t.Parallel()

	t.Run("drives a renderer under the stated ceiling", func(t *testing.T) {
		t.Parallel()

		rendered := 0
		setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				rendered++
				return []plugin.RenderedFile{{Name: "a.txt", Body: []byte("x\n")}}, nil
			}}, &backendtest.Fixture{Emit: plugin.NewEmit()}
		}
		result := benchRender(setup, roomy)
		assert.True(t, result.N > 0, "the benchmark ran")
		assert.True(t, rendered >= result.N, "every iteration rendered")
	})

	t.Run("refuses a budget stating no ceiling", func(t *testing.T) {
		t.Parallel()

		var unbounded, bounded int
		none := benchRender(counted(wellRendered, &unbounded), backendtest.Budget{})
		some := benchRender(counted(wellRendered, &bounded), roomy)

		assert.Equal(t, none.N, 0, "a benchmark without a ceiling records nothing")
		assert.Equal(t, unbounded, 0,
			"and refuses before it builds the fixture it would measure")
		assert.True(t, some.N > 0 && bounded > 0,
			"the same setup under a stated ceiling runs")
	})

	t.Run("refuses a setup carrying no fixture", func(t *testing.T) {
		t.Parallel()

		var setups int
		result := benchRender(counted(hollowSetup, &setups), roomy)
		assert.Equal(t, result.N, 0, "a corpus that does not exist is not measured")
		assert.Equal(t, setups, 1, "the setup ran once and the guard read it")
	})

	t.Run("settles the corpus once before the loop", func(t *testing.T) {
		t.Parallel()

		var held *backendtest.Fixture
		setup := func(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			r, f := wellRendered(tb)
			held = f
			return r, f
		}
		result := benchRender(setup, roomy)
		assert.True(t, result.N > 0, "the benchmark ran")
		assert.True(t, held != nil && held.Emit.Settled(),
			"a backend's corpus settles before the measurement, so the "+
				"number is the render's alone")
	})

	t.Run("refuses a lowering that drops its input's origin", func(t *testing.T) {
		t.Parallel()

		result := benchRender(driftingSetup, roomy)
		assert.Equal(t, result.N, 0,
			"a settle that fails the plan measures nothing")
	})

	t.Run("refuses a corpus the settle reports on", func(t *testing.T) {
		t.Parallel()

		result := benchRender(refusedSetup, roomy)
		assert.Equal(t, result.N, 0,
			"a ceiling over a partial settle measures the wrong thing")
	})

	t.Run("refuses a render that aborts", func(t *testing.T) {
		t.Parallel()

		rendered := 0
		aborting := func(assert.TB) (plugin.Renderer, *backendtest.Fixture) {
			return &fake{render: func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
				rendered++
				// The files arrive whole, so the error is the only
				// thing left to refuse.
				return []plugin.RenderedFile{{Name: "a.txt", Body: []byte("x\n")}},
					errors.New("kaboom")
			}}, &backendtest.Fixture{Emit: plugin.NewEmit()}
		}
		result := benchRender(aborting, roomy)
		assert.Equal(t, result.N, 0, "an aborted render records no number")
		assert.True(t, rendered > 0, "and the guard read a real render")
	})

	t.Run("refuses a render returning nothing", func(t *testing.T) {
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
}

func TestBenchSettle(t *testing.T) {
	t.Parallel()

	t.Run("settles a fresh fixture per iteration", func(t *testing.T) {
		t.Parallel()

		var setups int
		result := benchSettle(counted(wellRendered, &setups), roomy)
		assert.True(t, result.N > 0, "the benchmark ran")
		assert.True(t, setups >= result.N,
			"a settled store settles to itself, so every iteration builds one")
	})

	t.Run("refuses a budget stating no ceiling", func(t *testing.T) {
		t.Parallel()

		var setups int
		result := benchSettle(counted(wellRendered, &setups), backendtest.Budget{})
		assert.Equal(t, result.N, 0, "a benchmark without a ceiling records nothing")
		assert.Equal(t, setups, 0,
			"and refuses before it builds the fixture it would measure")
	})

	t.Run("refuses a setup carrying no fixture", func(t *testing.T) {
		t.Parallel()

		var setups int
		result := benchSettle(counted(hollowBacked, &setups), roomy)
		assert.Equal(t, result.N, 0, "a corpus that does not exist is not measured")
		assert.Equal(t, setups, 1, "the setup ran once and the guard read it")
	})

	t.Run("refuses a renderer declaring no seams", func(t *testing.T) {
		t.Parallel()

		seamless := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return nil, nil
		})
		result := benchSettle(seamless, roomy)
		assert.Equal(t, result.N, 0,
			"the settle takes the backend's declared seams, and a "+
				"hand-rolled renderer declares none")
	})

	t.Run("refuses a lowering that drops its input's origin", func(t *testing.T) {
		t.Parallel()

		result := benchSettle(driftingSetup, roomy)
		assert.Equal(t, result.N, 0,
			"a settle that fails the plan measures nothing")
	})

	t.Run("refuses a corpus the settle reports on", func(t *testing.T) {
		t.Parallel()

		result := benchSettle(refusedSetup, roomy)
		assert.Equal(t, result.N, 0,
			"a ceiling over a partial settle measures the wrong thing")
	})
}

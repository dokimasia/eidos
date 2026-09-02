// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
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
		result := testing.Benchmark(func(b *testing.B) {
			b.Helper()
			backendtest.BenchRender(b, setup, backendtest.Budget{MaxAllocs: 1 << 40})
		})
		assert.True(t, result.N > 0, "the benchmark ran")
		assert.True(t, rendered >= result.N, "every iteration rendered")
	})
}

func TestBenchSettle(t *testing.T) {
	t.Parallel()

	t.Run("settles a fresh fixture per iteration", func(t *testing.T) {
		t.Parallel()

		result := testing.Benchmark(func(b *testing.B) {
			b.Helper()
			backendtest.BenchSettle(b, wellRendered, backendtest.Budget{MaxAllocs: 1 << 40})
		})
		assert.True(t, result.N > 0, "the benchmark ran")
	})
}

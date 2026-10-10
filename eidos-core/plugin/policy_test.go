// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/plugin"
)

// The target whose policies the cases resolve, its two policy keys and
// the choices they take.
const (
	policyTarget plugin.Target    = "typescript"
	int64Key     plugin.PolicyKey = "typescript.int64"
	absentKey    plugin.PolicyKey = "typescript.absent"
	bigint       plugin.Choice    = "bigint"
	text         plugin.Choice    = "string"
	number       plugin.Choice    = "number"
	undefined    plugin.Choice    = "undefined"
	null         plugin.Choice    = "null"
)

// policySpecs returns the two policies of the fixture target, the
// absent policy first, so a resolution sorts them.
func policySpecs() []plugin.PolicySpec {
	return []plugin.PolicySpec{
		{
			Key: absentKey, Choices: []plugin.Choice{undefined, null}, Default: undefined,
			Doc: "the spelling of an absent value",
		},
		{
			Key: int64Key, Choices: []plugin.Choice{bigint, text, number}, Default: bigint,
			Doc: "the spelling of an integer of 64 bits",
		},
	}
}

// resolved returns the fixture target's policy with chosen selected,
// failing the test on a fault.
func resolved(tb assert.TB, chosen map[plugin.PolicyKey]plugin.Choice) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(policyTarget, policySpecs(), chosen)
	assert.NoError(tb, err, "the fixture policy resolves")
	return p
}

// TestPolicy checks the faults that NewPolicy returns and the choices
// that a spoke reads from a resolved policy.
func TestPolicy(t *testing.T) {
	t.Parallel()

	t.Run("PolicyKey", func(t *testing.T) {
		t.Parallel()

		t.Run("Param", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give plugin.PolicyKey
				want directive.ParamKey
			}{
				{name: "returns the name after the dot", give: int64Key, want: "int64"},
				{name: "returns the empty param for a key without a dot", give: "int64", want: ""},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Param(), tt.want, "the param is the policy's name")
				})
			}
		})
	})

	t.Run("NewPolicy", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each spec's default where nothing selects a choice", func(t *testing.T) {
			t.Parallel()

			p := resolved(t, nil)
			expect.Equal(t, p.Choice(int64Key), bigint, "the int64 policy takes its default")
			expect.Equal(t, p.Choice(absentKey), undefined, "the absent policy takes its default")
		})

		t.Run("returns the choice that chosen selects", func(t *testing.T) {
			t.Parallel()

			p := resolved(t, map[plugin.PolicyKey]plugin.Choice{int64Key: text})
			expect.Equal(t, p.Choice(int64Key), text, "the int64 policy takes the selected choice")
			expect.Equal(t, p.Choice(absentKey), undefined, "the absent policy keeps its default")
		})

		spec := func(edit func(*plugin.PolicySpec)) []plugin.PolicySpec {
			specs := policySpecs()
			edit(&specs[1])
			return specs
		}
		tests := []struct {
			name   string
			specs  []plugin.PolicySpec
			chosen map[plugin.PolicyKey]plugin.Choice
			want   string
		}{
			{
				name:  "returns an error for a spec without a key",
				specs: spec(func(s *plugin.PolicySpec) { s.Key = "" }),
				want:  "a policy without a key",
			},
			{
				name:  "returns an error for a key in another target's namespace",
				specs: spec(func(s *plugin.PolicySpec) { s.Key = "golang.int64" }),
				want:  "outside the namespace of target typescript",
			},
			{
				name:  "returns an error for a key without a namespace",
				specs: spec(func(s *plugin.PolicySpec) { s.Key = "int64" }),
				want:  "outside the namespace of target typescript",
			},
			{
				name:  "returns an error for a name that is not a valid param key",
				specs: spec(func(s *plugin.PolicySpec) { s.Key = "typescript.64bit" }),
				want:  "is not a valid param key of a directive",
			},
			{
				name:  "returns an error for a policy named after the name param",
				specs: spec(func(s *plugin.PolicySpec) { s.Key = "typescript.name" }),
				want:  "keeps for a name",
			},
			{
				name:  "returns an error for a spec without choices",
				specs: spec(func(s *plugin.PolicySpec) { s.Choices = nil }),
				want:  "does not declare a choice",
			},
			{
				name:  "returns an error for an empty choice",
				specs: spec(func(s *plugin.PolicySpec) { s.Choices = []plugin.Choice{bigint, ""} }),
				want:  "an empty choice",
			},
			{
				name:  "returns an error for a choice that a spec declares twice",
				specs: spec(func(s *plugin.PolicySpec) { s.Choices = []plugin.Choice{bigint, bigint} }),
				want:  "choice bigint twice",
			},
			{
				name:  "returns an error for a default outside the choices",
				specs: spec(func(s *plugin.PolicySpec) { s.Default = undefined }),
				want:  `defaults to "undefined", which is not one of bigint, string, number`,
			},
			{
				name:  "returns an error for a spec without documentation",
				specs: spec(func(s *plugin.PolicySpec) { s.Doc = "" }),
				want:  "has no documentation",
			},
			{
				name:  "returns an error for a key that two specs declare",
				specs: spec(func(s *plugin.PolicySpec) { s.Key = absentKey; s.Choices = []plugin.Choice{undefined} }),
				want:  "declares policy typescript.absent twice",
			},
			{
				name:   "returns an error with the declared keys for a key that no spec declares",
				specs:  policySpecs(),
				chosen: map[plugin.PolicyKey]plugin.Choice{"typescript.bytes": "string"},
				want:   "its policies are typescript.absent, typescript.int64",
			},
			{
				name:   "returns an error for a selection of a target without policies",
				chosen: map[plugin.PolicyKey]plugin.Choice{int64Key: text},
				want:   "target typescript does not declare a policy, and typescript.int64 is selected",
			},
			{
				name:   "returns an error with the choices for a choice outside them",
				specs:  policySpecs(),
				chosen: map[plugin.PolicyKey]plugin.Choice{int64Key: "long"},
				want:   `takes one of bigint, string, number, not "long"`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := plugin.NewPolicy(policyTarget, tt.specs, tt.chosen)
				assert.HasError(t, err, "the resolution fails")
				assert.Contains(t, err.Error(), tt.want, "the error contains the fault")
				assert.HasPrefix(t, err.Error(), "plugin: ", "the error has the package prefix")
			})
		}

		t.Run("returns an error for each fault", func(t *testing.T) {
			t.Parallel()

			specs := policySpecs()
			specs[0].Doc = ""
			_, err := plugin.NewPolicy(policyTarget, specs, map[plugin.PolicyKey]plugin.Choice{int64Key: "long"})
			assert.HasError(t, err, "the resolution fails")
			assert.Contains(t, err.Error(), "typescript.absent has no documentation",
				"the error contains the fault of the spec")
			assert.Contains(t, err.Error(), `not "long"`, "the error contains the fault of the selection")
		})
	})

	t.Run("Choice", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the override's choice where the override reports one", func(t *testing.T) {
			t.Parallel()

			p := resolved(t, nil).Overridden(func(k plugin.PolicyKey) (plugin.Choice, bool) {
				return number, k == int64Key
			})
			assert.Equal(t, p.Choice(int64Key), number, "the override's choice applies")
		})

		t.Run("returns the resolved choice where the override reports none", func(t *testing.T) {
			t.Parallel()

			p := resolved(t, nil).Overridden(func(k plugin.PolicyKey) (plugin.Choice, bool) {
				return number, k == int64Key
			})
			assert.Equal(t, p.Choice(absentKey), undefined, "the resolved choice applies")
		})

		t.Run("panics for a key that the policy does not declare", func(t *testing.T) {
			t.Parallel()

			p := resolved(t, nil)
			got := assert.Panics(t, func() { p.Choice("typescript.bytes") }, "a spoke reads its backend's keys alone")
			assert.Contains(t, got, "typescript.bytes", "the panic value contains the key")
		})

		t.Run("panics for any key of the zero policy", func(t *testing.T) {
			t.Parallel()

			var p plugin.Policy
			assert.Panics(t, func() { p.Choice(int64Key) }, "the zero policy has no keys")
		})
	})

	t.Run("Overridden", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves the policy that it copies without the override", func(t *testing.T) {
			t.Parallel()

			p := resolved(t, nil)
			p.Overridden(func(plugin.PolicyKey) (plugin.Choice, bool) { return number, true })
			assert.Equal(t, p.Choice(int64Key), bigint, "the original keeps its resolved choice")
		})

		t.Run("returns a copy without an override for a nil override", func(t *testing.T) {
			t.Parallel()

			p := resolved(t, nil).
				Overridden(func(plugin.PolicyKey) (plugin.Choice, bool) { return number, true }).
				Overridden(nil)
			assert.Equal(t, p.Choice(int64Key), bigint, "the copy reads the resolved choice")
		})
	})
}

// A spoke reads a policy on every translation, so Choice and Overridden
// allocate nothing, in the ordinary run, which runs no benchmark. The
// check runs alone, because the count includes every goroutine's
// allocations.
func TestPolicyAllocs(t *testing.T) {
	p := resolved(t, map[plugin.PolicyKey]plugin.Choice{int64Key: text})
	override := func(k plugin.PolicyKey) (plugin.Choice, bool) { return number, k == int64Key }
	var got plugin.Choice
	assert.MaxAllocs(t, func() { got = p.Choice(int64Key) }, 0, "Choice allocates nothing")
	assert.Equal(t, got, text, "Choice returns the selected choice")
	var copied plugin.Policy
	assert.MaxAllocs(t, func() { copied = p.Overridden(override) }, 0, "Overridden allocates nothing")
	assert.MaxAllocs(t, func() { got = copied.Choice(int64Key) }, 0, "Choice allocates nothing through an override")
	assert.Equal(t, got, number, "Choice returns the override's choice")
}

// BenchmarkPolicy measures the reads a spoke makes on every translation:
// a choice, a choice through an override, and the copy that the
// authoring kit makes for one translation.
func BenchmarkPolicy(b *testing.B) {
	p := resolved(b, map[plugin.PolicyKey]plugin.Choice{int64Key: text})
	override := func(k plugin.PolicyKey) (plugin.Choice, bool) { return number, k == int64Key }

	b.Run("Choice", func(b *testing.B) {
		b.Run("a resolved choice", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got plugin.Choice
			for c.Loop() {
				got = p.Choice(int64Key)
			}
			assert.Equal(b, got, text, "Choice returns the selected choice")
		})

		b.Run("an override's choice", func(b *testing.B) {
			copied := p.Overridden(override)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got plugin.Choice
			for c.Loop() {
				got = copied.Choice(int64Key)
			}
			assert.Equal(b, got, number, "Choice returns the override's choice")
		})
	})

	b.Run("Overridden", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var copied plugin.Policy
		for c.Loop() {
			copied = p.Overridden(override)
		}
		assert.Equal(b, copied.Choice(int64Key), number, "Overridden returns a copy that asks the override")
	})
}

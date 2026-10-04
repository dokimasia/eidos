// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag_test

import (
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
)

// testPrefix is the prefix of the codes these cases register, so nothing
// they claim collides with a kernel package's own.
const testPrefix diag.Prefix = "EIDTEST"

// claimed counts the numbers the cases have taken under
// [testPrefix].
var claimed atomic.Int64

// firstSpec is the registration the allocation checks and the
// benchmarks make: number 7, with its meaning.
var firstSpec = diag.CodeSpec{Number: 7, Meaning: "a code the cases register"}

// The ceilings of a registry's construction and of its first
// registration.
const (
	// newRegistryAllocs is one empty registry: the registry and its map.
	newRegistryAllocs = 2
	// registerAllocs is one first registration into a new registry: the
	// registry, its map, and the map's first group.
	registerAllocs = newRegistryAllocs + 1
)

// A code spells as its prefix and its padded number, and a registry
// refuses a number claimed twice under one prefix.
func TestCode(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			code diag.Code
			want string
		}{
			{
				name: "pads a small number",
				code: diag.Code{Prefix: diag.KernelPrefix, Number: 7},
				want: "EID-0007",
			},
			{
				name: "leaves a wide number alone",
				code: diag.Code{Prefix: "EIDGO", Number: 412},
				want: "EIDGO-0412",
			},
			{
				name: "does not truncate a number wider than the padding",
				code: diag.Code{Prefix: diag.KernelPrefix, Number: 12345},
				want: "EID-12345",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.code.String(), tt.want,
					"the code spells its prefix and padded number")
			})
		}
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the zero code", func(t *testing.T) {
			t.Parallel()

			assert.True(t, (diag.Code{}).IsZero(), "the zero Code names nothing")
		})

		t.Run("reports false for a registered code", func(t *testing.T) {
			t.Parallel()

			assert.False(t, (diag.Code{Prefix: diag.KernelPrefix, Number: 1}).IsZero(),
				"a registered Code names a finding")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the code it recorded", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			got, err := r.Register(diag.KernelPrefix, diag.CodeSpec{
				Number: 1, Meaning: "a declaration was added after Freeze",
			})
			assert.NoError(t, err, "a fresh number registers")
			assert.Equal(t, got, diag.Code{Prefix: diag.KernelPrefix, Number: 1},
				"Register returns the code it recorded")
			meaning, known := r.Meaning(got)
			assert.True(t, known, "the registry then knows the code")
			assert.NotEqual(t, meaning, "", "and returns its meaning")
		})

		t.Run("returns an error naming both meanings for a number claimed twice", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			first := diag.CodeSpec{Number: 1, Meaning: "the first claim"}
			second := diag.CodeSpec{Number: 1, Meaning: "the second claim"}
			_, err := r.Register(diag.KernelPrefix, first)
			assert.NoError(t, err, "the first claim registers")

			_, err = r.Register(diag.KernelPrefix, second)
			assert.HasError(t, err, "a number claimed twice is refused")
			assert.Contains(t, err.Error(), first.Meaning,
				"the refusal names the meaning registered first")
			assert.Contains(t, err.Error(), second.Meaning,
				"and the meaning that tried to claim it")
			assert.HasPrefix(t, err.Error(), "diag: ",
				"the error has the package prefix")
		})

		t.Run("registers one number under two prefixes", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			spec := diag.CodeSpec{Number: 1, Meaning: "the same number under another prefix"}
			_, err := r.Register(diag.KernelPrefix, spec)
			assert.NoError(t, err, "the number registers under the first prefix")
			_, err = r.Register("EIDGO", spec)
			assert.NoError(t, err, "and again under a second, because each prefix numbers its own codes")
		})

		t.Run("returns an error for a spec without a meaning", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			_, err := r.Register(diag.KernelPrefix, diag.CodeSpec{Number: 1})
			assert.HasError(t, err, "a spec without a meaning is refused, because the index anchors to it")
		})

		numbers := []struct {
			name string
			give int
		}{
			{name: "returns an error for the number zero", give: 0},
			{name: "returns an error for a negative number", give: -5},
		}
		for _, tt := range numbers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				spec := diag.CodeSpec{Number: tt.give, Meaning: "unnumbered"}
				_, err := diag.NewRegistry().Register(diag.KernelPrefix, spec)
				assert.HasError(t, err, "a code counts from one")
			})
		}

		t.Run("returns an error for the empty prefix", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			_, err := r.Register("", diag.CodeSpec{Number: 1, Meaning: "without a prefix"})
			assert.HasError(t, err, "a code belongs to the registrant of its prefix")
		})

		prefixes := []struct {
			name string
			give diag.Prefix
		}{
			{name: "returns an error for a lowercase prefix", give: "eid"},
			{name: "returns an error for a prefix with a hyphen", give: "E-D"},
			{name: "returns an error for a prefix with a digit", give: "E1D"},
			{name: "returns an error for a prefix with a space", give: "E D"},
		}
		for _, tt := range prefixes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := diag.NewRegistry().Register(tt.give, diag.CodeSpec{Number: 1, Meaning: "styled"})
				assert.HasError(t, err, "a prefix is uppercase letters alone, so PREFIX-NNNN reads back")
			})
		}
	})

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the kernel's prefix", func(t *testing.T) {
			t.Parallel()

			assert.True(t, diag.KernelPrefix.Valid(), "the kernel's prefix is valid")
		})

		t.Run("reports true for uppercase letters", func(t *testing.T) {
			t.Parallel()

			assert.True(t, diag.Prefix("GOLANG").Valid(), "uppercase letters are a prefix")
		})

		tests := []struct {
			name string
			give diag.Prefix
		}{
			{name: "reports false for the empty prefix", give: ""},
			{name: "reports false for lowercase letters", give: "eid"},
			{name: "reports false for a digit", give: "E1D"},
			{name: "reports false for a hyphen", give: "E-D"},
			{name: "reports false for a space", give: "E D"},
			{name: "reports false for a letter outside ASCII", give: "ÉID"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.False(t, tt.give.Valid(), "a prefix is uppercase ASCII letters alone")
			})
		}
	})

	// The kernel registry is package state, so its cases run in
	// sequence and not in parallel: they would otherwise race on the map
	// and see each other's registrations. That state outlives one test
	// run too, so every case claims a number no other has, which is what
	// lets the suite run twice in one binary.
	t.Run("MustRegister", func(t *testing.T) {
		t.Run("records into the kernel registry", func(t *testing.T) {
			spec := diag.CodeSpec{
				Number:  nextTestNumber(),
				Meaning: "a code declared where it is registered",
			}
			got := diag.MustRegister(testPrefix, spec)

			assert.Equal(t, got, diag.Code{Prefix: testPrefix, Number: spec.Number},
				"MustRegister returns the code it recorded")
			meaning, known := diag.Kernel().Meaning(got)
			assert.True(t, known, "the kernel registry knows it")
			assert.Equal(t, meaning, spec.Meaning, "under the declared meaning")
		})

		t.Run("panics for a number claimed twice", func(t *testing.T) {
			spec := diag.CodeSpec{Number: nextTestNumber(), Meaning: "the first claim"}
			diag.MustRegister(testPrefix, spec)
			spec.Meaning = "the second claim"

			assert.Panics(t, func() { diag.MustRegister(testPrefix, spec) },
				"a duplicate at package initialization is a defect, not a condition")
		})
	})

	t.Run("Kernel", func(t *testing.T) {
		t.Run("returns one registry for every call", func(t *testing.T) {
			first, second := diag.Kernel(), diag.Kernel()
			assert.True(t, first == second,
				"a code registered into one registry would be missing from another")
		})
	})

	t.Run("Codes", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the codes in prefix then number order", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			for _, in := range []diag.Code{
				{Prefix: "EIDGO", Number: 2},
				{Prefix: diag.KernelPrefix, Number: 9},
				{Prefix: "EIDGO", Number: 1},
				{Prefix: diag.KernelPrefix, Number: 3},
			} {
				_, err := r.Register(in.Prefix, diag.CodeSpec{
					Number: in.Number, Meaning: "registered",
				})
				assert.NoError(t, err, "every fixture code registers")
			}

			var got []string
			for _, c := range r.Codes() {
				got = append(got, c.String())
			}
			assert.Equal(t, got, []string{"EID-0003", "EID-0009", "EIDGO-0001", "EIDGO-0002"},
				"Codes returns prefix then number order")
		})
	})
}

// The registry's reads and a code's checks allocate nothing, a code's
// spelling and the sorted list of codes allocate what they return, a
// new registry allocates itself and its map, and a registration
// allocates its first map group. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestCodeAllocs(t *testing.T) {
	code := diag.Code{Prefix: diag.KernelPrefix, Number: 7}
	r := registryOf(t, code)
	assert.MaxAllocs(t, func() {
		if !diag.KernelPrefix.Valid() {
			t.Fatal("Valid refused the kernel's prefix")
		}
	}, 0, "Valid allocates nothing")
	var fresh *diag.Registry
	assert.MaxAllocs(t, func() { fresh = diag.NewRegistry() }, newRegistryAllocs,
		"NewRegistry allocates the registry and its map")
	assert.Empty(t, fresh.Codes(), "NewRegistry returns a registry without a code")
	assert.MaxAllocs(t, func() {
		fresh = diag.NewRegistry()
		if _, err := fresh.Register(diag.KernelPrefix, firstSpec); err != nil {
			t.Fatalf("Register: unexpected error: %v", err)
		}
	}, registerAllocs, "NewRegistry and a first Register allocate the registry and its map")
	assert.MaxAllocs(t, func() {
		diag.MustRegister(testPrefix, diag.CodeSpec{Number: nextTestNumber(), Meaning: firstSpec.Meaning})
	}, 0, "MustRegister allocates only to grow the kernel registry, below once per call")
	assert.MaxAllocs(t, func() {
		if diag.Kernel() == nil {
			t.Fatal("Kernel returned no registry")
		}
	}, 0, "Kernel allocates nothing")
	assert.MaxAllocs(t, func() {
		if _, known := r.Meaning(code); !known {
			t.Fatal("Meaning missed a registered code")
		}
	}, 0, "Meaning allocates nothing")
	assert.MaxAllocs(t, func() {
		if len(r.Codes()) != 1 {
			t.Fatal("Codes missed a registered code")
		}
	}, 1, "Codes allocates the list it returns")
	assert.MaxAllocs(t, func() {
		if code.IsZero() {
			t.Fatal("IsZero reported a registered code as zero")
		}
	}, 0, "IsZero allocates nothing")
	assert.MaxAllocs(t, func() {
		if code.String() != "EID-0007" {
			t.Fatal("String spelled another code")
		}
	}, 1, "String allocates the string it returns")
}

// BenchmarkCode measures the registry and a code's spelling, which
// every diagnostic a run reports and every refusal it returns contains.
func BenchmarkCode(b *testing.B) {
	code := diag.Code{Prefix: diag.KernelPrefix, Number: 7}
	r := registryOf(b, code)

	b.Run("Valid", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = diag.KernelPrefix.Valid()
		}
		assert.True(b, got, "the kernel's prefix is valid")
	})

	b.Run("NewRegistry", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newRegistryAllocs)
		defer c.End()
		var got *diag.Registry
		for c.Loop() {
			got = diag.NewRegistry()
		}
		assert.Empty(b, got.Codes(), "a new registry contains no code")
	})

	b.Run("Register", func(b *testing.B) {
		b.Run("a first code into a new registry", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(registerAllocs)
			defer c.End()
			var (
				fresh *diag.Registry
				got   diag.Code
				err   error
			)
			for c.Loop() {
				fresh = diag.NewRegistry()
				got, err = fresh.Register(diag.KernelPrefix, firstSpec)
			}
			assert.NoError(b, err, "a fresh number registers")
			assert.Equal(b, fresh.Codes(), []diag.Code{got}, "Register records the code it returns")
		})
	})

	b.Run("MustRegister", func(b *testing.B) {
		// Each call claims a number no call claimed before, so the
		// kernel registry grows. Its map doubles, so the growth averages
		// below one allocation per call, and the contract floors the
		// average to zero. A run of one iteration that includes a growth
		// counts that growth.
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got diag.Code
		for c.Loop() {
			got = diag.MustRegister(testPrefix, diag.CodeSpec{Number: nextTestNumber(), Meaning: firstSpec.Meaning})
		}
		_, known := diag.Kernel().Meaning(got)
		assert.True(b, known, "the kernel registry knows the last code")
	})

	b.Run("Kernel", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got *diag.Registry
		for c.Loop() {
			got = diag.Kernel()
		}
		assert.NotNil(b, got, "Kernel returns the registry MustRegister records into")
	})

	b.Run("Registry.Meaning", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got, _ = r.Meaning(code)
		}
		assert.Equal(b, got, firstSpec.Meaning, "Meaning returns the registered meaning")
	})

	b.Run("Registry.Codes", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got []diag.Code
		for c.Loop() {
			got = r.Codes()
		}
		assert.Equal(b, got, []diag.Code{code}, "Codes returns the registered code")
	})

	b.Run("IsZero", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := true
		for c.Loop() {
			got = code.IsZero()
		}
		assert.False(b, got, "a registered code names a finding")
	})

	b.Run("String", func(b *testing.B) {
		code := diag.Code{Prefix: diag.KernelPrefix, Number: 7}
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = code.String()
		}
		assert.Equal(b, got, "EID-0007", "String spells the prefix and the padded number")
	})
}

// registryOf returns a new registry that contains one code, under the
// meaning of [firstSpec].
func registryOf(tb testing.TB, c diag.Code) *diag.Registry {
	tb.Helper()

	r := diag.NewRegistry()
	_, err := r.Register(c.Prefix, diag.CodeSpec{Number: c.Number, Meaning: firstSpec.Meaning})
	assert.NoError(tb, err, "the code registers")
	return r
}

// nextTestNumber returns a code number no case has claimed.
func nextTestNumber() int { return int(claimed.Add(1)) }

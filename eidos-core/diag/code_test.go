// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
)

// testPrefix owns the codes these cases register, so nothing they
// claim collides with a kernel package's own.
const testPrefix diag.Prefix = "EIDTEST"

// claimed counts the numbers the cases have taken under
// [testPrefix].
var claimed atomic.Int64

// nextTestNumber answers a code number no case has claimed.
func nextTestNumber() int { return int(claimed.Add(1)) }

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

		assert.True(t, (diag.Code{}).IsZero(),
			"the zero Code names nothing")
		assert.False(t, (diag.Code{Prefix: diag.KernelPrefix, Number: 1}).IsZero(),
			"a registered Code names a finding")
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the code it recorded", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			got, err := r.Register(diag.KernelPrefix, diag.CodeSpec{
				Number: 1, Meaning: "a declaration was added after Freeze",
			})
			assert.NoError(t, err, "a fresh number registers")
			assert.Equal(t, got, diag.Code{Prefix: diag.KernelPrefix, Number: 1},
				"Register answers the code it recorded")
			meaning, known := r.Meaning(got)
			assert.True(t, known, "the registry then knows the code")
			assert.NotEqual(t, meaning, "", "and holds its meaning")
		})

		t.Run("refuses a number claimed twice in one prefix", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			first := diag.CodeSpec{Number: 1, Meaning: "the first claim"}
			second := diag.CodeSpec{Number: 1, Meaning: "the second claim"}
			_, err := r.Register(diag.KernelPrefix, first)
			assert.NoError(t, err, "the first claim registers")

			_, err = r.Register(diag.KernelPrefix, second)
			assert.HasError(t, err, "a number claimed twice is refused")
			assert.Contains(t, err.Error(), first.Meaning,
				"the refusal names the meaning already held")
			assert.Contains(t, err.Error(), second.Meaning,
				"and the meaning that tried to claim it")
			assert.HasPrefix(t, err.Error(), "diag: ",
				"the error carries the package prefix")
		})

		t.Run("admits one number under two prefixes", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			spec := diag.CodeSpec{Number: 1, Meaning: "the same number, another owner"}
			_, err := r.Register(diag.KernelPrefix, spec)
			assert.NoError(t, err, "the number registers under the first prefix")
			_, err = r.Register("EIDGO", spec)
			assert.NoError(t, err, "and again under a second: prefixes own their ranges")
		})

		t.Run("refuses a spec that names no meaning", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			_, err := r.Register(diag.KernelPrefix, diag.CodeSpec{Number: 1})
			assert.HasError(t, err, "a spec without a meaning is refused: the index anchors to it")
		})

		t.Run("refuses a prefix that owns nothing", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			_, err := r.Register("", diag.CodeSpec{Number: 1, Meaning: "unowned"})
			assert.HasError(t, err, "a code belongs to whoever owns its prefix")
		})
	})

	// The kernel registry is package state, so its cases run in
	// sequence rather than in parallel: they would otherwise race on
	// the map and see each other's registrations. That state outlives
	// one test run too, so every case claims a number no other has,
	// which is what lets the suite run twice in one binary.
	t.Run("MustRegister", func(t *testing.T) {
		t.Run("records into the kernel registry", func(t *testing.T) {
			spec := diag.CodeSpec{
				Number:  nextTestNumber(),
				Meaning: "a code declared where it is registered",
			}
			got := diag.MustRegister(testPrefix, spec)

			assert.Equal(t, got, diag.Code{Prefix: testPrefix, Number: spec.Number},
				"MustRegister answers the code it recorded")
			meaning, known := diag.Kernel().Meaning(got)
			assert.True(t, known, "the kernel registry holds it")
			assert.Equal(t, meaning, spec.Meaning, "under the declared meaning")
		})

		t.Run("panics on a number claimed twice", func(t *testing.T) {
			spec := diag.CodeSpec{Number: nextTestNumber(), Meaning: "the first claim"}
			diag.MustRegister(testPrefix, spec)
			spec.Meaning = "the second claim"

			assert.Panics(t, func() { diag.MustRegister(testPrefix, spec) },
				"a duplicate at package initialization is a defect, not a condition")
		})
	})

	t.Run("Kernel", func(t *testing.T) {
		first, second := diag.Kernel(), diag.Kernel()
		assert.True(t, first == second,
			"Kernel answers one registry, or a code registered into one would be missing from the other")
	})

	t.Run("Codes", func(t *testing.T) {
		t.Parallel()

		t.Run("answers in prefix then number order", func(t *testing.T) {
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
				"Codes answers prefix then number order")
		})
	})
}

// A code is spelled into every diagnostic a run reports and into
// every refusal it answers, so its rendering is on a hot path.
func BenchmarkCode(b *testing.B) {
	b.Run("String", func(b *testing.B) {
		b.ReportAllocs()

		code := diag.Code{Prefix: diag.KernelPrefix, Number: 7}
		for b.Loop() {
			_ = code.String()
		}
	})
}

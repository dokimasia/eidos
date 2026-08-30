// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"strings"
	"sync/atomic"
	"testing"

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
				if got := tt.code.String(); got != tt.want {
					t.Fatalf("String() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		if !(diag.Code{}).IsZero() {
			t.Fatal("the zero Code: IsZero() = false, want true")
		}
		if (diag.Code{Prefix: diag.KernelPrefix, Number: 1}).IsZero() {
			t.Fatal("a registered Code: IsZero() = true, want false")
		}
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("answers the code it recorded", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			got, err := r.Register(diag.KernelPrefix, diag.CodeSpec{
				Number: 1, Meaning: "a declaration was added after Freeze",
			})
			if err != nil {
				t.Fatalf("Register: unexpected error: %v", err)
			}
			want := diag.Code{Prefix: diag.KernelPrefix, Number: 1}
			if got != want {
				t.Fatalf("Register = %v, want %v", got, want)
			}
			if meaning, known := r.Meaning(got); !known || meaning == "" {
				t.Fatalf("Meaning(%v) = %q, %t; want the recorded meaning", got, meaning, known)
			}
		})

		t.Run("refuses a number claimed twice in one prefix", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			first := diag.CodeSpec{Number: 1, Meaning: "the first claim"}
			second := diag.CodeSpec{Number: 1, Meaning: "the second claim"}
			if _, err := r.Register(diag.KernelPrefix, first); err != nil {
				t.Fatalf("Register: unexpected error: %v", err)
			}

			_, err := r.Register(diag.KernelPrefix, second)
			if err == nil {
				t.Fatal("Register: error = nil, want the duplicate refused")
			}
			for _, want := range []string{first.Meaning, second.Meaning} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %q, want it to name %q", err, want)
				}
			}
			if !strings.HasPrefix(err.Error(), "diag: ") {
				t.Fatalf("error = %q, want the package prefix", err)
			}
		})

		t.Run("admits one number under two prefixes", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			spec := diag.CodeSpec{Number: 1, Meaning: "the same number, another owner"}
			if _, err := r.Register(diag.KernelPrefix, spec); err != nil {
				t.Fatalf("Register: unexpected error: %v", err)
			}
			if _, err := r.Register("EIDGO", spec); err != nil {
				t.Fatalf("Register under a second prefix: unexpected error: %v", err)
			}
		})

		t.Run("refuses a spec that names no meaning", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			if _, err := r.Register(diag.KernelPrefix, diag.CodeSpec{Number: 1}); err == nil {
				t.Fatal("Register without a meaning: error = nil, want non-nil")
			}
		})

		t.Run("refuses a prefix that owns nothing", func(t *testing.T) {
			t.Parallel()

			r := diag.NewRegistry()
			_, err := r.Register("", diag.CodeSpec{Number: 1, Meaning: "unowned"})
			if err == nil {
				t.Fatal("Register under the empty prefix: error = nil, want non-nil")
			}
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

			if want := (diag.Code{Prefix: testPrefix, Number: spec.Number}); got != want {
				t.Fatalf("MustRegister = %v, want %v", got, want)
			}
			meaning, known := diag.Kernel().Meaning(got)
			if !known || meaning != spec.Meaning {
				t.Fatalf("Kernel().Meaning(%v) = %q, %t; want %q, true",
					got, meaning, known, spec.Meaning)
			}
		})

		t.Run("panics on a number claimed twice", func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("MustRegister: recover() = nil, want the duplicate panic")
				}
			}()

			spec := diag.CodeSpec{Number: nextTestNumber(), Meaning: "the first claim"}
			diag.MustRegister(testPrefix, spec)
			spec.Meaning = "the second claim"
			diag.MustRegister(testPrefix, spec)
		})
	})

	t.Run("Kernel", func(t *testing.T) {
		first, second := diag.Kernel(), diag.Kernel()
		if first != second {
			t.Fatal("Kernel() answered two registries: a code registered " +
				"into one would be missing from the other")
		}
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
				if _, err := r.Register(in.Prefix, diag.CodeSpec{
					Number: in.Number, Meaning: "registered",
				}); err != nil {
					t.Fatalf("Register: unexpected error: %v", err)
				}
			}

			var got []string
			for _, c := range r.Codes() {
				got = append(got, c.String())
			}
			want := "EID-0003,EID-0009,EIDGO-0001,EIDGO-0002"
			if strings.Join(got, ",") != want {
				t.Fatalf("Codes() = %v, want %s", got, want)
			}
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

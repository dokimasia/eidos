// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
)

// somewhere is a position for a finding whose location is beside the
// point of the case reporting it.
var somewhere = position.Pos{File: "svc/store.go", Line: 1, Col: 1}

// The sink is what every layer reports into, so its order is a
// contract and its concurrency safety is load-bearing: frontends
// report while running alongside each other.
func TestSink(t *testing.T) {
	t.Parallel()

	// collect answers everything a sink holds, in the order it
	// answers it.
	collect := func(t *testing.T, s *diag.Sink) []diag.Diag {
		t.Helper()
		return slices.Collect(s.All())
	}

	t.Run("Report", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches the finding it was given", func(t *testing.T) {
			t.Parallel()

			want := diag.Diag{
				Code:     diag.Code{Prefix: diag.KernelPrefix, Number: 1},
				Severity: diag.SeverityWarning,
				Pos:      somewhere,
				Msg:      "Store carries a deprecated directive",
				Origin:   diag.PhaseAnnotate,
			}
			s := diag.NewSink()
			s.Report(want)

			got := collect(t, s)
			if len(got) != 1 {
				t.Fatalf("All() answered %d findings, want 1", len(got))
			}
			if got[0].Code != want.Code || got[0].Severity != want.Severity ||
				got[0].Pos != want.Pos || got[0].Msg != want.Msg ||
				got[0].Origin != want.Origin {
				t.Fatalf("All() = %+v, want %+v", got[0], want)
			}
		})

		t.Run("is safe to call concurrently", func(t *testing.T) {
			t.Parallel()

			const (
				reporters = 8
				each      = 64
			)
			s := diag.NewSink()
			var wg sync.WaitGroup
			for reporter := range reporters {
				wg.Go(func() {
					for range each {
						s.Errorf(diag.Code{Prefix: diag.KernelPrefix, Number: reporter},
							somewhere, diag.PhaseLoad, "finding from reporter %d", reporter)
					}
				})
			}
			wg.Wait()

			if got := len(collect(t, s)); got != reporters*each {
				t.Fatalf("All() answered %d findings, want %d: a report was lost",
					got, reporters*each)
			}
			if !s.Failed() {
				t.Fatal("Failed() = false after concurrent errors, want true")
			}
		})

		t.Run("is safe to call while a caller reads", func(t *testing.T) {
			t.Parallel()

			const each = 64
			s := diag.NewSink()
			var wg sync.WaitGroup
			wg.Go(func() {
				for range each {
					s.Report(diag.Diag{Pos: somewhere, Origin: diag.PhaseLoad})
				}
			})
			wg.Go(func() {
				for range each {
					for range s.All() {
						break
					}
				}
			})
			wg.Wait()
		})
	})

	t.Run("Errorf", func(t *testing.T) {
		t.Parallel()

		t.Run("reports at Error and formats the message", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			code := diag.Code{Prefix: diag.KernelPrefix, Number: 1}
			s.Errorf(code, somewhere, diag.PhaseFreeze, "%s is added after Freeze", "Store")

			got := collect(t, s)
			if len(got) != 1 {
				t.Fatalf("All() answered %d findings, want 1", len(got))
			}
			if got[0].Severity != diag.SeverityError {
				t.Fatalf("Severity = %v, want %v", got[0].Severity, diag.SeverityError)
			}
			if got[0].Code != code {
				t.Fatalf("Code = %v, want %v", got[0].Code, code)
			}
			if got[0].Origin != diag.PhaseFreeze {
				t.Fatalf("Origin = %q, want %q", got[0].Origin, diag.PhaseFreeze)
			}
			if got[0].Pos != somewhere {
				t.Fatalf("Pos = %v, want %v", got[0].Pos, somewhere)
			}
			if want := "Store is added after Freeze"; got[0].Msg != want {
				t.Fatalf("Msg = %q, want %q", got[0].Msg, want)
			}
		})
	})

	t.Run("Warnf", func(t *testing.T) {
		t.Parallel()

		t.Run("reports at Warning without failing the run", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Warnf(diag.Code{Prefix: diag.KernelPrefix, Number: 2},
				somewhere, diag.PhaseAnnotate, "the %s directive is deprecated", "shape")

			got := collect(t, s)
			if len(got) != 1 || got[0].Severity != diag.SeverityWarning {
				t.Fatalf("All() = %+v, want one Warning", got)
			}
			if s.Failed() {
				t.Fatal("Failed() = true after a Warning, want false")
			}
		})
	})

	t.Run("Infof", func(t *testing.T) {
		t.Parallel()

		t.Run("reports at Info without failing the run", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Infof(diag.Code{Prefix: diag.KernelPrefix, Number: 3},
				somewhere, diag.PhaseLoad, "loaded %d units", 4)

			got := collect(t, s)
			if len(got) != 1 || got[0].Severity != diag.SeverityInfo {
				t.Fatalf("All() = %+v, want one Info", got)
			}
			if want := "loaded 4 units"; got[0].Msg != want {
				t.Fatalf("Msg = %q, want %q", got[0].Msg, want)
			}
			if s.Failed() {
				t.Fatal("Failed() = true after an Info, want false")
			}
		})
	})

	t.Run("Failed", func(t *testing.T) {
		t.Parallel()

		t.Run("answers false for a sink holding nothing", func(t *testing.T) {
			t.Parallel()

			if diag.NewSink().Failed() {
				t.Fatal("Failed() = true for an empty sink, want false")
			}
		})

		t.Run("answers false for warnings and infos alone", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Report(diag.Diag{Severity: diag.SeverityWarning, Pos: somewhere})
			s.Report(diag.Diag{Severity: diag.SeverityInfo, Pos: somewhere})
			if s.Failed() {
				t.Fatal("Failed() = true without an Error, want false: " +
					"a warning never fails a run")
			}
		})

		t.Run("stays true once an error lands", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Report(diag.Diag{Severity: diag.SeverityError, Pos: somewhere})
			s.Report(diag.Diag{Severity: diag.SeverityInfo, Pos: somewhere})
			if !s.Failed() {
				t.Fatal("Failed() = false after an Error, want true")
			}
		})
	})

	t.Run("All", func(t *testing.T) {
		t.Parallel()

		t.Run("answers nothing for a sink holding nothing", func(t *testing.T) {
			t.Parallel()

			if got := collect(t, diag.NewSink()); len(got) != 0 {
				t.Fatalf("All() answered %d findings for an empty sink, want 0", len(got))
			}
		})

		t.Run("keeps report order within one origin", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			want := []string{"first", "second", "third"}
			for _, msg := range want {
				s.Report(diag.Diag{Msg: msg, Pos: somewhere, Origin: diag.PhaseLoad})
			}

			var got []string
			for d := range s.All() {
				got = append(got, d.Msg)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("All() = %v, want %v", got, want)
			}
		})

		t.Run("groups the origins in one order however they interleave", func(t *testing.T) {
			t.Parallel()

			// The two sinks receive the same findings in opposite
			// interleavings, which is what two schedulings of one
			// parallel run produce.
			ordered, interleaved := diag.NewSink(), diag.NewSink()
			for _, origin := range []diag.PluginID{diag.PhaseLoad, diag.PhaseAnnotate} {
				for _, msg := range []string{"first", "second"} {
					ordered.Report(diag.Diag{Msg: msg, Pos: somewhere, Origin: origin})
				}
			}
			for _, msg := range []string{"first", "second"} {
				for _, origin := range []diag.PluginID{diag.PhaseLoad, diag.PhaseAnnotate} {
					interleaved.Report(diag.Diag{Msg: msg, Pos: somewhere, Origin: origin})
				}
			}

			spell := func(s *diag.Sink) string {
				var out []string
				for d := range s.All() {
					out = append(out, string(d.Origin)+"/"+d.Msg)
				}
				return strings.Join(out, ",")
			}
			want := "annotate/first,annotate/second,load/first,load/second"
			if got := spell(ordered); got != want {
				t.Fatalf("All() = %s, want %s", got, want)
			}
			if got := spell(interleaved); got != want {
				t.Fatalf("interleaved All() = %s, want %s: the order is not stable", got, want)
			}
		})

		t.Run("stops when the caller stops", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			for _, msg := range []string{"first", "second", "third"} {
				s.Report(diag.Diag{Msg: msg, Pos: somewhere, Origin: diag.PhaseLoad})
			}

			seen := 0
			for range s.All() {
				seen++
				break
			}
			if seen != 1 {
				t.Fatalf("the iteration answered %d findings after a break, want 1", seen)
			}
		})
	})
}

// Every layer reports through one sink, and frontends report while
// running alongside each other, so what matters is the cost under
// contention rather than the cost of one call.
func BenchmarkSink(b *testing.B) {
	code := diag.Code{Prefix: diag.KernelPrefix, Number: 1}

	b.Run("Report", func(b *testing.B) {
		b.ReportAllocs()

		s := diag.NewSink()
		d := diag.Diag{Code: code, Pos: somewhere, Origin: diag.PhaseLoad, Msg: "a finding"}
		for b.Loop() {
			s.Report(d)
		}
	})

	b.Run("Report in parallel", func(b *testing.B) {
		b.ReportAllocs()

		s := diag.NewSink()
		d := diag.Diag{Code: code, Pos: somewhere, Origin: diag.PhaseLoad, Msg: "a finding"}
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				s.Report(d)
			}
		})
	})

	b.Run("Errorf", func(b *testing.B) {
		b.ReportAllocs()

		s := diag.NewSink()
		for b.Loop() {
			s.Errorf(code, somewhere, diag.PhaseFreeze, "%s is added after Freeze", "Store")
		}
	})

	b.Run("All", func(b *testing.B) {
		b.ReportAllocs()

		const findings = 1024
		s := diag.NewSink()
		for i := range findings {
			s.Report(diag.Diag{
				Code:   code,
				Pos:    somewhere,
				Origin: diag.PluginID("plugin" + strconv.Itoa(i%8)),
				Msg:    "a finding",
			})
		}

		for b.Loop() {
			seen := 0
			for range s.All() {
				seen++
			}
			if seen != findings {
				b.Fatalf("All answered %d findings, want %d", seen, findings)
			}
		}
	})
}

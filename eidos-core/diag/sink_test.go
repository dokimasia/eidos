// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/assert"

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

	// collect returns everything a sink holds, in the order it
	// returns it.
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

			assert.Equal(t, collect(t, s), []diag.Diag{want},
				"Report attaches the finding it was given, whole")
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

			assert.Length(t, collect(t, s), reporters*each,
				"no report is lost under concurrent reporters")
			assert.True(t, s.Failed(),
				"and the errors they reported failed the run")
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

			assert.Equal(t, collect(t, s), []diag.Diag{{
				Code:     code,
				Severity: diag.SeverityError,
				Pos:      somewhere,
				Msg:      "Store is added after Freeze",
				Origin:   diag.PhaseFreeze,
			}}, "Errorf reports at Error with the message formatted")
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
			assert.Length(t, got, 1, "Warnf reports one finding")
			assert.Equal(t, got[0].Severity, diag.SeverityWarning, "at Warning")
			assert.False(t, s.Failed(), "which never fails a run")
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
			assert.Length(t, got, 1, "Infof reports one finding")
			assert.Equal(t, got[0].Severity, diag.SeverityInfo, "at Info")
			assert.Equal(t, got[0].Msg, "loaded 4 units", "with the message formatted")
			assert.False(t, s.Failed(), "and it never fails a run")
		})
	})

	t.Run("Failed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false for a sink holding nothing", func(t *testing.T) {
			t.Parallel()

			assert.False(t, diag.NewSink().Failed(),
				"a sink holding nothing has failed nothing")
		})

		t.Run("returns false for warnings and infos alone", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Report(diag.Diag{Severity: diag.SeverityWarning, Pos: somewhere})
			s.Report(diag.Diag{Severity: diag.SeverityInfo, Pos: somewhere})
			assert.False(t, s.Failed(), "a warning never fails a run")
		})

		t.Run("stays true once an error arrives", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Report(diag.Diag{Severity: diag.SeverityError, Pos: somewhere})
			s.Report(diag.Diag{Severity: diag.SeverityInfo, Pos: somewhere})
			assert.True(t, s.Failed(), "an Error fails the run whatever follows it")
		})
	})

	t.Run("All", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a sink holding nothing", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, collect(t, diag.NewSink()),
				"a sink holding nothing returns nothing")
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
			assert.Equal(t, got, want, "report order holds within one origin")
		})

		t.Run("groups the origins in one order however they interleave", func(t *testing.T) {
			t.Parallel()

			// The two sinks receive the same findings in opposite
			// interleavings, which is what two schedulings of one
			// parallel run produce.
			ordered, interleaved := diag.NewSink(), diag.NewSink()
			for _, origin := range []diag.Origin{diag.PhaseLoad, diag.PhaseAnnotate} {
				for _, msg := range []string{"first", "second"} {
					ordered.Report(diag.Diag{Msg: msg, Pos: somewhere, Origin: origin})
				}
			}
			for _, msg := range []string{"first", "second"} {
				for _, origin := range []diag.Origin{diag.PhaseLoad, diag.PhaseAnnotate} {
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
			assert.Equal(t, spell(ordered), want,
				"the origins group in one order")
			assert.Equal(t, spell(interleaved), want,
				"however their reports interleaved")
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
			assert.Equal(t, seen, 1, "the iteration stops when the caller stops")
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
				Origin: diag.Origin("plugin" + strconv.Itoa(i%8)),
				Msg:    "a finding",
			})
		}

		for b.Loop() {
			seen := 0
			for range s.All() {
				seen++
			}
			if seen != findings {
				b.Fatalf("All returned %d findings, want %d", seen, findings)
			}
		}
	})
}

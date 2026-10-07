// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/history"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
)

// warmReports is how many findings a sink receives before an allocation
// check measures the next report. Three leave room for a fourth in the
// storage the append grew, and every later growth doubles the storage,
// so growths stay fewer than the reports measured.
const warmReports = 3

// somewhere is a position for a finding whose location is beside the
// point of the case reporting it.
var somewhere = position.Pos{File: "svc/store.go", Line: 1, Col: 1}

// Every layer reports into the sink, so its order is a contract, and
// frontends report into it from goroutines of their own.
func TestSink(t *testing.T) {
	t.Parallel()

	// collect returns every finding of a sink, in the order it
	// returns them.
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
				Msg:      "Store has a deprecated directive",
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
			outcomes := history.Concurrently(reporters, time.Minute, func(reporter int) (any, error) {
				for range each {
					s.Errorf(diag.Code{Prefix: diag.KernelPrefix, Number: reporter},
						somewhere, diag.PhaseLoad, "finding from reporter %d", reporter)
				}
				return reporter, nil
			})
			for _, o := range outcomes {
				expect.True(t, o.Finished, "every reporter finishes")
			}

			assert.Length(t, collect(t, s), reporters*each,
				"no report is lost under concurrent reporters")
			assert.True(t, s.Failed(),
				"and the errors they reported failed the run")
		})

		t.Run("is safe to call while a caller reads", func(t *testing.T) {
			t.Parallel()

			const each = 64
			s := diag.NewSink()
			outcomes := history.Concurrently(2, time.Minute, func(client int) (any, error) {
				for range each {
					if client == 0 {
						s.Report(diag.Diag{Pos: somewhere, Origin: diag.PhaseLoad})
						continue
					}
					for range s.All() {
						break
					}
				}
				return client, nil
			})
			for _, o := range outcomes {
				expect.True(t, o.Finished, "the reporter and the reader finish")
			}
			assert.Length(t, collect(t, s), each, "the reader loses no report")
		})
	})

	t.Run("Errorf", func(t *testing.T) {
		t.Parallel()

		t.Run("reports at Error with the message formatted", func(t *testing.T) {
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

		t.Run("returns false for an empty sink", func(t *testing.T) {
			t.Parallel()

			assert.False(t, diag.NewSink().Failed(), "an empty sink has failed nothing")
		})

		t.Run("returns false without an error", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Report(diag.Diag{Severity: diag.SeverityWarning, Pos: somewhere})
			s.Report(diag.Diag{Severity: diag.SeverityInfo, Pos: somewhere})
			assert.False(t, s.Failed(), "a warning never fails a run")
		})

		t.Run("returns true for an error followed by an info", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Report(diag.Diag{Severity: diag.SeverityError, Pos: somewhere})
			s.Report(diag.Diag{Severity: diag.SeverityInfo, Pos: somewhere})
			assert.True(t, s.Failed(), "an Error fails the run whatever follows it")
		})
	})

	t.Run("All", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for an empty sink", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, collect(t, diag.NewSink()), "an empty sink returns no finding")
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
			assert.Equal(t, got, want, "the findings of one origin are in report order")
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

		t.Run("returns the findings of the sink when the range starts", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			all := s.All()
			s.Report(diag.Diag{Msg: "late", Pos: somewhere, Origin: diag.PhaseLoad})
			var got []string
			for d := range all {
				got = append(got, d.Msg)
				s.Report(diag.Diag{Msg: "during", Pos: somewhere, Origin: diag.PhaseLoad})
			}
			assert.Equal(t, got, []string{"late"},
				"the snapshot has the report made before the range and none made during it")
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

// A new sink allocates itself, a report allocates only to grow the
// sink's storage, and a formatted report allocates its message besides.
// The outcome reads without allocating, and an enumeration allocates
// its snapshot. The check runs alone, because the count includes every
// goroutine's allocations.
func TestSinkAllocs(t *testing.T) {
	code := diag.Code{Prefix: diag.KernelPrefix, Number: 1}
	d := diag.Diag{Code: code, Pos: somewhere, Origin: diag.PhaseLoad, Msg: "a finding"}

	var s *diag.Sink
	assert.MaxAllocs(t, func() { s = diag.NewSink() }, 1, "NewSink allocates the sink")

	s = warmSink(d)
	assert.MaxAllocs(t, func() { s.Report(d) }, 0, "Report allocates only to grow the storage")

	s = warmSink(d)
	assert.MaxAllocs(t, func() {
		s.Errorf(code, somewhere, diag.PhaseFreeze, "%s is added after Freeze", "Store")
	}, 1, "Errorf allocates its message")

	s = warmSink(d)
	assert.MaxAllocs(t, func() {
		s.Warnf(code, somewhere, diag.PhaseAnnotate, "the %s directive is deprecated", "shape")
	}, 1, "Warnf allocates its message")

	s = warmSink(d)
	assert.MaxAllocs(t, func() {
		s.Infof(code, somewhere, diag.PhaseLoad, "loaded %d units", 4)
	}, 1, "Infof allocates its message")

	s = warmSink(d)
	var failed bool
	assert.MaxAllocs(t, func() { failed = s.Failed() }, 0, "Failed allocates nothing")
	assert.True(t, failed, "Failed reports the warm sink's errors")

	s = warmSink(d)
	var seen int
	assert.MaxAllocs(t, func() {
		seen = 0
		for range s.All() {
			seen++
		}
	}, 1, "All allocates its snapshot")
	assert.Equal(t, seen, warmReports, "All returns every finding")
}

// BenchmarkSink measures a new sink, a report into the sink, a formatted
// report at each severity, a read of the outcome, and an enumeration of
// 1,024 findings of eight origins. BenchmarkSinkParallel measures
// reports from many goroutines.
func BenchmarkSink(b *testing.B) {
	code := diag.Code{Prefix: diag.KernelPrefix, Number: 1}
	d := diag.Diag{
		Code: code, Severity: diag.SeverityWarning, Pos: somewhere, Origin: diag.PhaseLoad, Msg: "a finding",
	}

	b.Run("NewSink", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got *diag.Sink
		for c.Loop() {
			got = diag.NewSink()
		}
		assert.False(b, got.Failed(), "a new sink has failed nothing")
	})

	b.Run("Report", func(b *testing.B) {
		s := warmSink(d)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			s.Report(d)
		}
		assert.False(b, s.Failed(), "a report below Error fails nothing")
	})

	b.Run("Errorf", func(b *testing.B) {
		// The harness collects garbage before this run, which empties fmt's
		// pool of printers. One formatted report before the contract counts
		// pools a printer again, and leaves room in the sink for the first
		// measured report.
		s := diag.NewSink()
		for range warmReports {
			s.Errorf(code, somewhere, diag.PhaseFreeze, "%s is added after Freeze", "Store")
		}
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		for c.Loop() {
			s.Errorf(code, somewhere, diag.PhaseFreeze, "%s is added after Freeze", "Store")
		}
		assert.True(b, s.Failed(), "an Error fails the run")
	})

	b.Run("Warnf", func(b *testing.B) {
		// One formatted report before the contract counts pools a
		// printer again, as for Errorf.
		s := diag.NewSink()
		for range warmReports {
			s.Warnf(code, somewhere, diag.PhaseAnnotate, "the %s directive is deprecated", "shape")
		}
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		for c.Loop() {
			s.Warnf(code, somewhere, diag.PhaseAnnotate, "the %s directive is deprecated", "shape")
		}
		assert.False(b, s.Failed(), "a Warning fails nothing")
	})

	b.Run("Infof", func(b *testing.B) {
		// One formatted report before the contract counts pools a
		// printer again, as for Errorf.
		s := diag.NewSink()
		for range warmReports {
			s.Infof(code, somewhere, diag.PhaseLoad, "loaded %d units", 4)
		}
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		for c.Loop() {
			s.Infof(code, somewhere, diag.PhaseLoad, "loaded %d units", 4)
		}
		assert.False(b, s.Failed(), "an Info fails nothing")
	})

	b.Run("Failed", func(b *testing.B) {
		s := warmSink(d)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := true
		for c.Loop() {
			got = s.Failed()
		}
		assert.False(b, got, "warnings fail nothing")
	})

	b.Run("All", func(b *testing.B) {
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
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range s.All() {
				seen++
			}
		}
		assert.Equal(b, seen, findings, "All returns every finding")
	})
}

// BenchmarkSinkParallel measures reports from GOMAXPROCS goroutines into
// one sink, the cost under contention, under the ceiling of a serial
// report: a report allocates only to grow the sink's storage.
func BenchmarkSinkParallel(b *testing.B) {
	d := diag.Diag{Code: diag.Code{Prefix: diag.KernelPrefix, Number: 1}, Pos: somewhere, Origin: diag.PhaseLoad}

	b.Run("Report", func(b *testing.B) {
		s := diag.NewSink()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		c.RunParallel(func(pb *bench.PB) {
			for pb.Next() {
				s.Report(d)
			}
		})
		assert.Length(b, slices.Collect(s.All()), b.N, "no report is lost under contention")
	})
}

// warmSink returns a sink that received [warmReports] copies of d.
func warmSink(d diag.Diag) *diag.Sink {
	s := diag.NewSink()
	for range warmReports {
		s.Report(d)
	}
	return s
}

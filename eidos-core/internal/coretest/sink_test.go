// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/position"
)

// The sink helpers are what cases assert findings through, so their
// own contracts are held here: report order is preserved, a clean
// run compares equal to nothing, and a helper that would pass over
// a wrong report is a case that cannot fail.
func TestSink(t *testing.T) {
	t.Parallel()

	t.Run("Codes", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the codes in report order", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			report(s, first)
			report(s, second)
			assert.Equal(t, coretest.Codes(s), []diag.Code{first, second},
				"report order is the order the codes come back in")
		})

		t.Run("returns an empty slice for a clean sink", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, coretest.Codes(diag.NewSink()), []diag.Code{},
				"a sink that collected nothing yields nothing")
		})

		t.Run("keeps a repeated code once per report", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			report(s, first)
			report(s, first)
			assert.Equal(t, coretest.Codes(s), []diag.Code{first, first},
				"two reports of one code are two entries, so a case can "+
					"hold how many times a finding fired")
		})
	})

	t.Run("AssertCodes", func(t *testing.T) {
		t.Parallel()

		t.Run("holds an exact report", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			report(s, first)
			coretest.AssertCodes(t, s, first)
		})

		t.Run("holds a clean run when passed nothing", func(t *testing.T) {
			t.Parallel()

			coretest.AssertCodes(t, diag.NewSink())
		})

		t.Run("fails on a report that does not match", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			report(s, first)
			assert.Rejects(t, "a wrong code is caught", func(tb assert.TB) {
				coretest.AssertCodes(tb, s, second)
			})
		})

		t.Run("fails on an unexpected finding", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			report(s, first)
			assert.Rejects(t, "a clean run is held", func(tb assert.TB) {
				coretest.AssertCodes(tb, s)
			})
		})
	})

	t.Run("AssertReports", func(t *testing.T) {
		t.Parallel()

		t.Run("holds one finding among several", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			report(s, first)
			report(s, second)
			coretest.AssertReports(t, s, second)
		})

		t.Run("fails when the code never fired", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			report(s, first)
			assert.Rejects(t, "a code that never fired", func(tb assert.TB) {
				coretest.AssertReports(tb, s, second)
			})
		})
	})

	t.Run("AssertPositioned", func(t *testing.T) {
		t.Parallel()

		t.Run("holds a positioned report", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			report(s, first)
			coretest.AssertPositioned(t, s)
		})

		t.Run("fails on a finding carrying no position", func(t *testing.T) {
			t.Parallel()

			s := diag.NewSink()
			s.Errorf(first, position.Pos{}, origin, "nothing can act on this")
			assert.Rejects(t, "an unpositioned finding", func(tb assert.TB) {
				coretest.AssertPositioned(tb, s)
			})
		})
	})
}

// The codes the cases report. Two are enough to tell order from
// membership, and registering them here keeps them out of the
// kernel's own numbering.
var (
	first = diag.MustRegister(diag.Prefix("CORETEST"), diag.CodeSpec{
		Number:  1,
		Meaning: "the first fixture finding",
	})
	second = diag.MustRegister(diag.Prefix("CORETEST"), diag.CodeSpec{
		Number:  2,
		Meaning: "the second fixture finding",
	})
)

// origin is who the fixture findings are reported by.
const origin diag.Origin = "coretest"

// at is the position every fixture finding carries, so a case about
// positions has one to compare against.
var at = position.Pos{File: coretest.UnitFile, Line: 1, Col: 1}

// report puts one positioned finding of code c into s.
func report(s *diag.Sink, c diag.Code) {
	s.Errorf(c, at, origin, "the fixture reports %s", c.String())
}

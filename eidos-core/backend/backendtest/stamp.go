// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"cmp"
	"slices"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/output"
)

// AssertStamped renders the fixture and checks every file against the
// output contract. Each file stamps, the frame contains the body byte
// for byte, and the frame verifies whole under the same contract with
// the derivation that the file declares.
//
// It is not part of [RunBackendSuite], because the consumer chooses the
// brand and a [Setup] has none. A satellite runs it beside the suite
// with the contract of its own binary.
//
// Every file reports on its own, so one run reports each file that the
// contract refuses. The checks of the frame skip a file that does not
// stamp or whose frame does not verify.
func AssertStamped(tb assert.TB, setup Setup, c *output.Contract) {
	tb.Helper()

	files, _ := Rendered(tb, setup)
	for _, f := range files {
		stamped, err := c.Stamp(f)
		expect.NoError(tb, err, "the rendered file stamps: "+f.Path)
		if err != nil {
			continue
		}
		expect.Contains(tb, stamped, f.Body, "the frame contains the body byte for byte: "+f.Path)

		expect.Equal(tb, f.Plugins, sortedDistinct(f.Plugins),
			"the file's derivation is distinct and sorted: "+f.Path)
		expect.Equal(tb, f.Sources, sortedDistinct(f.Sources),
			"the file's sources are distinct and sorted: "+f.Path)

		record, err := c.Verify(stamped)
		expect.NoError(tb, err, "the stamped file verifies whole: "+f.Path)
		if err != nil {
			continue
		}
		expect.Equal(tb, record.Plugins, f.Plugins,
			"the frame records the derivation the file declared: "+f.Path)
		expect.Equal(tb, record.Sources, f.Sources,
			"and the sources it derives from: "+f.Path)
	}
}

// sortedDistinct returns a sorted copy of values without duplicates, and
// nil for an empty list. A rendered file lists its plugins and its
// sources distinct and sorted. The stamp sorts both lists itself, so a
// renderer that lists them unsorted passes the stamp and fails the
// byte-identity check, far from the cause. AssertStamped compares each
// list with this copy to report the cause at the file.
func sortedDistinct[T cmp.Ordered](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	out := slices.Clone(values)
	slices.Sort(out)
	return slices.Compact(out)
}

// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"slices"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// AssertStamped renders the fixture and takes every file through
// the output contract: each one stamps, the body survives byte
// for byte inside the frame, and the frame verifies whole under
// the same contract, with the derivation the file declared.
//
// It is not part of [RunBackendSuite], because a brand is the
// consumer's to state and the suite's [Setup] states none. A
// satellite runs this beside the suite with the contract its own
// binary ships.
//
// Every file reports on its own, so one run names each file the frame
// refuses. A file that does not stamp, or whose frame does not verify,
// skips the checks that read the frame.
func AssertStamped(tb assert.TB, setup Setup, c *output.Contract) {
	tb.Helper()

	files, _ := runRender(tb, setup)
	for _, f := range files {
		stamped, err := c.Stamp(f)
		expect.NoError(tb, err, "the rendered file stamps: "+f.Path)
		if err != nil {
			continue
		}
		expect.Contains(tb, stamped, f.Body, "the frame contains the body byte for byte: "+f.Path)

		expect.Equal(tb, f.Plugins, sortedIDs(f.Plugins),
			"the file's derivation is distinct and sorted: "+f.Path)
		expect.Equal(tb, f.Sources, sortedStrings(f.Sources),
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

// sortedIDs and sortedStrings return what a rendered file's
// derivation is documented to be: distinct and sorted. The stamp
// sorts on the way past, so without this comparison a renderer
// computing its derivation ad hoc would pass here and fail the
// byte-identity check instead, where the cause is further away.
func sortedIDs(ids []plugin.ID) []plugin.ID {
	if len(ids) == 0 {
		return nil
	}
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

func sortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := slices.Clone(values)
	slices.Sort(out)
	return slices.Compact(out)
}

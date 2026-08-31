// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backendtest

import (
	"bytes"
	"slices"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// AssertStamped renders the fixture and takes every file through
// the output contract: each one stamps, the body survives byte
// for byte inside the frame, and the frame verifies whole under
// the same contract, carrying the derivation the file declared.
//
// It is not part of [RunBackendSuite], because a brand is the
// consumer's to state and the suite's [Setup] carries none. A
// satellite runs this beside the suite with the contract its own
// binary ships.
func AssertStamped(tb assert.TB, setup Setup, c *output.Contract) {
	tb.Helper()

	files, _ := runRender(tb, setup)
	for _, f := range files {
		stamped, err := c.Stamp(f)
		assert.NoError(tb, err, "the rendered file stamps: "+f.Name)
		if err != nil {
			continue
		}
		assert.True(tb, bytes.Contains(stamped, f.Body),
			"the frame carries the body byte for byte: "+f.Name)

		assert.Equal(tb, f.Plugins, sortedIDs(f.Plugins),
			"the file's derivation is distinct and sorted: "+f.Name)
		assert.Equal(tb, f.Sources, sortedStrings(f.Sources),
			"the file's sources are distinct and sorted: "+f.Name)

		record, err := c.Verify(stamped)
		assert.NoError(tb, err, "the stamped file verifies whole: "+f.Name)
		if err != nil {
			continue
		}
		assert.Equal(tb, record.Plugins, f.Plugins,
			"the frame carries the derivation the file declared: "+f.Name)
		assert.Equal(tb, record.Sources, f.Sources,
			"and the sources it derives from: "+f.Name)
	}
}

// sortedIDs and sortedStrings answer what a rendered file's
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

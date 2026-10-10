// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/workspace"
)

// A code is API: consumers script against it and the published index
// anchors to it, so its number and meaning are contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	codes := []struct {
		name    string
		code    diag.Code
		spelt   string
		meaning string
	}{
		{
			name: "DriftedOutput", code: workspace.DriftedOutput, spelt: "EID-0055",
			meaning: "a generated file was edited since its stamp, and the run does not overwrite it",
		},
		{
			name: "ForeignFile", code: workspace.ForeignFile, spelt: "EID-0056",
			meaning: "a generated path contains a file the brand did not write",
		},
		{
			name: "PlanCollision", code: workspace.PlanCollision, spelt: "EID-0057",
			meaning: "two plans route a file to one path",
		},
		{
			name: "KeptOutput", code: workspace.KeptOutput, spelt: "EID-0058",
			meaning: "a stale output remains, because it was edited or lost its frame",
		},
		{
			name: "UnreadableRecord", code: workspace.UnreadableRecord, spelt: "EID-0059",
			meaning: "the previous record does not read, and the run removes nothing",
		},
		{
			name: "UnmetContract", code: workspace.UnmetContract, spelt: "EID-0060",
			meaning: "a declaration lacks a fact its key's completeness contract promises",
		},
		{
			name: "FailedDependency", code: workspace.FailedDependency, spelt: "EID-0061",
			meaning: "a plan or a check reads a plan that failed, and generates or checks nothing",
		},
		{
			name: "ColdState", code: workspace.ColdState, spelt: "EID-0062",
			meaning: "the run ignored the sealed state and ran cold",
		},
		{
			name: "StateLocked", code: workspace.StateLocked, spelt: "EID-0063",
			meaning: "another holder has the lock of the state directory, and the run writes nothing",
		},
		{
			name: "OutOfDate", code: workspace.OutOfDate, spelt: "EID-0064",
			meaning: "the committed output differs from what the run generates",
		},
		{
			name: "UnusedSuppression", code: workspace.UnusedSuppression, spelt: "EID-0065",
			meaning: "a diag directive removed no finding",
		},
	}
	for _, tt := range codes {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("spells the kernel prefix with its padded number", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.code.String(), tt.spelt, "the spelling is pinned")
			})

			t.Run("registers its meaning in the kernel registry", func(t *testing.T) {
				t.Parallel()

				meaning, held := diag.Kernel().Meaning(tt.code)
				assert.True(t, held, "the code registered at initialization")
				assert.Equal(t, meaning, tt.meaning, "the meaning is pinned")
			})
		})
	}
}

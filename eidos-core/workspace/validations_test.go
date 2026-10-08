// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
)

// The mirrors that the marker emits for the marked structs of the warm
// tree.
const (
	rowMirror = "ForRow"
	colMirror = "ForCol"
)

// A warm run routes each subject by the instances that the run validated
// for it, or else by the instances that the generation recorded.
func TestValidations(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		before := warmTree(rowLine, markedLine, readerLine, colLine)

		t.Run("routes a subject that the run did not validate again by its recorded instances", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}))
			report, err := warmAfter(t, w, before, warmTree(rowLine, markedLine, readerLine, widerCol))
			assert.NoError(t, err, "the run is clean")
			assert.Contains(t, mirrorsIn(report), rowMirror, "the marker mirrors the row")
		})

		t.Run("routes a subject that the run validated again by its new instances", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}))
			report, err := warmAfter(t, w, before, warmTree(rowLine, markedLine, readerLine, colLine, markedLine))
			assert.NoError(t, err, "the run is clean")
			assert.Contains(t, mirrorsIn(report), colMirror, "the marker mirrors Col")
		})

		t.Run("routes no instance of a subject whose directive the edit removed", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}))
			report, err := warmAfter(t, w, before, warmTree(rowLine, readerLine, colLine))
			assert.NoError(t, err, "the run is clean")
			assert.NotContains(t, mirrorsIn(report), rowMirror, "the marker does not mirror the row")
		})
	})
}

// mirrorsIn returns the names of the structs that the plan of a run
// emitted.
func mirrorsIn(report *workspace.Report) []string {
	var out []string
	for _, u := range units(report.Emits["plan"]) {
		for _, d := range u.Decls {
			if s, is := d.(*emit.Struct); is {
				out = append(out, s.Name)
			}
		}
	}
	return out
}

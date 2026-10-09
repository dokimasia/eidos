// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"os"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The detectors that the wiring of a fixture refers to.
const (
	probeWriter = "detectors.ProbeWriter"
	probeReader = "detectors.ProbeReader"
	writerFunc  = "detectors.Writer"
	readerFunc  = "detectors.Reader"
	detected    = "detected: true\n"
)

// The generator orders the detected shapes by their precedence, and
// reports the faults of the yields_to lists.
func TestModel(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("orders a shape after the shapes that it yields to", func(t *testing.T) {
			t.Parallel()

			files, findings := generated(t, os.DirFS(fixtureRoot))
			assert.Empty(t, findings, "the fixture catalog is valid")
			assert.ContainsInOrder(t, string(files[wiringFile]), []string{probeWriter, probeReader},
				"the reader yields to the writer, so the writer comes first")
		})

		t.Run("orders two shapes without a precedence by their names", func(t *testing.T) {
			t.Parallel()

			files, findings := generated(t, fstest.MapFS{
				writerPath: {Data: []byte("name: writer\n" + shapeHead + detected)},
				readerPath: {Data: []byte("name: reader\n" + shapeHead + detected)},
			})
			assert.Empty(t, findings, "the specs are valid")
			assert.ContainsInOrder(t, string(files[wiringFile]), []string{readerFunc, writerFunc},
				"the reader comes before the writer by its name")
		})

		t.Run("reports a yields_to entry that is no detected shape", func(t *testing.T) {
			t.Parallel()

			_, findings := generated(t, fstest.MapFS{
				readerPath: {
					Data: []byte("name: reader\n" + shapeHead + detected + "precedence:\n  yields_to: [declared]\n"),
				},
				declaredPath: {Data: []byte("name: declared\n" + shapeHead)},
			})
			expectFindings(t, findings, []diag.Code{specfront.SpecInvalid},
				[]string{"the shape reader yields to declared, which is no detected shape"})
		})

		t.Run("reports a yields_to entry that no spec has", func(t *testing.T) {
			t.Parallel()

			_, findings := generated(t, fstest.MapFS{
				readerPath: {
					Data: []byte("name: reader\n" + shapeHead + detected + "precedence:\n  yields_to: [ghost]\n"),
				},
			})
			expectFindings(t, findings, []diag.Code{specfront.SpecInvalid},
				[]string{"the shape reader yields to ghost, which is no detected shape"})
		})

		t.Run("reports a cycle at each shape of the cycle", func(t *testing.T) {
			t.Parallel()

			_, findings := generated(t, fstest.MapFS{
				readerPath: {
					Data: []byte("name: reader\n" + shapeHead + detected + "precedence:\n  yields_to: [writer]\n"),
				},
				writerPath: {
					Data: []byte("name: writer\n" + shapeHead + detected + "precedence:\n  yields_to: [reader]\n"),
				},
			})
			expectFindings(t, findings, []diag.Code{specfront.PrecedenceCycle, specfront.PrecedenceCycle}, []string{
				"the yields_to lists form a cycle through reader, so no order ranks it",
				"the yields_to lists form a cycle through writer, so no order ranks it",
			})
		})

		t.Run("reads the specs in the order of their names", func(t *testing.T) {
			t.Parallel()

			want := []string{"Declared", "ProbeReader", "ProbeWriter", "TxProbe", "XSSSafe"}
			assert.Equal(t, keptOf(declaredNames(t, registryOf(t)), want), want,
				"the registry declares the specs in name order, whatever their forms")
		})
	})
}

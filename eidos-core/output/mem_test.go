// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
)

// The memory sink is what a dry run and a test write through, so
// it has to keep the staging invisible exactly as the disk sink
// does: nothing is readable until the commit.
func TestMem(t *testing.T) {
	t.Parallel()

	t.Run("keeps the staging invisible until the commit", func(t *testing.T) {
		t.Parallel()

		m := output.NewMem()
		assert.NoError(t, m.Write("svc/store.go", []byte("package svc\n")),
			"the staging takes")
		assert.Length(t, m.Files(), 0,
			"staged means invisible, in memory as on disk")

		_, err := m.Commit()
		assert.NoError(t, err, "the commit runs")
		assert.Equal(t, string(m.Files()["svc/store.go"]), "package svc\n",
			"and the committed bytes read back")
	})

	t.Run("a discarded sink leaves nothing", func(t *testing.T) {
		t.Parallel()

		m := output.NewMem()
		assert.NoError(t, m.Write("svc/store.go", []byte("package svc\n")),
			"the staging takes")
		assert.NoError(t, m.Discard(), "the discard runs")
		assert.Length(t, m.Files(), 0, "a discarded sink held nothing")
	})

	t.Run("hands out a copy of what it holds", func(t *testing.T) {
		t.Parallel()

		m := output.NewMem()
		assert.NoError(t, m.Write("a.go", []byte("package a\n")), "the staging takes")
		_, err := m.Commit()
		assert.NoError(t, err, "the commit runs")

		files := m.Files()
		delete(files, "a.go")
		files["b.go"] = []byte("package b\n")
		assert.Length(t, m.Files(), 1,
			"a caller editing the map it was handed edits nothing here")
		assert.Equal(t, string(m.Files()["a.go"]), "package a\n", "the file stands")
	})
}

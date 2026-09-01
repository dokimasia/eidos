// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
)

// refusing is a sink that takes nothing, for the fan-out's error
// path.
type refusing struct{ err error }

func (r refusing) Write(string, []byte) error        { return r.err }
func (r refusing) Commit() ([]output.Written, error) { return nil, r.err }
func (r refusing) Discard() error                    { return r.err }

// The fan-out sink writes one staging to several destinations, so
// what it owes is that every one of them sees every call, and that
// no failure is swallowed on the way.
func TestTee(t *testing.T) {
	t.Parallel()

	t.Run("stages into every sink", func(t *testing.T) {
		t.Parallel()

		first, second := output.NewMem(), output.NewMem()
		tee := output.NewTee(first, second)
		assert.NoError(t, tee.Write("store.go", []byte("package svc\n")),
			"the staging takes")
		got, err := tee.Commit()
		assert.NoError(t, err, "the commit runs")

		assert.Equal(t, string(first.Files()["store.go"]), "package svc\n",
			"the first sink holds the bytes")
		assert.Equal(t, string(second.Files()["store.go"]), "package svc\n",
			"and so does the second")
		assert.Length(t, got, 1, "one record per staged file")
		assert.Equal(t, got[0].Path, "store.go",
			"read off the first sink, which is the one of record")
	})

	t.Run("joins what any sink refused", func(t *testing.T) {
		t.Parallel()

		refused := errors.New("nowhere to write")
		tee := output.NewTee(output.NewMem(), refusing{err: refused})

		assert.ErrorIs(t, tee.Write("store.go", []byte("package svc\n")), refused,
			"a staging that failed anywhere failed")
		_, err := tee.Commit()
		assert.ErrorIs(t, err, refused, "and so does a commit")
		assert.ErrorIs(t, tee.Discard(), refused, "and a discard")
	})

	t.Run("discards every sink", func(t *testing.T) {
		t.Parallel()

		first, second := output.NewMem(), output.NewMem()
		tee := output.NewTee(first, second)
		assert.NoError(t, tee.Write("store.go", []byte("package svc\n")),
			"the staging takes")
		assert.NoError(t, tee.Discard(), "the discard runs")
		assert.Length(t, first.Files(), 0, "the first sink held nothing")
		assert.Length(t, second.Files(), 0, "and neither did the second")
	})
}

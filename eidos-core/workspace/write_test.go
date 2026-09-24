// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// partialSink stages every write and commits the first staged file
// before failing: the shape of a disk commit refused part-way
// through a tree.
type partialSink struct{ staged []string }

func (s *partialSink) Write(path string, _ []byte) error {
	s.staged = append(s.staged, path)
	return nil
}

func (s *partialSink) Commit() ([]output.Written, error) {
	return []output.Written{{Path: s.staged[0], Action: output.ActionCreated}},
		errors.New("disk full")
}

func (*partialSink) Discard() error { return nil }

// The write is the run's last step: the render's staged files go
// to the sink, and the report records what reached the
// destination.
func TestWrite(t *testing.T) {
	t.Parallel()

	t.Run("a commit refused part-way reports the files it committed", func(t *testing.T) {
		t.Parallel()

		w, err := workspace.New().
			Annotators(stamper("noter", quiet)).
			Targets("fixture").
			Plans(workspace.Plan{
				Name:       "plan",
				Generators: []plugin.Generator{mirror("mirror")},
				Backend:    printer(t, "fixture"),
			}).
			Output(&partialSink{}, "eidos").
			Build()
		assert.NoError(t, err, "the writing composition composes")

		g, _ := alpha(t)
		run, err := w.Run(t.Context(), g)
		assert.HasError(t, err, "the refused commit fails the run")
		assert.Contains(t, err.Error(), "disk full", "carrying the sink's cause")
		assert.Length(t, run.Written, 1,
			"and the report records the file that reached its destination")
	})
}

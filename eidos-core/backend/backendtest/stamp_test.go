// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// AssertStamped stamps and verifies each file a backend renders. A
// backend can pass the render's checks and the contract's checks one at
// a time and still fail here: a body the frame refuses, or a derivation
// the frame cannot record, shows only when both run over one file.
func TestAssertStamped(t *testing.T) {
	t.Parallel()

	t.Run("accepts a backend whose files stamp and verify", func(t *testing.T) {
		t.Parallel()

		backendtest.AssertStamped(t, wellRendered, fixtureContract(t))
	})

	t.Run("rejects a body without a trailing newline", func(t *testing.T) {
		t.Parallel()

		unterminated := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return []plugin.RenderedFile{{Path: "a.txt", Body: []byte("no newline")}}, nil
		})

		failure := assert.Rejects(t, "a body without a trailing newline must fail",
			func(tb assert.TB) {
				backendtest.AssertStamped(tb, unterminated, fixtureContract(tb))
			})
		assert.Contains(t, failure, "stamps",
			"the check names the step that refused")
	})

	t.Run("rejects a derivation the file did not sort", func(t *testing.T) {
		t.Parallel()

		unsorted := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return []plugin.RenderedFile{{
				Path:    "a.txt",
				Plugins: []plugin.ID{"stubgen", "acme-audit"},
				Body:    []byte("package a\n"),
			}}, nil
		})

		failure := assert.Rejects(t, "a rendered file lists its emitters in sorted order",
			func(tb assert.TB) {
				backendtest.AssertStamped(tb, unsorted, fixtureContract(tb))
			})
		assert.Contains(t, failure, "derivation",
			"the check names the frame's derivation")
	})

	t.Run("rejects sources the file did not sort", func(t *testing.T) {
		t.Parallel()

		unsorted := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return []plugin.RenderedFile{{
				Path:    "a.txt",
				Sources: []string{"b.src", "a.src"},
				Body:    []byte("package a\n"),
			}}, nil
		})

		failure := assert.Rejects(t, "a rendered file lists its sources in sorted order",
			func(tb assert.TB) {
				backendtest.AssertStamped(tb, unsorted, fixtureContract(tb))
			})
		assert.Contains(t, failure, "sources",
			"the check names what the frame derives from")
		assert.Contains(t, failure, "distinct and sorted",
			"and refuses them before the frame reorders them, so the "+
				"failure is not the file's missing emitter")
	})

	t.Run("reports every file the frame refuses", func(t *testing.T) {
		t.Parallel()

		faulty := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return []plugin.RenderedFile{
				{Path: "a.txt", Body: []byte("no newline")},
				{Path: "b.txt", Body: []byte("a carriage\r\n")},
			}, nil
		})

		rec := assert.NewRecorder()
		backendtest.AssertStamped(rec, faulty, fixtureContract(t))
		assert.Equal(t, len(rec.Failures()), 2,
			"a file the frame refuses is reported and the next one is still read")
		for i, name := range []string{"a.txt", "b.txt"} {
			assert.Contains(t, rec.Failures()[i].Contract, name,
				"each refusal names the file it read")
		}
	})
}

// fixtureContract composes the contract a satellite would ship
// beside its backend: its own brand and its language's comments.
func fixtureContract(tb assert.TB) *output.Contract {
	tb.Helper()

	c, err := output.NewContract("acme", plugin.CommentSyntax{Line: []string{"//"}})
	assert.NoError(tb, err, "the fixture contract composes")
	return c
}

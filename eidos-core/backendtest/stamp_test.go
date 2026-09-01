// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backendtest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
)

// fixtureContract composes the contract a satellite would ship
// beside its backend: its own brand and its language's comments.
func fixtureContract(tb assert.TB) *output.Contract {
	tb.Helper()

	c, err := output.NewContract("acme", plugin.CommentSyntax{Line: []string{"//"}})
	assert.NoError(tb, err, "the fixture contract composes")
	return c
}

// Stamping is where the render and the output contract meet, and
// a backend can satisfy each half alone and still fail the join:
// a body the frame refuses, or a derivation the frame cannot
// carry, only shows up when both run over one file.
func TestAssertStamped(t *testing.T) {
	t.Parallel()

	t.Run("accepts a backend whose files stamp and verify", func(t *testing.T) {
		t.Parallel()

		backendtest.AssertStamped(t, wellRendered, fixtureContract(t))
	})

	t.Run("rejects a body the frame cannot carry", func(t *testing.T) {
		t.Parallel()

		unterminated := scripted(func(*plugin.RenderContext) ([]plugin.RenderedFile, error) {
			return []plugin.RenderedFile{{Name: "a.txt", Body: []byte("no newline")}}, nil
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
				Name:    "a.txt",
				Plugins: []plugin.ID{"stubgen", "acme-audit"},
				Body:    []byte("package a\n"),
			}}, nil
		})

		failure := assert.Rejects(t, "a rendered file carries its emitters sorted",
			func(tb assert.TB) {
				backendtest.AssertStamped(tb, unsorted, fixtureContract(tb))
			})
		assert.Contains(t, failure, "derivation",
			"the check names what the frame carries")
	})
}

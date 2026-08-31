// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backendtest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	java "go.dokimi.dev/eidos/lang-java"
	"go.dokimi.dev/eidos/lang-java/backend"
)

// setup builds the backend over the kernel's canonical fixture,
// filtered to this module's declared inventory: the class and
// interface kinds, with every body content form arriving through
// the class template's members.
func setup(tb assert.TB) (plugin.Renderer, *backendtest.Fixture) {
	tb.Helper()

	r, held := backend.New().(plugin.Renderer)
	assert.True(tb, held, "the built backend renders")
	return r, backendtest.CanonicalFixture(tb, backend.KindTemplates())
}

// The backend is the module's write half: the kernel suite holds
// it to the render checks over the canonical fixture, and the
// stamp check joins it to the output contract under this module's
// own comment forms.
func TestNew(t *testing.T) {
	t.Parallel()

	backendtest.RunBackendSuite(t, setup)

	t.Run("declares its identity and target", func(t *testing.T) {
		t.Parallel()

		b := backend.New()
		assert.Equal(t, b.Name(), java.Name, "the plugin identity")
		assert.Equal(t, b.Target(), java.Target, "the rendering target")
	})

	t.Run("stamps under the module's contract", func(t *testing.T) {
		t.Parallel()

		c, err := output.NewContract("java", java.Syntax())
		assert.NoError(t, err, "the module contract composes")
		backendtest.AssertStamped(t, setup, c)
	})
}

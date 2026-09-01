// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontendtest"
	"go.dokimi.dev/eidos/core/internal/fakelang"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The checks exist to catch broken frontends, so the broken ones
// are simulated and each check's own failure is asserted.
func TestChecks(t *testing.T) {
	t.Parallel()

	t.Run("AssertLinked rejects a Resolve that never answers", func(t *testing.T) {
		t.Parallel()

		msg := assert.Rejects(t, "a mute resolver over a two-package fixture", func(tb assert.TB) {
			frontendtest.AssertLinked(tb, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return &mute{fakelang.New()}, fixture()
			})
		})
		assert.Contains(t, msg, "resolving nothing", "the rejection says what never happened")
	})

	t.Run("AssertClassified rejects a silently dropped file", func(t *testing.T) {
		t.Parallel()

		msg := assert.Rejects(t, "a parse that swallows a file", func(tb assert.TB) {
			frontendtest.AssertClassified(tb, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return &swallower{fakelang.New()}, fixture()
			})
		})
		assert.Contains(t, msg, "silently dropped", "the rejection names the class")
	})
}

// mute resolves nothing, which a two-package fixture must expose.
type mute struct {
	*fakelang.Frontend
}

// Resolve answers no candidate for any spelling.
func (*mute) Resolve(plugin.ImportScope, string) []symbol.Identity { return nil }

// swallower parses every unit's first member alone and says
// nothing about the rest, which the no-drop check must expose.
type swallower struct {
	*fakelang.Frontend
}

// Parse hands only the first member to the real lowering, leaving
// the rest undeclared and unreported.
func (s *swallower) Parse(_ context.Context, u *plugin.SourceUnit) error {
	return s.ParseFile(u, u.Files()[0].Path)
}

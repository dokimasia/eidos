// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// policy returns the TypeScript target's policy with the choices that
// chosen selects.
func policy(tb testing.TB, chosen map[plugin.PolicyKey]plugin.Choice) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(typescript.Target, typescript.Policies(), chosen)
	assert.NoError(tb, err, "the TypeScript policies resolve")
	return p
}

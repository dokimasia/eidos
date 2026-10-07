// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/sdk/toolchain"
)

// AssertVets runs go vet over the generated output under ctx, the
// Go-only addition to the kernel's set. A refusal records a failure
// whose detail states go vet's own error.
//
// Vet finds what compiles and is still wrong: a format verb that
// does not match its argument, a lock copied by value, an
// unreachable return. That is the class of defect a generator
// produces most easily, because a template that assembles a call
// site correctly for one type assembles it wrongly for the next.
func AssertVets(ctx context.Context, tb assert.TB, a toolchain.Adapter, g toolchain.Generated) {
	tb.Helper()

	dir, done := toolchain.Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	_, err := run(ctx, dir, "vet", allPackages)
	expect.NoError(tb, err, string(a.Lang())+": go vet accepts the generated output")
}

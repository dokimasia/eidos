// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import "go.dokimi.dev/eidos/sdk/toolchain"

// AssertVets holds the generated output to go vet, the Go-only
// addition to the kernel's set.
//
// Vet finds what compiles and is still wrong: a format verb that
// does not match its argument, a lock copied by value, an
// unreachable return. That is the class of defect a generator
// produces most easily, because a template that assembles a call
// site correctly for one type assembles it wrongly for the next.
func AssertVets(tb toolchain.TB, a toolchain.Adapter, g toolchain.Generated) {
	tb.Helper()

	dir, done := toolchain.Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	if _, err := run(dir, "vet", allPackages); err != nil {
		tb.Errorf("%s: go vet refused the generated output: %v", a.Lang(), err)
	}
}

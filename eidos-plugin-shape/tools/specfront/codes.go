// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import "go.dokimi.dev/eidos/sdk/diag"

// Prefix is the prefix of the codes that the spec frontend and the
// registry generator report under.
const Prefix diag.Prefix = "SHAPESPEC"

var (
	// SpecInvalid reports a spec that does not decode, or that breaks a
	// rule of its form, at the key or the value at fault.
	SpecInvalid = diag.MustRegister(Prefix, diag.CodeSpec{
		Number:  1,
		Meaning: "a spec does not decode, or breaks a rule of its form",
	})
	// SpecDuplicate reports two specs with one name, and a Go identifier
	// that two specs or one spec give twice. A finding is at each spec of
	// the duplicate.
	SpecDuplicate = diag.MustRegister(Prefix, diag.CodeSpec{
		Number:  2,
		Meaning: "two specs have one name, or the specs give one Go identifier twice",
	})
	// PrecedenceCycle reports detected shapes whose yields_to lists form
	// a cycle, at each spec of the cycle.
	PrecedenceCycle = diag.MustRegister(Prefix, diag.CodeSpec{
		Number:  3,
		Meaning: "the yields_to lists of detected shapes form a cycle",
	})
)

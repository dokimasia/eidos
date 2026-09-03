// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import "go.dokimi.dev/eidos/core/diag"

// AbsentRules reports a handler asking the rules of a language the
// composition registered none for. The refusal is a value, so the
// run continues; the Warning is what keeps the omission from
// passing as a language with no members.
var AbsentRules = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  39,
	Meaning: "a plugin asked the rules of a language the composition registers none for",
})

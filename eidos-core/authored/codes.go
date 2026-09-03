// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored

import "go.dokimi.dev/eidos/core/diag"

// UnknownWitnessParam refuses a witness key naming a type parameter
// the declaration does not declare, positioned at the carrier.
var UnknownWitnessParam = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 41, Meaning: "a witness names a type parameter its declaration does not declare",
})

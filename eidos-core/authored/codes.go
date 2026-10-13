// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored

import "go.dokimi.dev/eidos/core/diag"

// UnknownWitnessParam refuses a witness key naming a type parameter
// the declaration does not declare, positioned at the carrier.
var UnknownWitnessParam = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 41, Meaning: "a witness names a type parameter its declaration does not declare",
})

// EmptySample refuses a sample instance that states neither a value
// nor an alternate, positioned at the carrier.
var EmptySample = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 76, Meaning: "a sample states neither a value nor an alternate",
})

// EmptyWitness refuses a witness instance that names no type
// parameter, positioned at the carrier.
var EmptyWitness = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 77, Meaning: "a witness names no type parameter",
})

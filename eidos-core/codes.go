// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
)

// RefusedStamp is [meta.RefusedStamp], re-exported where annotator
// authors read. It reports a refused stamp at the subject's
// position under the stamping plugin's identity, and the phase
// continues. The code is registered in core/meta beside the fact
// store, because the classification path reports it too.
var RefusedStamp = meta.RefusedStamp

// RefusedType reports a type of another language without a spelling in
// the plan's target. The finding is at the declaration with the type.
// [Emitter.Type] reports it as an Error, so the plan fails.
var RefusedType = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  69,
	Meaning: "the plan's target cannot spell a type of another language",
})

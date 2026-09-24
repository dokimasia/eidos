// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import "go.dokimi.dev/eidos/core/diag"

// RefusedStamp reports a refused stamp. The fact store refuses a
// write naming an unregistered key, a subject kind the key does not
// admit, a false boolean, a second value from one rank source, or,
// on the raw path, a value outside the vocabulary or of another type
// than the key's. The stamping surface refuses a write to a
// declaration its subject does not declare. The witness handler
// refuses a stamp whose type argument did not resolve. The refusal
// arrives at the subject's position under the stamping plugin's
// identity, and the phase continues.
var RefusedStamp = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  20,
	Meaning: "a stamp was refused",
})

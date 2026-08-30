// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import "go.dokimi.dev/eidos/core/diag"

// RefusedStamp reports a stamp the fact store refused: the write
// named an unregistered key, a subject kind the key does not admit,
// a false boolean, or a second value from one rank source. The
// refusal lands at the subject's position under the stamping
// plugin's identity, and the phase continues.
var RefusedStamp = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  20,
	Meaning: "the fact store refused a stamp",
})

// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registrygen

import "go.dokimi.dev/eidos/sdk/diag"

// CodePrefix is the prefix of the codes that registrygen reports under.
const CodePrefix diag.Prefix = "REGISTRYGEN"

// Generic reports a struct under the directive register that has type
// parameters. The registry passes the zero value of each struct, and a
// generic struct has none without its type arguments. The finding is an
// Error at the struct, and registrygen records nothing for it.
var Generic = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a registry cannot make a value of a struct with type parameters",
})

// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stubgen

import "go.dokimi.dev/eidos/sdk/diag"

// CodePrefix is the prefix of the codes that stubgen reports under.
const CodePrefix diag.Prefix = "STUBGEN"

// GenericMethod reports an interface under the directive stub with a method
// that has type parameters. The call record of the method is outside the
// scope of those parameters, so it cannot state the types of the arguments.
// The finding is an Error at the interface, and stubgen does not write a
// stub of it.
var GenericMethod = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a call record cannot state the arguments of a method with type parameters",
})

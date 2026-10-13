// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package builder

import "go.dokimi.dev/eidos/sdk/diag"

// CodePrefix is the prefix of the codes that builder reports under.
const CodePrefix diag.Prefix = "BUILDER"

// BadDefault reports a field of a struct under the directive builder whose
// declared default is no value of the type of the field. Such a default is
// neither a literal nor a name or a call that resolves in the scope of the
// struct. The constructor of the builder cannot assign it, so the finding
// is an Error at the field, and builder writes nothing for the struct.
var BadDefault = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "the declared default of a field is no value of the type of the field",
})

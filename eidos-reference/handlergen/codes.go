// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package handlergen

import "go.dokimi.dev/eidos/sdk/diag"

// CodePrefix is the prefix of the codes that handlergen reports under.
const CodePrefix diag.Prefix = "HANDLERGEN"

// MissingServe reports a struct under the directive handler that declares
// no method Serve with two parameters. The handler calls Serve, so
// handlergen stamps no fact on the struct and writes no handler for it.
// The finding is an Error at the struct.
var MissingServe = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a struct under the directive handler declares no method Serve for its handler to call",
})

// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package callee

import "go.dokimi.dev/eidos/sdk/diag"

// CodePrefix is the prefix of the codes that the plugins of the ensemble
// share.
const CodePrefix diag.Prefix = "REFERENCE"

// Missing reports a plugin whose options leave the package or the function
// of its call empty. The finding is an Error at the subject of the rule,
// and the plugin appends nothing for it.
var Missing = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "the options of a plugin set no function for the plugin to call",
})

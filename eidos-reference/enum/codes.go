// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package enum

import "go.dokimi.dev/eidos/sdk/diag"

// CodePrefix is the prefix of the codes that enum reports under.
const CodePrefix diag.Prefix = "ENUM"

// DuplicateText reports a variant of an enumeration under the directive
// enum with the text of an earlier variant. The parser maps a text to one
// variant, so the finding is an Error at the second variant, and enum
// writes nothing for the enumeration.
var DuplicateText = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "two variants of an enumeration have one text",
})

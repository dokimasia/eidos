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

// NoProjection reports an enumeration under the directive enum whose
// language has rules that do not project an enumeration. The finding is an
// Error at the enumeration, and enum writes nothing for it.
var NoProjection = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "the rules of the language of an enumeration do not project it",
})

// UnknownText reports a variant of an enumeration of text whose value the
// frontend did not evaluate, such as a value that another package decides.
// The finding is an Error at the variant, and enum writes nothing for the
// enumeration.
var UnknownText = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  3,
	Meaning: "a variant of an enumeration of text has no text",
})

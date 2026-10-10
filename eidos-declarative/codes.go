// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import "go.dokimi.dev/eidos/core/diag"

// CodePrefix is the prefix of the codes that this package registers.
const CodePrefix diag.Prefix = "DECLARATIVE"

// IncompleteMembers reports a subject whose members the rules of its
// language cannot list in full, such as an interface that embeds an
// interface that the load did not resolve. The finding is an Error at
// the subject, and the plugin does not emit a declaration for the
// subject.
var IncompleteMembers = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  1,
	Meaning: "a declarative plugin cannot mirror a declaration whose members are incomplete",
})

// UndelegatedProperty reports an interface with a property under a rule
// that emits implementations. Each method of an implementation delegates
// its call to the field next, and a property has no call to delegate. The
// finding is an Error at the subject, and the plugin does not emit an
// implementation of the subject.
var UndelegatedProperty = diag.MustRegister(CodePrefix, diag.CodeSpec{
	Number:  2,
	Meaning: "an implementation cannot delegate a property of its interface",
})

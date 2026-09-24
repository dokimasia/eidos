// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import "go.dokimi.dev/eidos/core/diag"

// UnspeltKind reports a declaration whose kind the target language
// holds no template for: the declaration is skipped and the file
// renders without it.
var UnspeltKind = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 21, Meaning: "a declaration's kind has no template in the target language",
})

// UnformattedFile reports a rendered file the language formatter
// refused: the file is withheld, because the sink never receives
// an unformatted file, and the pass continues with its siblings.
var UnformattedFile = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 22, Meaning: "the language formatter refused a rendered file",
})

// RefusedTemplate reports a template that exists and still failed
// at execute time, naming the emitting plugin: the declaration is
// skipped and the file renders without it.
var RefusedTemplate = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 23, Meaning: "a template refused a declaration at execute time",
})

// UnspeltValue reports a scaffold value the target language has no
// form for: a raw literal written in another language, a
// conversion where the language has none. The declaration is
// skipped and the file renders without it, the way any refused
// spelling is.
var UnspeltValue = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 42, Meaning: "a scaffold value has no spelling in the target language",
})

// BodyConflict reports a body holding more than one content form:
// the standard and named slots still render, and no contested
// content is guessed at.
var BodyConflict = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 24, Meaning: "a body holds more than one content form",
})

// UnresolvedRef reports a template reference nothing returns: no
// tree declared for the emitting plugin, no template of that name
// in it, or a template that does not parse. The body falls back to
// its slots, so the extension points survive the broken claim.
var UnresolvedRef = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 25, Meaning: "a template reference resolves to nothing in its emitting plugin's tree",
})

// UndeclaredOverride reports a plugin helper shadowing a shared
// vocabulary name without declaring the override: the shared
// helper stands, because a silent replacement is the drift
// byte-identity cannot tolerate.
var UndeclaredOverride = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 27, Meaning: "a plugin shadows a shared template helper without declaring the override",
})

// DroppedSlots reports a body-claiming template that placed no
// marker for pending slot content: the template owns the layout,
// so nothing is appended for it, and the Error names the emitting
// plugin and counts what went unplaced. The contributor cannot be
// named, because a slot statement carries no attribution.
var DroppedSlots = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 26, Meaning: "a body-claiming template places no marker for pending contributions",
})

// UnknownGroup reports a cluster naming a group the language
// declares no template for: the cluster's declarations are
// skipped and the file renders without them.
var UnknownGroup = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 28, Meaning: "a cluster names a group the target language declares no template for",
})

// HelperCollision reports two plugins registering one template
// helper name the shared vocabulary does not own: the first
// registration in composition order stands, because a helper
// whose meaning follows the schedule renders different bytes from
// one declaration.
var HelperCollision = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number: 36, Meaning: "two plugins register one template helper name",
})

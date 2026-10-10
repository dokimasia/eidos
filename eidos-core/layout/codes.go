// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import "go.dokimi.dev/eidos/core/diag"

// UndeclaredFamily reports a unit whose family its plugin does not
// declare: a routable declaration has no family to route through, and
// the unit's declarations are refused.
var UndeclaredFamily = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  48,
	Meaning: "a unit's family is not among its plugin's declared families",
})

// UnknownTag reports a tag override naming a family its plugin does not
// declare, at the carrier line that wrote it. The declaration is
// refused.
var UnknownTag = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  49,
	Meaning: "a tag override names no family the plugin declares",
})

// AmbiguousOverride reports a filename override on a declaration its
// plugin emits into more than one family from, without a tag that
// picks one: every family would be written to one file. The
// declarations are refused.
var AmbiguousOverride = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  50,
	Meaning: "a filename override applies to more than one family of one plugin",
})

// NoDestination reports a declaration no directory resolves for: a
// per-source or per-package declaration whose origin has no workspace
// source, such as one in a dependency store, or whose family changed
// cardinality without an origin to derive the new key from.
var NoDestination = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  51,
	Meaning: "no directory resolves for a declaration",
})

// EscapingPath reports a path override that is absolute or whose
// elements leave the workspace root, at the carrier line that wrote
// it. The declaration is refused and nothing is written for it.
var EscapingPath = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  52,
	Meaning: "a path override is absolute or leaves the workspace root",
})

// PathCollision reports routed paths one tree cannot contain: two
// packages' declarations at one path, two paths that differ only in
// case, or a path another path needs as a directory. The second
// declaration's origin is the position and the first's is related.
// The declarations at both paths are refused.
var PathCollision = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  53,
	Meaning: "two routed paths collide",
})

// UnderivedPackage reports a reference into a routed file whose
// package the target derives no identity for, at the referencing
// declaration's origin. The referencing declaration is refused.
var UnderivedPackage = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  54,
	Meaning: "a reference needs the package of a routed file, and none derives",
})

// UntranslatedReference reports a reference to a declaration of another
// language that the plan does not emit. The finding is at the origin of
// the referencing declaration, and the referencing declaration is
// refused.
var UntranslatedReference = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  70,
	Meaning: "a translated reference refers to a declaration that the plan does not emit",
})

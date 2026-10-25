// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Importable returns the reason that a Go file of the package file cannot
// refer to the referent of ref, as [golang.CheckReference] decides it. The
// package of the referent is the package that the Go backend imports for
// ref, which [spellref.PackageOf] returns, and its name is the spelling of
// ref without its qualifier. A nil reference has no referent. The Go backend declares Importable as its import rule, so
// the layout refuses a declaration whose file cannot refer to one of its
// references, and keeps the other declarations of the file.
//
// Importable allocates nothing for a reference that the file can refer to. A
// refusal allocates its error.
func Importable(file symbol.Identity, ref *emit.TypeRef) error {
	if ref == nil {
		return nil
	}
	return golang.CheckReference(file.Package, spellref.PackageOf(ref, golang.Lang), golang.Unqualified(ref.Spelling))
}
